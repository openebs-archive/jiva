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

package replica

import (
	"fmt"
	"syscall"
	"time"

	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/types"
	"github.com/openebs/sparse-tools/sparse"
	"github.com/sirupsen/logrus"
)

const (
	snapBlockSize = 2 << 20 // 2MiB
)

/*
type DeltaBlockBackupOperations interface {
	HasSnapshot(id, volumeID string) bool
	CompareSnapshot(id, compareID, volumeID string) (*metadata.Mappings, error)
	OpenSnapshot(id, volumeID string) error
	ReadSnapshot(id, volumeID string, start int64, data []byte) error
	CloseSnapshot(id, volumeID string) error
}
*/
/*
type Backup struct {
	backingFile *BackingFile
	replica     *Replica
	volumeID    string
	snapshotID  string
}

func NewBackup(backingFile *BackingFile) *Backup {
	return &Backup{
		backingFile: backingFile,
	}
}

func (rb *Backup) HasSnapshot(id, volumeID string) bool {
	//TODO Check current in the volume directory of volumeID
	if err := rb.assertOpen(id, volumeID); err != nil {
		return false
	}

	to := rb.findIndex(id)
	if to < 0 {
		return false
	}
	return true
}

func (rb *Backup) OpenSnapshot(id, volumeID string) error {
	if rb.volumeID == volumeID && rb.snapshotID == id {
		return nil
	}

	if rb.volumeID != "" {
		return fmt.Errorf("Volume %s and snapshot %s are already open, close first", rb.volumeID, rb.snapshotID)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Cannot get working directory: %v", err)
	}
	r, err := NewReadOnly(dir, id, rb.backingFile)
	if err != nil {
		return err
	}

	rb.replica = r
	rb.volumeID = volumeID
	rb.snapshotID = id

	return nil
}

func (rb *Backup) assertOpen(id, volumeID string) error {
	if rb.volumeID != volumeID || rb.snapshotID != id {
		return fmt.Errorf("Invalid state volume [%s] and snapshot [%s] are open, not [%s], [%s]", rb.volumeID, rb.snapshotID, id, volumeID)
	}
	return nil
}

func (rb *Backup) ReadSnapshot(id, volumeID string, start int64, data []byte) error {
	if err := rb.assertOpen(id, volumeID); err != nil {
		return err
	}

	if rb.snapshotID != id && rb.volumeID != volumeID {
		return fmt.Errorf("Snapshot %s and volume %s are not open", id, volumeID)
	}

	_, err := rb.replica.ReadAt(data, start)
	return err
}

func (rb *Backup) CloseSnapshot(id, volumeID string) error {
	if rb.volumeID == "" {
		return nil
	}

	if err := rb.assertOpen(id, volumeID); err != nil {
		return err
	}

	err := rb.replica.Close()

	rb.replica = nil
	rb.volumeID = ""
	rb.snapshotID = ""
	return err
}

func (rb *Backup) CompareSnapshot(id, compareID, volumeID string) (*backupstore.Mappings, error) {
	if err := rb.assertOpen(id, volumeID); err != nil {
		return nil, err
	}

	rb.replica.Lock()
	defer rb.replica.Unlock()

	from := rb.findIndex(id)
	if from < 0 {
		return nil, fmt.Errorf("Failed to find snapshot %s in chain", id)
	}

	to := rb.findIndex(compareID)
	if to < 0 {
		return nil, fmt.Errorf("Failed to find snapshot %s in chain", compareID)
	}

	mappings := &backupstore.Mappings{
		BlockSize: snapBlockSize,
	}
	mapping := backupstore.Mapping{
		Offset: -1,
	}

	if err := preload(&rb.replica.volume); err != nil {
		return nil, err
	}

	for i, val := range rb.replica.volume.location {
		if val <= uint16(from) && val > uint16(to) {
			offset := int64(i) * rb.replica.volume.sectorSize
			// align
			offset -= (offset % snapBlockSize)
			if mapping.Offset != offset {
				mapping = backupstore.Mapping{
					Offset: offset,
					Size:   snapBlockSize,
				}
				mappings.Mappings = append(mappings.Mappings, mapping)
			}
		}
	}

	return mappings, nil
}

func (rb *Backup) findIndex(id string) int {
	if id == "" {
		if rb.backingFile == nil {
			return 0
		}
		return 1
	}

	for i, disk := range rb.replica.activeDiskData {
		if i == 0 {
			continue
		}
		if disk.Name == id {
			return i
		}
	}
	return -1
}
*/

