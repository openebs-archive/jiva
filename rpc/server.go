package rpc

import (
	"io"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/types"
)

type Server struct {
	wire                *Wire
	responses           chan *Message
	done                chan struct{}
	data                types.DataProcessor
	monitorChan         chan struct{}
	pingStart           time.Time
	readExit, writeExit bool
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
		err    error
		ticker = time.NewTicker(5 * time.Second)
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
		case err = <-ret:
			if err != nil {
				if err := s.Stop(); err != nil {
					logrus.Error("Failed to stop rpc server, error: ", err)
					return err
				}
				return err
			}
		case <-ticker.C:
			if time.Since(s.pingStart) >= opPingTimeout*2 {
				// Close the connection as Ping is not recieved
				// since long time.
				if err := s.Stop(); err != nil {
					return err
				}
			}
		}
	}
}

func (s *Server) readWrite(ret chan<- error) {
	s.pingStart = time.Now()
	for {
		inject.AddPingTimeout()
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
		case TypeSync:
			s.handleSync(msg)
		case TypeUnmap:
			s.handleUnmap(msg)
			/*
				case TypeUpdate:
					go s.handleUpdate(msg)
			*/
		}

		if s.readExit {
			logrus.Warning("Can't write, read is closed on rpc conn")
			break
		}
		if err := s.write(msg); err != nil {
			ret <- err
			break
		}
	}
	logrus.Error("Closing rpc server")
	s.writeExit = true
}

func (s *Server) Stop() error {
	if err := s.wire.CloseRead(); err != nil {
		return err
	}
	s.readExit = true
	if err := s.wire.CloseWrite(); err != nil {
		return err
	}
	for {
		if s.writeExit {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return s.wire.Close()
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
	s.pingStart = time.Now()
}

func (s *Server) handleSync(msg *Message) {
	_, err := s.data.Sync()
	s.createResponse(0, msg, err)
}
func (s *Server) handleUnmap(msg *Message) {
	_, err := s.data.Unmap(msg.Offset, msg.Size)
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
	msg.Size = int64(len(msg.Data))
	if msg.Type == TypeWrite {
		msg.Data = nil
	}
	msg.Type = TypeResponse
	if err == io.EOF {
		msg.Type = TypeEOF
		if msg.Data != nil {
			msg.Data = msg.Data[:count]
		}
		msg.Size = int64(len(msg.Data))
	} else if err != nil {
		msg.Type = TypeError
		msg.Data = []byte(err.Error())
		msg.Size = int64(len(msg.Data))
	}
}

func (s *Server) write(msg *Message) error {
	if err := s.wire.Write(msg); err != nil {
		logrus.Errorf("Failed to write: %v", err)
		return err
	}
	return nil
}
