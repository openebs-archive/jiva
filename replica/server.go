package replica

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/types"
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
}

func NewServer(dir string, backing *BackingFile, sectorSize int64, serverType string) *Server {
	ActionChannel = make(chan string, 5)
	Dir = dir
	StartTime = time.Now()
	return &Server{
		dir:               dir,
		backing:           backing,
		defaultSectorSize: sectorSize,
		ServerType:        serverType,
		MonitorChannel:    make(chan struct{}),
	}
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

func (s *Server) Create(size int64) error {
	s.Lock()
	defer s.Unlock()
	if err := os.Mkdir(s.dir, 0700); err != nil && !os.IsExist(err) {
		logrus.Errorf("failed to create directory: %s", s.dir)
		return err
	}
	if err := isExtentSupported(s.dir); err != nil {
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
	r, err := New(size, sectorSize, s.dir, s.backing, s.ServerType)
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
	r, err := New(size, sectorSize, s.dir, s.backing, s.ServerType)
	if err != nil {
		logrus.Errorf("Error %v during open", err)
		return err
	}
	s.r = r
	return nil
}

func (s *Server) Reload() error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		logrus.Infof("returning as s.r is nil in reloading volume")
		return nil
	}

	logrus.Infof("Reloading volume")
	newReplica, err := s.r.Reload()
	if err != nil {
		logrus.Errorf("error in Reload")
		return err
	}

	oldReplica := s.r
	s.r = newReplica
	oldReplica.Close()
	return nil
}

func (s *Server) UpdateCloneInfo(snapName string) error {
	s.Lock()
	defer s.Unlock()

	if s.r == nil {
		return nil
	}

	logrus.Infof("Update Clone Info")
	return s.r.UpdateCloneInfo(snapName)
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

func (s *Server) Close(signalMonitor bool) error {
	logrus.Infof("Closing replica")
	s.Lock()

	if s.r == nil {
		logrus.Infof("Close replica failed, s.r not set")
		s.Unlock()
		return nil
	}

	if err := s.r.Close(); err != nil {
		s.Unlock()
		return err
	}

	s.r = nil
	s.Unlock()
	if signalMonitor {
		logrus.Infof("Signal MonitorChannel")
		s.MonitorChannel <- struct{}{}
	}
	return nil
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
