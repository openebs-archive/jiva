package rpc

import (
	"io"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/types"
)

type Server struct {
	wire        *Wire
	responses   chan *Message
	done        chan struct{}
	data        types.DataProcessor
	monitorChan chan struct{}
}

func NewServer(conn net.Conn, data types.DataProcessor) *Server {
	return &Server{
		wire:      NewWire(conn),
		responses: make(chan *Message, 1024),
		done:      make(chan struct{}, 5),
		data:      data,
	}
}

func (s *Server) SetMonitorChannel(monitorChan chan struct{}) {
	s.monitorChan = monitorChan
}

func (s *Server) Handle() error {
	var (
		msg *Message
		err error
	)
	defer func() {
		select {
		case s.monitorChan <- struct{}{}:
		default:
		}
	}()
	ret := make(chan error)
	go s.readWrite(ret)

	for {
		select {
		case <-s.done:
			msg = &Message{
				Type: TypeClose,
			}
			//Best effort to notify client to close connection
			s.write(msg)
			return nil
		case err = <-ret:
			if err != nil {
				return err
			}
		}
	}
}

func (s *Server) readWrite(ret chan<- error) {
	for {
		msg, err := s.wire.Read()
		if err == io.EOF {
			logrus.Errorf("Received EOF: %v", err)
			ret <- err
			break
		} else if err != nil {
			logrus.Errorf("Failed to read: %v", err)
			ret <- err
			break
		}
		switch msg.Type {
		case TypeRead:
			s.handleRead(msg)
		case TypeWrite:
			s.handleWrite(msg)
		case TypePing:
			s.handlePing(msg)
			/*
				case TypeUpdate:
					go s.handleUpdate(msg)
			*/
		}
		if err := s.write(msg); err != nil {
			ret <- err
			break
		}
	}
}

func (s *Server) Stop() {
	s.done <- struct{}{}
}

func (s *Server) handleRead(msg *Message) {
	msg.Data = make([]byte, msg.Size)
	c, err := s.data.ReadAt(msg.Data, msg.Offset)
	s.createResponse(c, msg, err)
}

func (s *Server) handleWrite(msg *Message) {
	c, err := s.data.WriteAt(msg.Data, msg.Offset)
	s.createResponse(c, msg, err)
}

func (s *Server) handlePing(msg *Message) {
	err := s.data.PingResponse()
	s.createResponse(0, msg, err)
}

/*
func (s *Server) handleUpdate(msg *Message) {
	err := s.data.Update()
	s.pushResponse(0, msg, err)
}
*/

func (s *Server) createResponse(count int, msg *Message, err error) {
	msg.MagicVersion = MagicVersion
	msg.Size = uint32(len(msg.Data))
	if msg.Type == TypeWrite {
		msg.Data = nil
	}
	msg.Type = TypeResponse
	if err == io.EOF {
		msg.Type = TypeEOF
		if msg.Data != nil {
			msg.Data = msg.Data[:count]
		}
		msg.Size = uint32(len(msg.Data))
	} else if err != nil {
		msg.Type = TypeError
		msg.Data = []byte(err.Error())
		msg.Size = uint32(len(msg.Data))
	}
}

func (s *Server) write(msg *Message) error {
	if err := s.wire.Write(msg); err != nil {
		logrus.Errorf("Failed to write: %v", err)
		return err
	}
	return nil
}
