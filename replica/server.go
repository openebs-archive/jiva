package replica

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	fibmap "github.com/frostschutz/go-fibmap"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/types"
	"github.com/sirupsen/logrus"
)

const (
	Initial    = State("initial")
	Open       = State("open")
	Closed     = State("closed")
	Dirty      = State("dirty")
	Rebuilding = State("rebuilding")
	Error      = State("error")
)

type State string

var ActionChannel chan string
var Dir string
var StartTime time.Time

type Server struct {
	sync.RWMutex
	r                 *Replica
	ServerType        string
	dir               string
	defaultSectorSize int64
	backing           *BackingFile
	//This channel is used to montitor the IO connection
	//between controller and replica. If the connection is broken,
	//the replica attempts to connect back to controller
	MonitorChannel chan struct{}
	//closeSync      chan struct{}
	preload bool
}

func NewServer(dir string, sectorSize int64, serverType string) *Server {
	ActionChannel = make(chan string, 5)
	Dir = dir
	StartTime = time.Now()
	return &Server{
		dir:               dir,
		defaultSectorSize: sectorSize,
		ServerType:        serverType,
		MonitorChannel:    make(chan struct{}),
		preload:           true,
		//	closeSync:         make(chan struct{}),
	}
}

// SetPreload sets/unsets preloadDuringOpen flag
func (s *Server) SetPreload(preload bool) error {
	s.Lock()
	defer s.Unlock()
	s.preload = preload
	return nil
}

func (s *Server) Start(action string) error {
	s.Lock()
	defer s.Unlock()
	ActionChannel <- action

	return nil
}

func (s *Server) getSectorSize() int64 {
	if s.backing != nil && s.backing.SectorSize > 0 {
		return s.backing.SectorSize
	}
	return s.defaultSectorSize
}

func (s *Server) getSize(size int64) int64 {
	if s.backing != nil && s.backing.Size > 0 {
		return s.backing.Size
	}
	return size
}

func (s *Server) createTempFile(filePath string) (*os.File, error) {
	var file *os.File
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(filePath)
			if err != nil {
				return nil, err
			}
			return file, nil
		}
		return nil, err
	}
	// Open file in case file exists (not removed)
	file, err = os.OpenFile(filePath, os.O_RDWR|os.O_SYNC, 0600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Server) isExtentSupported() error {
	filePath := s.dir + "/tmpFile.tmp"
	file, err := s.createTempFile(filePath)
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Remove(filePath)
	}()

	_, err = file.WriteString("This is temp file\n")
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	fiemapFile := fibmap.NewFibmapFile(file)
	if _, errno := fiemapFile.Fiemap(uint32(fileInfo.Size())); errno != 0 {
		// verify is FIBMAP is supported incase FIEMAP failed
		if _, err := fiemapFile.FibmapExtents(); err != 0 {
			return fmt.Errorf("failed to find extents, error: %v", err.Error())
		}
		return errno
	}
	return file.Close()
}

func (s *Server) Create(size int64) error {
	s.Lock()
	defer s.Unlock()
	if err := s.isExtentSupported(); err != nil {
		return err
	}
	state, _ := s.Status()

	if state != Initial {
		fmt.Println("STATE = ", state)
		return nil
	}

	size = s.getSize(size)
	sectorSize := s.getSectorSize()

	logrus.Infof("Creating volume %s, size %d/%d", s.dir, size, sectorSize)
	// Preload is not needed over here since the volume is being newly created
	r, err := New(false, size, sectorSize, s.dir, s.backing, s.ServerType)
	if err != nil {
		return err
	}

	return r.Close()
}

func (s *Server) Open() error {
	s.Lock()
	defer s.Unlock()

	if s.r != nil {
		return fmt.Errorf("Replica is already open")
	}

	_, info := s.Status()
	size := s.getSize(info.Size)
	sectorSize := s.getSectorSize()
	logrus.Infof("Opening volume %s, size %d/%d", s.dir, size, sectorSize)
	r, err := New(s.preload, size, sectorSize, s.dir, s.backing, s.ServerType)
	if err != nil {
		logrus.Errorf("Error %v during open", err)
		return err
	}
	s.r = r
	return nil
}

