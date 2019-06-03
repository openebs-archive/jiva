package rpc

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/types"
)

type operation func(*Message)

var (
	errInvalidReq = errors.New("Received invalid req")
)

type Server struct {
	wire        *Wire
	responses   chan *Message
	done        chan struct{}
	data        types.DataProcessor
	monitorChan chan struct{}
	// pingRecvd is the time connection get established
	// with the client and get updated each time when ping
	// is received from client.
	pingRecvd time.Time
	rwExit    bool
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
	ret := make(chan error, 1)
	go s.readWrite(ret)
	for {
		select {
		case err = <-ret:
			if err != nil {
				if err == errInvalidReq {
					logrus.Warningf("Failed to serve client: %v, rejecting request, err: %v", s.wire.conn.RemoteAddr(), err)
					// ignore error, connection may be closed from the other side.
					_ = s.Stop() // close connection with the invalid client
					// return nil since, no need to close files
					return nil
				}
				return err
			}
		case <-ticker.C:
			if s.pingRecvd.IsZero() {
				// ignore if no ping received, i.e, replica is
				// opening and preload is going on.
				break
			}
			since := time.Since(s.pingRecvd)
			if since >= opPingTimeout*2 {
				return fmt.Errorf("Couldn't received ping since: %v", since)
			}
		}
	}
}

// timed is wrapper over various operations to varify whether
// those operations took more than the expected time to complete.
func timed(f operation, msg *Message) {
	msgType := msg.Type
	start := time.Now()
	f(msg)
	timeSinceStart := time.Since(start)
	switch msgType {
	case TypeRead:
		if timeSinceStart > opReadTimeout {
			logrus.Warningf("Read time: %vs greater than read timeout: %v at controller", timeSinceStart.Seconds(), opReadTimeout)
		}
	case TypeWrite:
		if timeSinceStart > opWriteTimeout {
			logrus.Warningf("Write time: %vs greater than write timeout: %v at controller", timeSinceStart.Seconds(), opWriteTimeout)
		}
	case TypeSync:
		if timeSinceStart > opSyncTimeout {
			logrus.Warningf("Sync time: %vs greater than sync timeout: %v at controller", timeSinceStart.Seconds(), opSyncTimeout)
		}
	case TypeUnmap:
		if timeSinceStart > opUnmapTimeout {
			logrus.Warningf("Unmap time: %vs greater than unmap timeout: %v at controller", timeSinceStart.Seconds(), opUnmapTimeout)
		}
	case TypePing:
		if timeSinceStart > opPingTimeout {
			logrus.Warningf("Ping time: %vs greater than ping timeout: %v at controller", timeSinceStart.Seconds(), opPingTimeout)
		}
	}
}

func (s *Server) readWrite(ret chan<- error) {
	for {
		inject.AddPingTimeout()
		msg, err := s.wire.Read()
		if err == io.EOF {
			logrus.Errorf("Received EOF: %v", err)
			ret <- err
			break
		} else if err != nil {
			logrus.Errorf("Failed to read: %v", err)
			// check if client is valid, since MagicVersion will always
			// be a constant for valid clients such as jiva controller.
			// This is done to ignore the requests from invalid clients
			// such as Prometheus which by default scraps from all the open
			// ports.
			msg, errRead := s.wire.ReadMessage()
			if errRead == nil {
				if msg.MagicVersion != MagicVersion {
					ret <- errInvalidReq
					break
				}
			}
			ret <- err
			break
		}

		switch msg.Type {
		case TypeRead:
			timed(s.handleRead, msg)
		case TypeWrite:
			timed(s.handleWrite, msg)
		case TypePing:
			timed(s.handlePing, msg)
		case TypeSync:
			timed(s.handleSync, msg)
		case TypeUnmap:
			timed(s.handleUnmap, msg)
			/*
				case TypeUpdate:
					go s.handleUpdate(msg)
			*/
			break
		}

		if err := s.write(msg); err != nil {
			ret <- err
			break
		}
	}
	logrus.Error("Closing rpc server")
	s.rwExit = true
}

func (s *Server) isIOError(err error) bool {
	if err1, ok := err.(*os.PathError); ok {
		switch err1.Err.(syscall.Errno) {
		case syscall.EIO:
			return true
		}
	}
	return false
}

func (s *Server) Stop() error {
	if err := s.wire.CloseRead(); err != nil {
		return err
	}
	if err := s.wire.CloseWrite(); err != nil {
		return err
	}
	for {
		if s.rwExit {
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
	if err != nil {
		logrus.Errorf("Failed to read data, error: %v", err)
		if s.isIOError(err) {
			logrus.Fatal("Exiting...")
		}
	}
}

func (s *Server) handleWrite(msg *Message) {
	c, err := s.data.WriteAt(msg.Data, msg.Offset)
	s.createResponse(c, msg, err)
	if err != nil {
		logrus.Errorf("Failed to write data, error: %v", err)
		if s.isIOError(err) {
			logrus.Fatal("Exiting...")
		}
	}
}

func (s *Server) handlePing(msg *Message) {
	err := s.data.PingResponse()
	s.createResponse(0, msg, err)
	s.pingRecvd = time.Now()
}

func (s *Server) handleSync(msg *Message) {
	_, err := s.data.Sync()
	s.createResponse(0, msg, err)
	if err != nil {
		logrus.Errorf("Failed to sync data, error: %v", err)
		if s.isIOError(err) {
			logrus.Fatal("Exiting...")
		}
	}
}
func (s *Server) handleUnmap(msg *Message) {
	_, err := s.data.Unmap(msg.Offset, msg.Size)
	s.createResponse(0, msg, err)
	if err != nil {
		logrus.Errorf("Failed to unmap data, error: %v", err)
		if s.isIOError(err) {
			logrus.Fatal("Exiting...")
		}
	}
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
