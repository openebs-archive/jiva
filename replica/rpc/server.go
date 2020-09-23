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
	"net"
	"time"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/rpc"
	"github.com/sirupsen/logrus"
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

		server := rpc.NewServer(conn, s.s)
		if err := server.Handle(); err != nil {
			// ignore err for below operations,as connection may be
			// closed from the other side and also files may have
			// been closed already or not initialized yet.
			_ = server.Stop() // shutdown fd and conn
			_ = s.s.Close()   // close all the open files before fataling
			logrus.Fatalf("Failed to handle connection, err: %v, shutdown replica...", err)
		}
	}
}
