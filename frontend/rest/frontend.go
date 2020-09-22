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

package rest

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/openebs/jiva/types"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithFields(logrus.Fields{"pkg": "rest-frontend"})
)

type Device struct {
	Name       string
	Size       int64
	SectorSize int64

	isUp    bool
	backend types.ReaderWriterAt
}

func New() types.Frontend {
	return &Device{}
}

func (d *Device) Startup(name string, frontendIP string, clusterIP string, size, sectorSize int64, rw types.IOs) error {
	d.Name = name
	d.backend = rw
	d.Size = size
	d.SectorSize = sectorSize

	if err := d.start(); err != nil {
		return err
	}

	d.isUp = true
	return nil
}

func (d *Device) Shutdown() error {
	return d.stop()
}

func (d *Device) start() error {
	listen := "localhost:9414"
	server := NewServer(d)
	router := http.Handler(NewRouter(server))
	router = handlers.LoggingHandler(os.Stdout, router)
	router = handlers.ProxyHeaders(router)

	log.Infof("Rest Frontend listening on %s", listen)

	go func() {
		http.ListenAndServe(listen, router)
	}()
	return nil
}

func (d *Device) stop() error {
	d.isUp = false
	return nil
}

func (d *Device) State() types.State {
	if d.isUp {
		return types.StateUp
	}
	return types.StateDown
}

func (d *Device) Stats() types.Stats {
	return types.Stats{}
}

func (d *Device) Resize(size uint64) error {
	return nil
}
