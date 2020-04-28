package milter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/emersion/go-message/textproto"
)

type Client struct {
	// Dialer is used to establish new connections to the milter.
	// Set to empty net.Dialer{} by NewClient.
	Dialer interface {
		Dial(network string, addr string) (net.Conn, error)
	}

	network string
	address string
}

func NewClient(network, address string) *Client {
	return &Client{
		Dialer:  &net.Dialer{},
		network: network,
		address: address,
	}
}

func (c *Client) Session(actionMask OptAction, protoMask OptProtocol) (*ClientSession, error) {
	s := &ClientSession{}

	// TODO(foxcpp): Connection pooling.

	conn, err := c.Dialer.Dial(c.network, c.address)
	if err != nil {
		return nil, fmt.Errorf("milter: session create: %w", err)
	}

	s.conn = conn
	if err := s.negotiate(actionMask, protoMask); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Client) Close() error {
	// Reserved for use in connection pooling.
	return nil
}

type ClientSession struct {
	conn       net.Conn
	actionMask OptAction
	protoMask  OptProtocol
}

// negotiate exchanges OPTNEG messages with the milter and sets s.mask to the
// negotiated value.
func (s *ClientSession) negotiate(actionMask OptAction, protoMask OptProtocol) error {
	// Send our mask, get mask from milter and take the lowest common
	// denomiator as the effective mask.
	msg := &Message{
		Code: byte(CodeOptNeg), // TODO(foxcpp): Get rid of casts by changing msg.Code to have Code type
		Data: make([]byte, 4*3),
	}
	binary.BigEndian.PutUint32(msg.Data, protocolVersion)
	binary.BigEndian.PutUint32(msg.Data[4:], uint32(actionMask))
	binary.BigEndian.PutUint32(msg.Data[8:], uint32(protoMask))

	if err := writePacket(s.conn, msg); err != nil {
		return fmt.Errorf("milter: negotiate: optneg write: %w", err)
	}
	msg, err := readPacket(s.conn)
	if err != nil {
		return fmt.Errorf("milter: negotiate: optneg read: %w", err)
	}
	if Code(msg.Code) != CodeOptNeg {
		return fmt.Errorf("milter: negotiate: unexpected code: %v", rune(msg.Code))
	}
	if len(msg.Data) != 4*3 /* version + action mask + proto mask */ {
		return fmt.Errorf("milter: negotiate: unexpected data size: %v", len(msg.Data))
	}
	milterVersion := binary.BigEndian.Uint32(msg.Data[:4])
	if milterVersion != protocolVersion {
		return fmt.Errorf("milter: negotiate: unsupported protocol version: %v", milterVersion)
	}

	// AND it with our mask in case milter does not do that.
	milterActionMask := binary.BigEndian.Uint32(msg.Data[4:])
	s.actionMask = actionMask & OptAction(milterActionMask)
	milterProtoMask := binary.BigEndian.Uint32(msg.Data[8:])
	s.protoMask = protoMask & OptProtocol(milterProtoMask)

	return nil
}

func (s *ClientSession) Macros(code Code, kv ...string) error {
	// Note: kv is ...string with the expectation that the list of macro names
	// will be static and not dynamically constructed.

	msg := &Message{
		Code: byte(CodeMacro),
		Data: []byte{byte(code)},
	}
	for _, str := range kv {
		msg.Data = appendCString(msg.Data, str)
	}

	if err := writePacket(s.conn, msg); err != nil {
		return fmt.Errorf("milter: macros: %w", err)
	}

	return nil
}

func appendUint16(dest []byte, val uint16) []byte {
	dest = append(dest, 0x00, 0x00)
	binary.BigEndian.PutUint16(dest[len(dest)-2:], val)
	return dest
}

type ActionCode rune

