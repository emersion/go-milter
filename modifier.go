/* Modifier object instance is provided to milter handlers to modify email messages */
package milter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/textproto"
)

/* Modifier represents incoming milter command */
type Modifier struct {
	Macros      map[string]string
	Headers     textproto.MIMEHeader
	Body        []byte
	WritePacket func(*Message) error
}

/* AddRecipient appends a new recipient for current message */
func (m *Modifier) AddRecipient(r string) error {
	data := []byte(fmt.Sprintf("<%s>", r))
	return m.WritePacket(NewResponse('+', data).Response())
}

/* DeleteRecipient removes a recipient address from message */
func (m *Modifier) DeleteRecipient(r string) error {
	data := []byte(fmt.Sprintf("<%s>", r))
	return m.WritePacket(NewResponse('-', data).Response())
}

/* ReplaceBody substitutes message body with provided body */
func (m *Modifier) ReplaceBody(body []byte) error {
	return m.WritePacket(NewResponse('b', body).Response())
}

/* AddHeader appends a new email message header to response message */
func (m *Modifier) AddHeader(name, value string) error {
	data := EncodeCStrings([]string{name, value})
	return m.WritePacket(NewResponse('h', data).Response())
}

/* ChangeHeader replaces the header at the position specified position with a new one */
func (m *Modifier) ChangeHeader(index int, name, value string) error {
	buffer := new(bytes.Buffer)
	// encode header index in the beginning
	if err := binary.Write(buffer, binary.BigEndian, uint32(index)); err != nil {
		return err
	}
	// add header name and value to buffer
	data := []string{name, value}
	if _, err := buffer.Write(EncodeCStrings(data)); err != nil {
		return err
	}
	// prepare and send response packet
	return m.WritePacket(NewResponse('m', buffer.Bytes()).Response())
}

/* NewModifier creates a new Modifier object */
func NewModifier(s *MilterSession) *Modifier {
	return &Modifier{
		Macros:      s.Macros,
		Headers:     s.Headers,
		Body:        s.Body,
		WritePacket: s.WritePacket,
	}
}