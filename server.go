package milter

import (
	"net"
)

// Server is a milter server.
type Server struct {
	NewMilter func() Milter
	Actions OptAction
	Protocol OptProtocol
}

// Serve starts the server.
func (s *Server) Serve(l net.Listener) error {
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		session := milterSession{
			actions:   s.Actions,
			protocol:  s.Protocol,
			conn:      conn,
			backend:   s.NewMilter(),
		}
		go session.HandleMilterCommands()
	}
}
