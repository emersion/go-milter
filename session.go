package milter

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
	"time"
)

var errCloseSession = errors.New("Stop current milter processing")

// OptAction sets which actions the milter wants to perform.
// Multiple options can be set using a bitmask.
type OptAction uint32

// set which actions the milter wants to perform
const (
	OptAddHeader    OptAction = 0x01
	OptChangeBody   OptAction = 0x02
	OptAddRcpt      OptAction = 0x04
	OptRemoveRcpt   OptAction = 0x08
	OptChangeHeader OptAction = 0x10
	OptQuarantine   OptAction = 0x20
	OptChangeFrom   OptAction = 0x40
)

// OptProtocol masks out unwanted parts of the SMTP transaction.
// Multiple options can be set using a bitmask.
type OptProtocol uint32

const (
	OptNoConnect  OptProtocol = 0x01
	OptNoHelo     OptProtocol = 0x02
	OptNoMailFrom OptProtocol = 0x04
	OptNoRcptTo   OptProtocol = 0x08
	OptNoBody     OptProtocol = 0x10
	OptNoHeaders  OptProtocol = 0x20
	OptNoEOH      OptProtocol = 0x40

	// [v6] MTA supports ActSkip.
	OptSkip OptProtocol = 0x400

	// [v6] milter will not send action response for following MTA messages.
	OptNoHeaderReply  OptProtocol = 0x80
	OptNoConnReply    OptProtocol = 0x1000
	OptNoHeloReply    OptProtocol = 0x2000
	OptNoMailReply    OptProtocol = 0x4000
	OptNoRcptReply    OptProtocol = 0x8000
	OptNoDataReply    OptProtocol = 0x10000
	OptNoUnknownReply OptProtocol = 0x20000
	OptNoEOHReply     OptProtocol = 0x40000
	OptNoBodyReply    OptProtocol = 0x80000
)

// milterSession keeps session state during MTA communication
type milterSession struct {
	actions  OptAction
	protocol OptProtocol
	conn     net.Conn
	headers  textproto.MIMEHeader
	macros   map[string]string
	backend  Milter
}

// ReadPacket reads incoming milter packet
func (c *milterSession) ReadPacket() (*Message, error) {
	return readPacket(c.conn, 0)
}

func readPacket(conn net.Conn, timeout time.Duration) (*Message, error) {
	if timeout != 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
		defer conn.SetReadDeadline(time.Time{})
	}

	// read packet length
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	// read packet data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	// prepare response data
	message := Message{
		Code: data[0],
		Data: data[1:],
	}

	return &message, nil
}

// WritePacket sends a milter response packet to socket stream
func (m *milterSession) WritePacket(msg *Message) error {
	return writePacket(m.conn, msg, 0)
}

func writePacket(conn net.Conn, msg *Message, timeout time.Duration) error {
	if timeout != 0 {
		conn.SetWriteDeadline(time.Now().Add(timeout))
		defer conn.SetWriteDeadline(time.Time{})
	}

	buffer := bufio.NewWriter(conn)

	// calculate and write response length
	length := uint32(len(msg.Data) + 1)
	if err := binary.Write(buffer, binary.BigEndian, length); err != nil {
		return err
	}

	// write response code
	if err := buffer.WriteByte(msg.Code); err != nil {
		return err
	}

	// write response data
	if _, err := buffer.Write(msg.Data); err != nil {
		return err
	}

	// flush data to network socket stream
	if err := buffer.Flush(); err != nil {
		return err
	}

	return nil
}