// Hole holds the fd, len and offset for fallocate operation
type Hole struct {
	f      types.DiffDisk
	offset int64
	len    int64
}

var HoleCreatorChan = make(chan Hole, 1000000)

func holeDrainer() {
	types.DrainOps = types.DrainStart
	HoleCreatorChan <- Hole{}
	for {
		if types.DrainOps == types.DrainDone {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

// drainHoleCreatorChan is called if replica is closed, to drain
// all the data for punching hole in the HoleCreatorChan.
func drainHoleCreatorChan() {
	for {
		select {
		case <-HoleCreatorChan:
		default:
			return
		}
	}
}

// CreateHoles removes the offsets from corresponding sparse files
func CreateHoles() {
	var (
		fd uintptr
	)
	retryCount := 0
	for {
		hole := <-HoleCreatorChan
		inject.AddPunchHoleTimeout()
		if types.DrainOps == types.DrainStart {
			drainHoleCreatorChan()
			types.DrainOps = types.DrainDone
			continue
		}
		if (Hole{}) == hole {
			continue
		}
		fd = hole.f.Fd()
	retry:
		if err := syscall.Fallocate(int(fd),
			sparse.FALLOC_FL_KEEP_SIZE|sparse.FALLOC_FL_PUNCH_HOLE,
			hole.offset, hole.len); err != nil {
			logrus.Errorf("ERROR in creating hole: %v, Retry_Count: %v", err, retryCount)
			time.Sleep(1)
			retryCount++
			if retryCount == 5 {
				logrus.Fatalf("Error Creating holes: %v", err)
			}
			goto retry
		}
		retryCount = 0
	}
}

func sendToCreateHole(f types.DiffDisk, offset int64, len int64) {
	hole := Hole{
		f:      f,
		offset: offset,
		len:    len,
	}
	HoleCreatorChan <- hole
}

func shouldCreateHoles() bool {
	if types.ShouldPunchHoles {
		return true
	}
	return false
}

// preload creates a mapping of block number to fileIndx (d.location).
// This is done with the help of extent list fetched from filesystem.
// Extents list in each file is traversed and the location table is updated
// TODO: Visit this function again in case of optimization while preload call.
func preload(d *diffDisk) error {
	var file types.DiffDisk
	var length int64
	var lOffset int64
	var fileIndx uint16
	// userCreatedSnapIndx represents the index of the latest User Created
	// snapshot traversed till this point. This value is used instead of
	// d.SnapIndx because this will also aid in removing duplicate blocks
	// in auto-created snapshots between 2 user created snapshots
	var userCreatedSnapIndx uint16
	for i, f := range d.files {
		if i == 0 {
			continue
		}
		if d.UserCreatedSnap[i] {
			userCreatedSnapIndx = uint16(i)
		}
		generator := newGenerator(d, f)
		for offset := range generator.Generate() {
			if d.location[offset] != 0 {
				// d.UsedBlocks is being incremented and decremented to accomodate user
				// created snapshots
				// We are looking for continuous blocks over here.
				// If the file of the next block is changed, we punch a hole
				// for the previous unpunched blocks, and reset the file and
				// fileIndx pointed to by this block
				if d.files[d.location[offset]] != file ||
					offset != lOffset+length {
					if file != nil && fileIndx > userCreatedSnapIndx && shouldCreateHoles() {
						d.UsedBlocks -= length
						sendToCreateHole(file, lOffset*d.sectorSize, length*d.sectorSize)
					}
					file = d.files[d.location[offset]]
					fileIndx = d.location[offset]
					length = 1
					lOffset = offset
				} else {
					// If this is the last block in the loop, hole for this
					// block will be punched outside the loop
					length++
				}
			} else {
				d.UsedLogicalBlocks++
			}
			d.location[offset] = uint16(i)
			d.UsedBlocks++
		}
		// This will take care of the case when the last call in the above loop
		// enters else case
		if file != nil && fileIndx > userCreatedSnapIndx && shouldCreateHoles() {
			d.UsedBlocks -= length
			sendToCreateHole(file, lOffset*d.sectorSize, length*d.sectorSize)
		}
		file = nil
		fileIndx = 0

		if generator.Err() != nil {
			fmt.Println("GENERATOR ERROR")
			return generator.Err()
		}
	}

	return nil
}

func PreloadLunMap(d *diffDisk) error {
	logrus.Info("Start reading extents")
	inject.AddPreloadTimeout()
	if err := preload(d); err != nil {
		return err
	}
	logrus.Info("Read extents successful")
	return nil
}