const (
	ActAccept     ActionCode = accept
	ActContinue   ActionCode = continue_
	ActDiscard    ActionCode = discard
	ActQuarantine ActionCode = quarantine
	ActReject     ActionCode = reject
	ActTempFail   ActionCode = tempFail
	ActReplyCode  ActionCode = replyCode
)

type Action struct {
	Code ActionCode

	// Quarantine reason if Code == ActQurantine.
	Reason string

	// SMTP code if Code == ActReplyCode.
	SMTPCode int
	// Reply text if Code == ActReplyCode.
	SMTPText string
}

func (s *ClientSession) readAction() (*Action, error) {
	for {
		msg, err := readPacket(s.conn)
		if err != nil {
			return nil, fmt.Errorf("action read: %w", err)
		}
		if msg.Code == 'p' /* progress */ {
			continue
		}

		return parseAction(msg)
	}
}

func parseAction(msg *Message) (*Action, error) {
	act := &Action{
		Code: ActionCode(msg.Code),
	}
	var err error

	switch ActionCode(msg.Code) {
	case ActAccept, ActContinue, ActDiscard, ActReject, ActTempFail:
	case ActQuarantine:
		act.Reason = readCString(msg.Data)
	case ActReplyCode:
		if len(msg.Data) <= 4 {
			return nil, fmt.Errorf("action read: unexpected data length: %v", len(msg.Data))
		}
		act.SMTPCode, err = strconv.Atoi(string(msg.Data[:3]))
		if err != nil {
			return nil, fmt.Errorf("action read: malformed SMTP code: %v", msg.Data[:3])
		}
		// There is 0x20 (' ') in between.
		act.SMTPText = readCString(msg.Data[4:])
	default:
		return nil, fmt.Errorf("action read: unexpected code: %v", msg.Code)
	}

	return act, nil
}

// Helo sends the connection information to the milter.
//
// It should be called once per milter session (from NewSession to Close).
func (s *ClientSession) Conn(hostname string, family ProtoFamily, port uint16, addr string) (*Action, error) {
	if s.protoMask&OptNoConnect != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeConn),
	}
	msg.Data = appendCString(msg.Data, hostname)
	msg.Data = append(msg.Data, byte(family))
	if family != FamilyUnknown {
		if family == FamilyInet || family == FamilyInet6 {
			msg.Data = appendUint16(msg.Data, port)
		}
		msg.Data = appendCString(msg.Data, addr)
	}

	if err := writePacket(s.conn, msg); err != nil {
		return nil, fmt.Errorf("milter: conn: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: conn: %w", err)
	}
	return act, nil
}

// Helo sends the HELO hostname to the milter.
//
// It should be called once per milter session (from NewSession to Close).
func (s *ClientSession) Helo(helo string) (*Action, error) {
	// Synthesise response as if server replied "go on" while in fact it does
	// not support that message.
	if s.protoMask&OptNoHelo != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeHelo),
		Data: appendCString(nil, helo),
	}

	if err := writePacket(s.conn, msg); err != nil {
		return nil, fmt.Errorf("milter: helo: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: helo: %w", err)
	}
	return act, nil
}

func (s *ClientSession) Mail(sender string, esmtpArgs []string) (*Action, error) {
	if s.protoMask&OptNoMailFrom != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeMail),
	}

	msg.Data = appendCString(msg.Data, "<"+sender+">")
	for _, arg := range esmtpArgs {
		msg.Data = appendCString(msg.Data, arg)
	}

	if err := writePacket(s.conn, msg); err != nil {
		return nil, fmt.Errorf("milter: mail: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: mail: %w", err)
	}
	return act, nil
}

func (s *ClientSession) Rcpt(rcpt string, esmtpArgs []string) (*Action, error) {
	if s.protoMask&OptNoRcptTo != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeRcpt),
	}

	msg.Data = appendCString(msg.Data, "<"+rcpt+">")
	for _, arg := range esmtpArgs {
		msg.Data = appendCString(msg.Data, arg)
	}

	if err := writePacket(s.conn, msg); err != nil {
		return nil, fmt.Errorf("milter: rcpt: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: rcpt: %w", err)
	}
	return act, nil
}

