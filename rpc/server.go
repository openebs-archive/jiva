package rpc

import (
	"io"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/types"
)

type Server struct {
	wire      *Wire
	responses chan *Message
	done      chan struct{}
	data      types.DataProcessor
}

func NewServer(conn net.Conn, data types.DataProcessor) *Server {
	return &Server{
		wire:      NewWire(conn),
		responses: make(chan *Message, 1024),
		done:      make(chan struct{}, 5),
		data:      data,
	}
}

var monitorChan chan struct{}

func (s *Server) CreateMonitorChannel() chan struct{} {
	monitorChan = make(chan struct{}, 5)
	return monitorChan
}

func (s *Server) GetMonitorChannel() chan struct{} {
	return monitorChan
}

func (s *Server) Handle() error {
	go s.write()
	defer func() {
		s.done <- struct{}{}
	}()
	err := s.read()
	monitorChan <- struct{}{}
	return err
}

func (s *Server) readFromWire(ret chan<- error) {
	msg, err := s.wire.Read()
	if err == io.EOF {
		ret <- err
		return
	} else if err != nil {
		logrus.Errorf("Failed to read: %v", err)
		ret <- err
		return
	}
	switch msg.Type {
	case TypeRead:
		go s.handleRead(msg)
	case TypeWrite:
		go s.handleWrite(msg)
	case TypePing:
		go s.handlePing(msg)
		/*
			case TypeUpdate:
				go s.handleUpdate(msg)
		*/
	}
	ret <- nil
}

func (s *Server) read() error {
	ret := make(chan error)
	for {
		go s.readFromWire(ret)

		select {
		case err := <-ret:
			if err != nil {
				return err
			}
			continue
		case <-s.done:
			logrus.Debugf("RPC server stopped")
			return nil
		}
	}
}

func (s *Server) Stop() {
	s.done <- struct{}{}
}

func (s *Server) handleRead(msg *Message) {
	c, err := s.data.ReadAt(msg.Data, msg.Offset)
	s.pushResponse(c, msg, err)
}

func (s *Server) handleWrite(msg *Message) {
	c, err := s.data.WriteAt(msg.Data, msg.Offset)
	s.pushResponse(c, msg, err)
}

func (s *Server) handlePing(msg *Message) {
	err := s.data.PingResponse()
	s.pushResponse(0, msg, err)
}

/*
func (s *Server) handleUpdate(msg *Message) {
	err := s.data.Update()
	s.pushResponse(0, msg, err)
}
*/

func (s *Server) pushResponse(count int, msg *Message, err error) {
	msg.MagicVersion = MagicVersion
	msg.Type = TypeResponse
	if err == io.EOF {
		msg.Data = msg.Data[:count]
		msg.Type = TypeEOF
	} else if err != nil {
		msg.Type = TypeError
		msg.Data = []byte(err.Error())
	}
	s.responses <- msg
}

func (s *Server) write() {
	for {
		select {
		case msg := <-s.responses:
			if err := s.wire.Write(msg); err != nil {
				logrus.Errorf("Failed to write: %v", err)
			}
		case <-s.done:
			msg := &Message{
				Type: TypeClose,
			}
			//Best effort to notify client to close connection
			s.wire.Write(msg)
			break
		}
	}
}
