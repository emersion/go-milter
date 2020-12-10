package milter

import (
	"errors"
	"net"
)

// Milter protocol version implemented by the server.
//
// Note: Not exported as we might want to support multiple versions
// transparently in the future.
var serverProtocolVersion uint32 = 2

// ErrServerClosed is returned by the Server's Serve method after a call to
// Close.
var ErrServerClosed = errors.New("milter: server closed")

// Server is a milter server.
type Server struct {
	NewMilter func() Milter
	Actions   OptAction
	Protocol  OptProtocol

	listeners []net.Listener
	closed    bool
}

// Serve starts the server.
func (s *Server) Serve(ln net.Listener) error {
	defer ln.Close()

	s.listeners = append(s.listeners, ln)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.closed {
				return ErrServerClosed
			}
			return err
		}

		session := milterSession{
			actions:  s.Actions,
			protocol: s.Protocol,
			conn:     conn,
			backend:  s.NewMilter(),
		}
		go session.HandleMilterCommands()
	}
}

func (s *Server) Close() error {
	s.closed = true
	for _, ln := range s.listeners {
		if err := ln.Close(); err != nil {
			return err
		}
	}
	return nil
}