/*
TODO: Enabling periodic sync will slow down replica a bit
need to verify how much penalty we have to pay by Enabling it.
func (s *Server) periodicSync() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	logrus.Info("Start periodic sync")
	for {
		select {
		case <-s.closeSync:
			logrus.Info("Stop periodic sync")
			return
		case <-ticker.C:
			s.RLock()
			if s.r == nil {
				logrus.Warning("Stop periodic sync as s.r not set")
				s.RUnlock()
				return
			}
			if _, err := s.r.Sync(); err != nil {
				logrus.Warningf("Fail to sync, err: %v", err)
			}
			s.RUnlock()
		}
	}
}
*/

func (s *Server) Reload() error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("returning as s.r is nil in reloading volume")
		return nil
	}

	types.ShouldPunchHoles = true
	logrus.Infof("Reloading volume")
	newReplica, err := s.r.Reload(s.preload)
	if err != nil {
		logrus.Errorf("error in Reload")
		types.ShouldPunchHoles = false
		return err
	}

	oldReplica := s.r
	s.r = newReplica
	oldReplica.Close()
	return nil
}

func (s *Server) UpdateCloneInfo(snapName, revCount string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		return nil
	}

	logrus.Infof("Update Clone Info")
	return s.r.UpdateCloneInfo(snapName, revCount)
}

func (s *Server) Status() (State, Info) {
	if s.r == nil {
		info, err := ReadInfo(s.dir)
		if os.IsNotExist(err) {
			return Initial, Info{}
		} else if err != nil {
			return Error, Info{}
		}
		return Closed, info
	}
	info := s.r.Info()
	switch {
	case info.Rebuilding:
		return Rebuilding, info
	case info.Dirty:
		return Dirty, info
	default:
		return Open, info
	}
}

func (s *Server) PrevStatus() (State, Info) {
	info, err := ReadInfo(s.dir)
	if os.IsNotExist(err) {
		return Initial, Info{}
	} else if err != nil {
		return Error, Info{}
	}
	switch {
	case info.Rebuilding:
		return Rebuilding, info
	case info.Dirty:
		return Dirty, info
	}

	return Closed, info
}

// Stats returns the revisionCache and Peerdetails
// TODO: What to return in Stats and GetUsage if s.r is nil?
func (s *Server) Stats() *types.Stats {
	r := s.r
	var revisionCache int64

	revisionCache = 0
	if r != nil {
		revisionCache = r.revisionCache
	}

	stats1 := &types.Stats{
		RevisionCounter: revisionCache,
	}
	return stats1
}

// GetRevisionCounter reads the revison counter
func (s *Server) GetRevisionCounter() (int64, error) {
	tmpReplica := Replica{
		dir: Dir,
	}
	err := tmpReplica.initRevisionCounter()
	if err != nil {
		return 0, err
	}
	return tmpReplica.revisionCache, nil
}

// GetUsage returns the used size of volume
func (s *Server) GetUsage() (*types.VolUsage, error) {
	if s.r != nil {
		return s.r.GetUsage()
	}
	return &types.VolUsage{
		UsedLogicalBlocks: 0,
		UsedBlocks:        0,
		SectorSize:        0,
	}, nil
}

func (s *Server) SetRebuilding(rebuilding bool) error {
	s.Lock()
	defer s.Unlock()

	state, _ := s.Status()
	// Must be Open/Dirty to set true or must be Rebuilding to set false
	if (rebuilding && state != Open && state != Dirty) ||
		(!rebuilding && state != Rebuilding) {
		return fmt.Errorf("Can not set rebuilding=%v from state %s", rebuilding, state)
	}

	return s.r.SetRebuilding(rebuilding)
}

func (s *Server) Resize(size string) error {
	s.Lock()
	defer s.Unlock()
	return s.r.Resize(size)
}

func (s *Server) Replica() *Replica {
	return s.r
}

func (s *Server) Revert(name, created string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("Revert is not performed as s.r is nil")
		return nil
	}

	logrus.Infof("Reverting to snapshot [%s] on volume at %s", name, created)
	r, err := s.r.Revert(name, created)
	if err != nil {
		return err
	}

	s.r = r
	return nil
}

