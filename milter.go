// Package milter provides an interface to implement milter mail filters
package milter

import (
	"net"
	"net/textproto"
)

// Backend is an interface for milter callback handlers
type Backend interface {
	// Connect is called to provide SMTP connection data for incoming message
	//   supress with NoConnect
	Connect(host string, family string, port uint16, addr net.IP, m *Modifier) (Response, error)

	// Helo is called to process any HELO/EHLO related filters
	//   supress with NoHelo
	Helo(name string, m *Modifier) (Response, error)

	// MailFrom is called to process filters on envelope FROM address
	//   supress with NoMailForm
	MailFrom(from string, m *Modifier) (Response, error)

	// RcptTo is called to process filters on envelope TO address
	//   supress with NoRcptTo
	RcptTo(rcptTo string, m *Modifier) (Response, error)

	// Header is called once for each header in incoming message
	//   supress with NoHeaders
	Header(name string, value string, m *Modifier) (Response, error)

	// Headers is called when all message headers have been processed
	//   supress with NoHeaders
	Headers(h textproto.MIMEHeader, m *Modifier) (Response, error)

	// BodyChunk is called to process next message body chunk data (up to 64KB in size)
	//   supress with NoBody
	BodyChunk(chunk []byte, m *Modifier) (Response, error)

	// Body is called at the end of each message
	//   all changes to message's content & attributes must be done here
	Body(m *Modifier) (Response, error)
}
