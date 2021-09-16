package milter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-message/textproto"
)

// Milter protocol version implemented by the client.
//
// Note: Not exported as we might want to support multiple versions
// transparently in the future.

// [sjs] - changed protocol version to 2 to work with the server
const clientProtocolVersion uint32 = 2

// Client is a wrapper for managing milter connections.
//
// Currently it just creates new connections using provided Dialer.
type Client struct {
	opts    ClientOptions
	network string
	address string
}

type Dialer interface {
	Dial(network string, addr string) (net.Conn, error)
}

type ClientOptions struct {
	Dialer       Dialer
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	ActionMask   OptAction
	ProtocolMask OptProtocol
}

var defaultOptions = ClientOptions{
	Dialer: &net.Dialer{
		Timeout: 10 * time.Second,
	},
	ReadTimeout:  10 * time.Second,
	WriteTimeout: 10 * time.Second,
	ActionMask:   OptAddHeader | OptAddRcpt | OptChangeBody | OptChangeFrom | OptChangeHeader,
	ProtocolMask: 0,
}

// NewDefaultClient creates a new Client object using default options.
//
// It uses 10 seconds for connection/read/write timeouts and allows milter to
// send any actions supported by library.
func NewDefaultClient(network, address string) *Client {
	return NewClientWithOptions(network, address, defaultOptions)
}

// NewClientWithOptions creates a new Client object using provided options.
//
// You generally want to use options to restrict ActionMask to what your code
// supports and ProtocolMask to what you intend to submit.
//
// If opts.Dialer is not set, empty net.Dialer object will be used.
func NewClientWithOptions(network, address string, opts ClientOptions) *Client {
	if opts.Dialer == nil {
		opts.Dialer = &net.Dialer{}
	}

	return &Client{
		opts:    opts,
		network: network,
		address: address,
	}
}

func (c *Client) Session() (*ClientSession, error) {
	s := &ClientSession{
		readTimeout:  c.opts.ReadTimeout,
		writeTimeout: c.opts.WriteTimeout,
	}

	// TODO(foxcpp): Connection pooling.

	conn, err := c.opts.Dialer.Dial(c.network, c.address)
	if err != nil {
		return nil, fmt.Errorf("milter: session create: %w", err)
	}

	s.conn = conn
	if err := s.negotiate(c.opts.ActionMask, c.opts.ProtocolMask); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Client) Close() error {
	// Reserved for use in connection pooling.
	return nil
}

type ClientSession struct {
	conn net.Conn

	// Bitmask of negotiated action options.
	ActionOpts OptAction

	// Bitmask of negotiated protocol options.
	ProtocolOpts OptProtocol

	needAbort bool

	readTimeout  time.Duration
	writeTimeout time.Duration
}

// negotiate exchanges OPTNEG messages with the milter and sets s.mask to the
// negotiated value.
func (s *ClientSession) negotiate(actionMask OptAction, protoMask OptProtocol) error {
	// Send our mask, get mask from milter..
	msg := &Message{
		Code: byte(CodeOptNeg), // TODO(foxcpp): Get rid of casts by changing msg.Code to have Code type
		Data: make([]byte, 4*3),
	}
	binary.BigEndian.PutUint32(msg.Data, clientProtocolVersion)
	binary.BigEndian.PutUint32(msg.Data[4:], uint32(actionMask))
	binary.BigEndian.PutUint32(msg.Data[8:], uint32(protoMask))

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return fmt.Errorf("milter: negotiate: optneg write: %w", err)
	}
	msg, err := readPacket(s.conn, s.readTimeout)
	if err != nil {
		return fmt.Errorf("milter: negotiate: optneg read: %w", err)
	}
	if Code(msg.Code) != CodeOptNeg {
		return fmt.Errorf("milter: negotiate: unexpected code: %v", rune(msg.Code))
	}
	if len(msg.Data) < 4*3 /* version + action mask + proto mask */ {
		return fmt.Errorf("milter: negotiate: unexpected data size: %v", len(msg.Data))
	}
	milterVersion := binary.BigEndian.Uint32(msg.Data[:4])

	// Not a strict comparison since we might be able to work correctly with
	// milter using a newer protocol as long as masks negotiated are meaningful.
	if milterVersion < clientProtocolVersion {
		return fmt.Errorf("milter: negotiate: unsupported protocol version: %v", milterVersion)
	}

	milterActionMask := binary.BigEndian.Uint32(msg.Data[4:])
	s.ActionOpts = OptAction(milterActionMask)
	milterProtoMask := binary.BigEndian.Uint32(msg.Data[8:])
	s.ProtocolOpts = OptProtocol(milterProtoMask)

	s.needAbort = true

	return nil
}