// UpdateLUNMap updates the original LUNmap with any changes which have been
// encountered after PreloadVolume. This can be called only once just after
// reload is called after syncing the files from healthy replicas
func (s *Server) UpdateLUNMap() error {
	// With this lock r.volume data structure is copied, and a new slice is
	// created different than the one in r.volume.location. After this we will
	// be having 2 LUNMaps, one being filled by the parallel write operations
	// and the other being filled by preload operation.
	s.Lock()
	volume := s.r.volume
	volume.location = make([]uint16, len(s.r.volume.location))
	s.Unlock()
	// LUNmap is populated with the extents after the sync operation is
	// completed
	if err := PreloadLunMap(&volume); err != nil {
		return err
	}
	inject.AddUpdateLUNMapTimeout()
	s.Lock()
	var (
		holeLength          int64
		holeOffset          int
		fileIndx            uint16
		prevHoleFileIndx    uint16
		offset              int
		userCreatedSnapIndx uint16
	)
	// userCreatedSnapIndx holds the latest user created snapshot index
	for i, isUserCreated := range volume.UserCreatedSnap {
		if isUserCreated {
			userCreatedSnapIndx = uint16(i)
		}
	}

	// While this loop is being executed IOs have been stopped(s.Lock()) which
	// inturn halts the updates on the original LunMap.
	// Empty offsets in the original LunMap are filled by corresponding entries
	// in preloaded lunmap.
	// If offsets are present in both LunMaps and are different,
	// hole is punched in the file at that offset contained in Preloaded LunMap.
	// Sequesnce of holes are being punched at once.
	var extraUsedBlocks, extraLogicalBlocks int64

	for offset, fileIndx = range volume.location {
		if fileIndx == 0 {
			if s.r.volume.location[offset] != 0 {
				extraUsedBlocks++
				extraLogicalBlocks++
			}
			continue
		}
		if s.r.volume.location[offset] > fileIndx {
			// It is being incremented and decremented to accomodate user
			// created snapshots
			if prevHoleFileIndx != fileIndx || int64(offset) != int64(holeOffset)+holeLength {
				if prevHoleFileIndx > userCreatedSnapIndx && shouldCreateHoles() && prevHoleFileIndx != 0 {
					extraUsedBlocks -= holeLength
					sendToCreateHole(volume.files[prevHoleFileIndx], int64(holeOffset)*volume.sectorSize, holeLength*volume.sectorSize)
				}
				holeLength = 1
				holeOffset = offset
				prevHoleFileIndx = fileIndx
			} else {
				holeLength++
			}
			extraUsedBlocks++
		} else {
			// No hole drilling over here as that offset is empty
			s.r.volume.location[offset] = volume.location[offset]
			if prevHoleFileIndx > userCreatedSnapIndx && shouldCreateHoles() && prevHoleFileIndx != 0 {
				extraUsedBlocks -= holeLength
				sendToCreateHole(volume.files[prevHoleFileIndx], int64(holeOffset)*volume.sectorSize, holeLength*volume.sectorSize)
			}
			holeOffset = 0
			holeLength = 0
			prevHoleFileIndx = 0
		}
	}
	if prevHoleFileIndx > userCreatedSnapIndx && shouldCreateHoles() && prevHoleFileIndx != 0 {
		extraUsedBlocks -= holeLength
		sendToCreateHole(volume.files[prevHoleFileIndx], int64(holeOffset)*volume.sectorSize, holeLength*volume.sectorSize)
	}
	s.r.volume.UsedLogicalBlocks = volume.UsedLogicalBlocks + extraLogicalBlocks
	s.r.volume.UsedBlocks = volume.UsedBlocks + extraUsedBlocks
	s.Unlock()
	return nil
}

func (s *Server) Snapshot(name string, userCreated bool, createdTime string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("snapshot is not performed as s.r is nil")
		return nil
	}

	logrus.Infof("Snapshotting [%s] volume, user created %v, created time %v",
		name, userCreated, createdTime)
	return s.r.Snapshot(name, userCreated, createdTime)
}

