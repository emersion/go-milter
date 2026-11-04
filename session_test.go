package milter

import (
	"io"
	"net"
	"testing"
)

func TestMilterSessionMalformedCommands(t *testing.T) {
	tests := []struct {
		name string
		code Code
		data string
	}{
		{"empty connect", CodeConn, ""},
		{"unterminated hostname", CodeConn, "host"},
		{"empty hostname without family", CodeConn, "\x00"},
		{"missing family", CodeConn, "host\x00"},
		{"empty macro", CodeMacro, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			peerErr := make(chan error, 1)
			go func() {
				defer c1.Close()
				msg := &Message{Code: byte(tc.code), Data: []byte(tc.data)}
				if err := writePacket(c1, msg, 0); err != nil {
					peerErr <- err
					return
				}
				var response [1]byte
				_, err := c1.Read(response[:])
				peerErr <- err
			}()

			session := milterSession{conn: c2, backend: NoOpMilter{}, server: &Server{}}
			session.HandleMilterCommands()
			if err := <-peerErr; err != io.EOF {
				t.Fatal("Expected connection close without response, got:", err)
			}
		})
	}
}

func TestMilterSessionConnectUnknown(t *testing.T) {
	mm := &MockMilter{}
	session := milterSession{backend: mm}
	_, err := session.Process(&Message{Code: byte(CodeConn), Data: []byte("host\x00U")})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if mm.Host != "host" || mm.Family != "unknown" || mm.Port != 0 || mm.Addr != nil {
		t.Fatalf("Unexpected connection: %+v", mm)
	}
}

func TestMilterSessionEmptyMacros(t *testing.T) {
	session := milterSession{}
	resp, err := session.Process(&Message{Code: byte(CodeMacro), Data: []byte{byte(CodeConn)}})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if resp != nil || len(session.macros) != 0 {
		t.Fatal("Expected empty macros and no response")
	}
}