// HeaderField sends a single header field to the milter.
//
// HeaderEnd() must be called after the last field.
func (s *ClientSession) HeaderField(key, value string) (*Action, error) {
	if s.protoMask&OptNoHeaders != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeHeader),
	}
	msg.Data = appendCString(msg.Data, key)
	msg.Data = appendCString(msg.Data, value)

	if err := writePacket(s.conn, msg); err != nil {
		return nil, fmt.Errorf("milter: header field: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: header field: %w", err)
	}
	return act, nil
}

// HeaderEnd send the EOH (End-Of-Header) message to the milter.
//
// No HeaderField calls are allowed after this point.
func (s *ClientSession) HeaderEnd() (*Action, error) {
	if s.protoMask&OptNoEOH != 0 {
		return &Action{Code: ActContinue}, nil
	}

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeEOH),
	}); err != nil {
		return nil, fmt.Errorf("milter: header end: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: header end: %w", err)
	}
	return act, nil
}

// Header sends each field from textproto.Header followed by EOH unless
// header messages are disabled during negotiation.
func (s *ClientSession) Header(hdr textproto.Header) (*Action, error) {
	for f := hdr.Fields(); f.Next(); {
		act, err := s.HeaderField(f.Key(), f.Value())
		if err != nil {
			return nil, err
		}
		if act.Code != ActContinue {
			return act, nil
		}
	}
	return s.HeaderEnd()
}

// BodyChunk sends a single body chunk to the milter.
//
// It is callers responsibility to ensure every chunk is not bigger than
// MaxBodyChunk.
func (s *ClientSession) BodyChunk(chunk []byte) (*Action, error) {
	if s.protoMask&OptNoBody != 0 {
		return &Action{Code: ActContinue}, nil
	}

	// Developers tend to be irresponsible... /s
	if len(chunk) > MaxBodyChunk {
		return nil, fmt.Errorf("milter: body chunk: too big body chunk: %v", len(chunk))
	}

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeBody),
		Data: chunk,
	}); err != nil {
		return nil, fmt.Errorf("milter: body chunk: %w", err)
	}

	act, err := s.readAction()
	if err != nil {
		return nil, fmt.Errorf("milter: body chunk: %w", err)
	}
	return act, nil
}

// Body is a helper function that calls BodyChunk repeately to transmit entire
// body from io.Reader and then calls End.
//
// See documentation for these functions for details.
func (s *ClientSession) Body(r io.Reader) ([]ModifyAction, *Action, error) {
	// It is problematic to use io.WriteCloser since we may need to report
	// action after each write.

	buf := make([]byte, MaxBodyChunk)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		if n == 0 {
			break
		}

		act, err := s.BodyChunk(buf[:n])
		if err != nil {
			return nil, nil, err
		}
		if act.Code != ActContinue {
			return nil, act, nil
		}
	}

	return s.End()
}

type ModifyActCode byte

const (
	ActAddRcpt      ModifyActCode = '+'
	ActDelRcpt      ModifyActCode = '-'
	ActReplBody     ModifyActCode = 'b'
	ActAddHeader    ModifyActCode = 'h'
	ActChangeHeader ModifyActCode = 'm'
	ActInsertHeader ModifyActCode = 'i'
	ActChangeFrom   ModifyActCode = 'e'
)

