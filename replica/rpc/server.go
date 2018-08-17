package rpc

import (
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/rpc"
)

type Server struct {
	address string
	s       *replica.Server
}

func New(address string, s *replica.Server) *Server {
	return &Server{
		address: address,
		s:       s,
	}
}

func (s *Server) ListenAndServe() error {
	addr, err := net.ResolveTCPAddr("tcp", s.address)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}
		err = conn.SetKeepAlive(true)
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}

		err = conn.SetKeepAlivePeriod(10 * time.Second)
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}

		logrus.Infof("New connection from: %v", conn.RemoteAddr())

		go func(conn net.Conn) {
			server := rpc.NewServer(conn, s.s)
			server.SetMonitorChannel(s.s.MonitorChannel)
			server.Handle()
		}(conn)
	}
}