// ProtocolOption checks whether the option is set in negotiated options, that
// is, requested by both sides.
func (s *ClientSession) ProtocolOption(opt OptProtocol) bool {
	return s.ProtocolOpts&opt != 0
}

// ActionOption checks whether the option is set in negotiated options, that
// is, requested by both sides.
func (s *ClientSession) ActionOption(opt OptAction) bool {
	return s.ActionOpts&opt != 0
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

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return fmt.Errorf("milter: macros: %w", err)
	}

	return nil
}

func appendUint16(dest []byte, val uint16) []byte {
	dest = append(dest, 0x00, 0x00)
	binary.BigEndian.PutUint16(dest[len(dest)-2:], val)
	return dest
}

type Action struct {
	Code ActionCode

	// SMTP code if Code == ActReplyCode.
	SMTPCode int
	// Reply text if Code == ActReplyCode.
	SMTPText string
}

func (s *ClientSession) readAction() (*Action, error) {
	for {
		msg, err := readPacket(s.conn, s.readTimeout)
		if err != nil {
			return nil, fmt.Errorf("action read: %w", err)
		}
		if msg.Code == 'p' /* progress */ {
			continue
		}
		if ActionCode(msg.Code) != ActContinue {
			s.needAbort = false
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

// Conn sends the connection information to the milter.
//
// It should be called once per milter session (from Session to Close).
func (s *ClientSession) Conn(hostname string, family ProtoFamily, port uint16, addr string) (*Action, error) {
	if s.ProtocolOpts&OptNoConnect != 0 {
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

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: conn: %w", err)
	}

	if !s.ProtocolOption(OptNoConnReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: conn: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

// Helo sends the HELO hostname to the milter.
//
// It should be called once per milter session (from Session to Close).
func (s *ClientSession) Helo(helo string) (*Action, error) {
	// Synthesise response as if server replied "go on" while in fact it does
	// not support that message.
	if s.ProtocolOpts&OptNoHelo != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeHelo),
		Data: appendCString(nil, helo),
	}

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: helo: %w", err)
	}

	if !s.ProtocolOption(OptNoHeloReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: helo: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

func (s *ClientSession) Mail(sender string, esmtpArgs []string) (*Action, error) {
	if s.ProtocolOpts&OptNoMailFrom != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeMail),
	}

	msg.Data = appendCString(msg.Data, "<"+sender+">")
	for _, arg := range esmtpArgs {
		msg.Data = appendCString(msg.Data, arg)
	}

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: mail: %w", err)
	}

	if !s.ProtocolOption(OptNoMailReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: mail: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

func (s *ClientSession) Rcpt(rcpt string, esmtpArgs []string) (*Action, error) {
	if s.ProtocolOpts&OptNoRcptTo != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeRcpt),
	}

	msg.Data = appendCString(msg.Data, "<"+rcpt+">")
	for _, arg := range esmtpArgs {
		msg.Data = appendCString(msg.Data, arg)
	}

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: rcpt: %w", err)
	}

	if !s.ProtocolOption(OptNoRcptReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: rcpt: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

// HeaderField sends a single header field to the milter.
//
// Value should be the original field value without any unfolding applied.
//
// HeaderEnd() must be called after the last field.
func (s *ClientSession) HeaderField(key, value string) (*Action, error) {
	if s.ProtocolOpts&OptNoHeaders != 0 {
		return &Action{Code: ActContinue}, nil
	}

	msg := &Message{
		Code: byte(CodeHeader),
	}
	msg.Data = appendCString(msg.Data, key)
	msg.Data = appendCString(msg.Data, value)

	if err := writePacket(s.conn, msg, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: header field: %w", err)
	}

	if !s.ProtocolOption(OptNoHeaderReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: header field: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

// HeaderEnd send the EOH (End-Of-Header) message to the milter.
//
// No HeaderField calls are allowed after this point.
func (s *ClientSession) HeaderEnd() (*Action, error) {
	if s.ProtocolOpts&OptNoEOH != 0 {
		return &Action{Code: ActContinue}, nil
	}

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeEOH),
	}, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: header end: %w", err)
	}

	if !s.ProtocolOption(OptNoEOHReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: header end: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
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
//
// If OptSkip was specified during negotiation, caller should be ready to
// handle return ActSkip and stop sending body chunks if it is returned.
func (s *ClientSession) BodyChunk(chunk []byte) (*Action, error) {
	if s.ProtocolOpts&OptNoBody != 0 {
		return &Action{Code: ActContinue}, nil
	}

	// Callers tend to be irresponsible... /s
	if len(chunk) > MaxBodyChunk {
		return nil, fmt.Errorf("milter: body chunk: too big body chunk: %v", len(chunk))
	}

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeBody),
		Data: chunk,
	}, s.writeTimeout); err != nil {
		return nil, fmt.Errorf("milter: body chunk: %w", err)
	}

	if !s.ProtocolOption(OptNoBodyReply) {
		act, err := s.readAction()
		if err != nil {
			return nil, fmt.Errorf("milter: body chunk: %w", err)
		}
		return act, nil
	}
	return &Action{Code: ActContinue}, nil
}

