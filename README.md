# go-milter

[![GoDoc](https://godoc.org/github.com/emersion/go-milter?status.svg)](https://godoc.org/github.com/emersion/go-milter)
[![builds.sr.ht status](https://builds.sr.ht/~emersion/go-milter/commits.svg)](https://builds.sr.ht/~emersion/go-milter/commits?)

A Go library to write mail filters.

## Server Example

```go
package main

import (
	"log"
	"net"
	"net/textproto"

	"github.com/emersion/go-milter"
)

type backend struct{}

func (backend) Connect(host string, family string, port uint16, addr net.IP, m *milter.Modifier) (milter.Response, error) {
	// TODO Connect
	return milter.RespContinue, nil
}

func (backend) Helo(name string, m *milter.Modifier) (milter.Response, error) {
	// TODO Helo
	return milter.RespContinue, nil
}

func (backend) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	// TODO MailFrom
	return milter.RespContinue, nil
}

func (backend) RcptTo(rcptTo string, m *milter.Modifier) (milter.Response, error) {
	// TODO RcptTo
	return milter.RespContinue, nil
}

func (backend) Header(name string, value string, m *milter.Modifier) (milter.Response, error) {
	// TODO Header
	return milter.RespContinue, nil
}

func (backend) Headers(h textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	// TODO Headers
	return milter.RespContinue, nil
}

func (backend) BodyChunk(chunk []byte, m *milter.Modifier) (milter.Response, error) {
	// TODO BodyChunk
	return milter.RespContinue, nil
}

func (backend) Body(m *milter.Modifier) (milter.Response, error) {
	// TODO Body
	return milter.RespAccept, nil
}

func main() {
	server := milter.NewDefaultServer(func() milter.Milter {
		return backend{}
	})

	listener, err := net.Listen("tcp4", "127.0.0.1:5000")
	if err != nil {
		log.Fatal(err)
	}

	if err = server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
```

## License

BSD 2-Clause