// Process processes incoming milter commands
func (m *milterSession) Process(msg *Message) (Response, error) {
	switch Code(msg.Code) {
	case CodeAbort:
		// abort current message and start over
		m.headers = nil
		m.macros = nil
		// do not send response
		return nil, nil

	case CodeBody:
		// body chunk
		return m.backend.BodyChunk(msg.Data, newModifier(m))

	case CodeConn:
		// new connection, get hostname
		Hostname := readCString(msg.Data)
		msg.Data = msg.Data[len(Hostname)+1:]
		// get protocol family
		protocolFamily := msg.Data[0]
		msg.Data = msg.Data[1:]
		// get port
		var Port uint16
		if protocolFamily == '4' || protocolFamily == '6' {
			if len(msg.Data) < 2 {
				return RespTempFail, nil
			}
			Port = binary.BigEndian.Uint16(msg.Data)
			msg.Data = msg.Data[2:]
		}
		// get address
		Address := readCString(msg.Data)
		// convert address and port to human readable string
		family := map[byte]string{
			'U': "unknown",
			'L': "unix",
			'4': "tcp4",
			'6': "tcp6",
		}
		// run handler and return
		return m.backend.Connect(
			Hostname,
			family[protocolFamily],
			Port,
			net.ParseIP(Address),
			newModifier(m))

	case CodeMacro:
		// define macros
		m.macros = make(map[string]string)
		// convert data to Go strings
		data := decodeCStrings(msg.Data[1:])
		if len(data) != 0 {
			// store data in a map
			for i := 0; i < len(data); i += 2 {
				m.macros[data[i]] = data[i+1]
			}
		}
		// do not send response
		return nil, nil

	case CodeEOB:
		// call and return milter handler
		return m.backend.Body(newModifier(m))

	case CodeHelo:
		// helo command
		name := strings.TrimSuffix(string(msg.Data), null)
		return m.backend.Helo(name, newModifier(m))

	case CodeHeader:
		// make sure headers is initialized
		if m.headers == nil {
			m.headers = make(textproto.MIMEHeader)
		}
		// add new header to headers map
		HeaderData := decodeCStrings(msg.Data)
		if len(HeaderData) == 2 {
			m.headers.Add(HeaderData[0], HeaderData[1])
			// call and return milter handler
			return m.backend.Header(HeaderData[0], HeaderData[1], newModifier(m))
		}

	case CodeMail:
		// envelope from address
		envfrom := readCString(msg.Data)
		return m.backend.MailFrom(strings.Trim(envfrom, "<>"), newModifier(m))

	case CodeEOH:
		// end of headers
		return m.backend.Headers(m.headers, newModifier(m))

	case CodeOptNeg:
		// ignore request and prepare response buffer
		buffer := new(bytes.Buffer)
		// prepare response data
		for _, value := range []uint32{protocolVersion, uint32(m.actions), uint32(m.protocol)} {
			if err := binary.Write(buffer, binary.BigEndian, value); err != nil {
				return nil, err
			}
		}
		// build and send packet
		return NewResponse('O', buffer.Bytes()), nil

	case CodeQuit:
		// client requested session close
		return nil, errCloseSession

	case CodeRcpt:
		// envelope to address
		envto := readCString(msg.Data)
		return m.backend.RcptTo(strings.Trim(envto, "<>"), newModifier(m))

	case CodeData:
		// data, ignore

	default:
		// print error and close session
		log.Printf("Unrecognized command code: %c", msg.Code)
		return nil, errCloseSession
	}

	// by default continue with next milter message
	return RespContinue, nil
}

// HandleMilterComands processes all milter commands in the same connection
func (m *milterSession) HandleMilterCommands() {
	// close session socket on exit
	defer m.conn.Close()

	for {
		// ReadPacket
		msg, err := m.ReadPacket()
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading milter command: %v", err)
			}
			return
		}

		// process command
		resp, err := m.Process(msg)
		if err != nil {
			if err != errCloseSession {
				// log error condition
				log.Printf("Error performing milter command: %v", err)
			}
			return
		}

		// ignore empty responses
		if resp != nil {
			// send back response message
			if err = m.WritePacket(resp.Response()); err != nil {
				log.Printf("Error writing packet: %v", err)
				return
			}

			if !resp.Continue() {
				return
			}

		}
	}
}