// BodyReadFrom is a helper function that calls BodyChunk repeately to transmit entire
// body from io.Reader and then calls End.
//
// See documentation for these functions for details.
func (s *ClientSession) BodyReadFrom(r io.Reader) ([]ModifyAction, *Action, error) {
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
		if act.Code == ActSkip {
			break
		}
		if act.Code != ActContinue {
			return nil, act, nil
		}
	}

	return s.End()
}

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
	// E.g. HeaderIndex = 3 and HdrName = "DKIM-Signature" mean "change third
	// DKIM-Signature field". Order is the same as of HeaderField calls.
	HeaderIndex uint32

	// Header field name to be added/changed if Code == ActAddHeader or
	// ActChangeHeader or ActInsertHeader.
	HeaderName string

	// Header field value to be added/changed if Code == ActAddHeader or
	// ActChangeHeader or ActInsertHeader. If set to empty string - the field
	// should be removed.
	HeaderValue string

	// Quarantine reason if Code == ActQuarantine.
	Reason string
}

func parseModifyAct(msg *Message) (*ModifyAction, error) {
	act := &ModifyAction{
		Code: ModifyActCode(msg.Code),
	}

	switch ModifyActCode(msg.Code) {
	case ActAddRcpt, ActDelRcpt:
		act.Rcpt = readCString(msg.Data)
	case ActQuarantine:
		act.Reason = readCString(msg.Data)
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
		act.HeaderIndex = binary.BigEndian.Uint32(msg.Data)

		msg.Data = msg.Data[4:]
		fallthrough
	case ActAddHeader:
		// TODO: Change readCString to return last index.
		act.HeaderName = readCString(msg.Data)
		nul := bytes.IndexByte(msg.Data, 0x00)
		if nul == -1 {
			return nil, fmt.Errorf("read modify action: missing NUL delimiter")
		}
		if nul == len(msg.Data) {
			return nil, fmt.Errorf("read modify action: missing header value")
		}
		act.HeaderValue = readCString(msg.Data[nul+1:])
	default:
		return nil, fmt.Errorf("read modify action: unexpected message code: %v", msg.Code)
	}

	return act, nil
}

func (s *ClientSession) readModifyActs() (modifyActs []ModifyAction, act *Action, err error) {
	for {
		msg, err := readPacket(s.conn, s.readTimeout)
		if err != nil {
			return nil, nil, fmt.Errorf("action read: %w", err)
		}
		if msg.Code == 'p' /* progress */ {
			continue
		}

		switch ModifyActCode(msg.Code) {
		case ActAddRcpt, ActDelRcpt, ActReplBody, ActChangeHeader, ActInsertHeader,
			ActAddHeader, ActChangeFrom, ActQuarantine:
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
	}, s.writeTimeout); err != nil {
		return nil, nil, fmt.Errorf("milter: end: %w", err)
	}

	modifyActs, act, err := s.readModifyActs()
	if err != nil {
		return nil, nil, fmt.Errorf("milter: end: %w", err)
	}

	return modifyActs, act, nil
}

// Close releases resources associated with the session.
//
// If there a milter sequence in progress - it is aborted.
func (s *ClientSession) Close() error {
	if s.needAbort {
		writePacket(s.conn, &Message{
			Code: byte(CodeAbort),
		}, s.writeTimeout)
	}

	if err := writePacket(s.conn, &Message{
		Code: byte(CodeQuit),
	}, s.writeTimeout); err != nil {
		return fmt.Errorf("milter: close: %w", err)
	}
	return s.conn.Close()
}
