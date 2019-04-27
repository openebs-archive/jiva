/*
Copyright 2016 openebs authors All rights reserved.

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

package backingstore

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/openebs/gotgt/pkg/api"
	"github.com/openebs/gotgt/pkg/scsi"
)

func init() {
	scsi.RegisterBackingStore("RemBs", newRemBs)
}

type RemBackingStore struct {
	scsi.BaseBackingStore
	RemBs api.IOs
}

func newRemBs() (api.BackingStore, error) {
	return &RemBackingStore{
		BaseBackingStore: scsi.BaseBackingStore{
			Name:            "RemBs",
			OflagsSupported: 0,
		},
	}, nil
}

func (bs *RemBackingStore) Open(dev *api.SCSILu, path string) error {
	bs.DataSize = uint64(dev.Size)
	bs.RemBs = scsi.GetTargetBSMap(path)
	return nil
}

func (bs *RemBackingStore) Close(dev *api.SCSILu) error {
	/* TODO return bs.File.Close()*/
	return nil
}

func (bs *RemBackingStore) Init(dev *api.SCSILu, Opts string) error {
	return nil
}

func (bs *RemBackingStore) Exit(dev *api.SCSILu) error {
	return nil
}

func (bs *RemBackingStore) Size(dev *api.SCSILu) uint64 {
	return bs.DataSize
}

func (bs *RemBackingStore) Read(offset, tl int64) ([]byte, error) {
	if bs.RemBs == nil {
		return nil, fmt.Errorf("Backend store is nil")
	}
	tmpbuf := make([]byte, tl)
	length, err := bs.RemBs.ReadAt(tmpbuf, offset)
	if err != nil {
		return nil, err
	}
	if length != len(tmpbuf) {
		return nil, fmt.Errorf("Incomplete read expected:%d actual:%d", tl, length)
	}
	return tmpbuf, nil
}

func (bs *RemBackingStore) Write(wbuf []byte, offset int64) error {
	length, err := bs.RemBs.WriteAt(wbuf, offset)
	if err != nil {
		log.Error(err)
		return err
	}
	if length != len(wbuf) {
		return fmt.Errorf("Incomplete write expected:%d actual:%d", len(wbuf), length)
	}
	return nil
}

func (bs *RemBackingStore) DataAdvise(offset, length int64, advise uint32) error {
	return nil
}

func (bs *RemBackingStore) DataSync() (err error) {
	_, err = bs.RemBs.Sync()
	return
}