func (s *Server) RemoveDiffDisk(name string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("RemoveDiffDisk is not performed as s.r is nil")
		return nil
	}

	logrus.Infof("Removing disk: %s", name)
	return s.r.RemoveDiffDisk(name)
}

func (s *Server) ReplaceDisk(target, source string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("ReplicaDisk is not performed as s.r is nil")
		return nil
	}

	logrus.Infof("Replacing disk %v with %v", target, source)
	return s.r.ReplaceDisk(target, source)
}

func (s *Server) PrepareRemoveDisk(name string) ([]PrepareRemoveAction, error) {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("PrepareRemoveDisk is not performed as s.r is nil")
		return nil, nil
	}

	logrus.Infof("Prepare removing disk: %s", name)
	return s.r.PrepareRemoveDisk(name)
}

// CheckPreDeleteConditions checks if any replica exists.
// If it exists, it closes all the connections with the replica
// and deletes the entry from the controller.
func (s *Server) CheckPreDeleteConditions() error {
	if s.r == nil {
		return errors.New("s.r is nil")
	}

	logrus.Infof("Closing volume")
	if err := s.r.Close(); err != nil {
		return err
	}
	return nil
}

// Delete deletes the volume metadata and revision counter file.
func (s *Server) Delete() error {
	s.Lock()
	defer s.Unlock()

	err := s.CheckPreDeleteConditions()
	if err != nil {
		return err
	}

	logrus.Info("Delete the metadata and revision counter file")
	err = s.r.Delete()
	s.r = nil
	return err
}

// DeleteAll deletes all the contents of the mounted directory.
func (s *Server) DeleteAll() error {
	s.Lock()
	defer s.Unlock()

	err := s.CheckPreDeleteConditions()
	if err != nil {
		return err
	}

	logrus.Infof("Deleting all the contents of the volume")
	err = s.r.DeleteAll()
	s.r = nil
	return err
}

// Close drain the data from HoleCreatorChan and close
// all the associated files with the replica instance.
func (s *Server) Close() error {
	logrus.Infof("Closing replica")
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("Skip closing replica, s.r not set")
		return nil
	}

	// r.holeDrainer is initialized at construct
	// function in replica.go
	s.r.holeDrainer()
	// notify periodicSync go routine to stop
	//s.closeSync <- struct{}{}
	if err := s.r.Close(); err != nil {
		logrus.Errorf("Failed to close replica, err: %v", err)
		return err
	}

	s.r = nil
	return nil
}

func (s *Server) Sync() (int, error) {
	s.RLock()
	defer s.RUnlock()

	if s.r == nil {
		return -1, fmt.Errorf("Volume no longer exist")
	}
	n, err := s.r.Sync()
	return n, err
}
func (s *Server) Unmap(offset int64, length int64) (int, error) {
	s.RLock()
	defer s.RUnlock()

	if s.r == nil {
		return -1, fmt.Errorf("Volume no longer exist")
	}
	n, err := s.r.Unmap(offset, length)
	return n, err
}

func (s *Server) WriteAt(buf []byte, offset int64) (int, error) {
	s.RLock()
	defer s.RUnlock()

	if s.r == nil {
		return 0, fmt.Errorf("Volume no longer exist")
	}
	i, err := s.r.WriteAt(buf, offset)
	return i, err
}

func (s *Server) ReadAt(buf []byte, offset int64) (int, error) {
	s.RLock()
	defer s.RUnlock()

	if s.r == nil {
		return 0, fmt.Errorf("Volume no longer exist")
	}
	i, err := s.r.ReadAt(buf, offset)
	return i, err
}

// SetReplicaMode ...
func (s *Server) SetReplicaMode(mode string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("s.r is nil during setReplicaMode")
		return nil
	}
	return s.r.SetReplicaMode(mode)
}

func (s *Server) SetRevisionCounter(counter int64) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("s.r is nil during setRevisionCounter")
		return nil
	}
	return s.r.SetRevisionCounter(counter)
}

func (s *Server) PingResponse() error {
	state, _ := s.Status()
	if state != Open && state != Dirty && state != Rebuilding {
		return fmt.Errorf("ping failure: replica state %v", state)
	}
	return nil
}
