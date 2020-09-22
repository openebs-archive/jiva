/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package rpc

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type Wire struct {
	WriteLock           sync.Mutex
	ReadLock            sync.Mutex
	conn                net.Conn
	writer              *bufio.Writer
	reader              io.Reader
	readExit, writeExit bool
}

func NewWire(conn net.Conn) *Wire {
	return &Wire{
		conn:   conn,
		writer: bufio.NewWriterSize(conn, writeBufferSize),
		reader: bufio.NewReaderSize(conn, readBufferSize),
	}
}

func (w *Wire) Write(msg *Message) error {
	w.WriteLock.Lock()
	defer w.WriteLock.Unlock()
	if err := binary.Write(w.writer, binary.LittleEndian, msg.MagicVersion); err != nil {
		logrus.Errorf("Write MAgicVersion failed, Error: %v", err)
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, msg.Seq); err != nil {
		logrus.Errorf("Write msg.Seq failed, Error: %v", err)
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, msg.Type); err != nil {
		logrus.Errorf("Write msg.Type failed, Error: %v", err)
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, msg.Offset); err != nil {
		logrus.Errorf("Write msg.Offset failed, Error: %v", err)
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, msg.Size); err != nil {
		logrus.Errorf("Write msg.Size failed, Error: %v", err)
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, uint32(len(msg.Data))); err != nil {
		logrus.Errorf("Write len(msg.Data) failed, Error: %v", err)
		return err
	}
	if len(msg.Data) > 0 {
		if _, err := w.writer.Write(msg.Data); err != nil {
			logrus.Errorf("Write msg.Data failed, Error: %v", err)
			return err
		}
	}
	return w.writer.Flush()
}

func (w *Wire) Read() (*Message, error) {
	var (
		msg    Message
		length uint32
	)
	w.ReadLock.Lock()
	defer w.ReadLock.Unlock()

	if err := binary.Read(w.reader, binary.LittleEndian, &msg.MagicVersion); err != nil {
		logrus.Errorf("Read msg.Version failed, Error: %v", err)
		return nil, err
	}
	if msg.MagicVersion != MagicVersion {
		return &msg, fmt.Errorf("Wrong API version received: 0x%x", &msg.MagicVersion)
	}
	if err := binary.Read(w.reader, binary.LittleEndian, &msg.Seq); err != nil {
		logrus.Errorf("Read msg.Seq failed, Error: %v", err)
		return nil, err
	}

	if err := binary.Read(w.reader, binary.LittleEndian, &msg.Type); err != nil {
		logrus.Errorf("Read msg.Type failed, Error: %v", err)
		return nil, err
	}

	if err := binary.Read(w.reader, binary.LittleEndian, &msg.Offset); err != nil {
		logrus.Errorf("Read msg.Offset failed, Error: %v", err)
		return nil, err
	}
	if err := binary.Read(w.reader, binary.LittleEndian, &msg.Size); err != nil {
		logrus.Errorf("Read msg.Size failed, Error: %v", err)
		return nil, err
	}

	if err := binary.Read(w.reader, binary.LittleEndian, &length); err != nil {
		logrus.Errorf("Read length failed, Error: %v", err)
		return nil, err
	}
	if length > 0 {
		msg.Data = make([]byte, length)
		if _, err := io.ReadFull(w.reader, msg.Data); err != nil {
			logrus.Errorf("Read msg.Data failed, Error: %v", err)
			return nil, err
		}
	}

	return &msg, nil
}

func (w *Wire) CloseRead() error {
	if conn, ok := w.conn.(*net.TCPConn); ok {
		logrus.Info("Closing read on RPC connection")
		return conn.CloseRead()
	}
	return fmt.Errorf("failed to close read on RPC conn with replica: %v, type assert error", w.conn.RemoteAddr())
}

func (w *Wire) CloseWrite() error {
	if conn, ok := w.conn.(*net.TCPConn); ok {
		logrus.Info("Closing write on RPC connection")
		return conn.CloseWrite()
	}
	return fmt.Errorf("failed to close write on RPC conn with replica: %v, type assert error", w.conn.RemoteAddr())
}

func (w *Wire) Close() error {
	logrus.Warning("Closing RPC conn with replica: ", w.conn.RemoteAddr())
	return w.conn.Close()
}