type ModifyAction struct {
	Code ModifyActCode

	// Recipient to add/remove if Code == ActAddRcpt or ActDelRcpt.
	Rcpt string

	// New envelope sender if Code = ActChangeFrom.
	From string

	// ESMTP arguments for envelope sender if Code = ActChangeFrom.
	FromArgs []string

	// Portion of body to be replaced if Code == ActReplBody.
	Body []byte

	// Index of the header field to be changed if Code = ActChangeHeader or Code = ActInsertHeader.
	// Index is 1-based and is per value of HdrName.
	// E.g. HdrIndex = 3 and HdrName = "DKIM-Signature" mean "change third
	// DKIM-Signature field". Order is the same as of HeaderField calls.
	HdrIndex uint32

	// Header field name to be added/changed if Code == ActAddHeader or
	// ActChangeHeader or ActInsertHeader.
	HdrName string

	// Header field value to be added/changed if Code == ActAddHeader or
	// ActChangeHeader or ActInsertHeader. If set to empty string - the field
	// should be removed.
	HdrValue string
}

func parseModifyAct(msg *Message) (*ModifyAction, error) {
	act := &ModifyAction{
		Code: ModifyActCode(msg.Code),
	}

	switch ModifyActCode(msg.Code) {
	case ActAddRcpt, ActDelRcpt:
		act.Rcpt = readCString(msg.Data)
	case ActReplBody:
		act.Body = msg.Data
	case ActChangeFrom:
		argv := bytes.Split(msg.Data, []byte{0x00})
		act.From = string(argv[0])
		for _, arg := range argv[1:] {
			act.FromArgs = append(act.FromArgs, string(arg))
		}
	case ActChangeHeader, ActInsertHeader:
		if len(msg.Data) < 4 {
			return nil, fmt.Errorf("read modify action: missing header index")
		}
		act.HdrIndex = binary.BigEndian.Uint32(msg.Data)

		msg.Data = msg.Data[4:]
		fallthrough
	case ActAddHeader:
		// TODO: Change readCString to return last index.
		act.HdrName = readCString(msg.Data)
		nul := bytes.IndexByte(msg.Data, 0x00)
		if nul == -1 {
			return nil, fmt.Errorf("read modify action: missing NUL delimiter")
		}
		if nul == len(msg.Data) {
			return nil, fmt.Errorf("read modify action: missing header value")
		}
		act.HdrValue = readCString(msg.Data[nul+1:])
	default:
		return nil, fmt.Errorf("read modify action: unexpected message code: %v", msg.Code)
	}

	return act, nil
}

func (s *ClientSession) readModifyActs() (modifyActs []ModifyAction, act *Action, err error) {
	for {
		msg, err := readPacket(s.conn)
		if err != nil {
			return nil, nil, fmt.Errorf("action read: %w", err)
		}
		if msg.Code == 'p' /* progress */ {
			continue
		}

		switch ModifyActCode(msg.Code) {
		case ActAddRcpt, ActDelRcpt, ActReplBody, ActChangeHeader, ActInsertHeader, ActAddHeader, ActChangeFrom:
			modifyAct, err := parseModifyAct(msg)
			if err != nil {
				return nil, nil, err
			}
			modifyActs = append(modifyActs, *modifyAct)
		default:
			act, err = parseAction(msg)
			if err != nil {
				return nil, nil, err
			}

			return modifyActs, act, nil
		}
	}
}

// End sends the EOB message and resets session back to the state before Mail
// call. The same ClientSession can be used to check another message arrived
// within the same SMTP connection (Helo and Conn information is preserved).
//
// Close should be called to conclude session.
func (s *ClientSession) End() ([]ModifyAction, *Action, error) {
	if err := writePacket(s.conn, &Message{
		Code: byte(CodeEOB),
	}); err != nil {
		return nil, nil, fmt.Errorf("milter: end: %w", err)
	}

	modifyActs, act, err := s.readModifyActs()
	if err != nil {
		return nil, nil, fmt.Errorf("milter: end: %w", err)
	}

	return modifyActs, act, nil
}

// Close releases resources associated with the session.
func (s *ClientSession) Close() error {
	// TODO(foxcpp): Send ABORT and return connection to the pool instead.

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeQuit),
	}); err != nil {
		return fmt.Errorf("milter: close: %w", err)
	}
	return s.Close()
}
