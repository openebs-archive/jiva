package controller

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/backend/remote"
	"github.com/openebs/jiva/types"
)

var (
	ErrNoBackend = errors.New("No backend available")
)

type replicator struct {
	backendsAvailable bool
	backends          map[string]backendWrapper
	quorumBackends    map[string]backendWrapper
	writerIndex       map[int]string
	updaterIndex      map[int]string
	readerIndex       map[int]string
	readers           []io.ReaderAt
	writer            io.WriterAt
	next              int
}

type BackendError struct {
	Errors map[string]error
}

func (b *BackendError) Error() string {
	errors := []string{}
	for address, err := range b.Errors {
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", address, err.Error()))
		}
	}

	switch len(errors) {
	case 0:
		return "Unknown"
	case 1:
		return errors[0]
	default:
		return strings.Join(errors, "; ")
	}
}

func (r *replicator) AddQuorumBackend(address string, backend types.Backend) {
	if _, ok := r.quorumBackends[address]; ok {
		return
	}

	logrus.Infof("Adding quorum backend: %s", address)

	if r.quorumBackends == nil {
		r.quorumBackends = map[string]backendWrapper{}
	}

	r.quorumBackends[address] = backendWrapper{
		backend: backend,
		mode:    types.WO,
	}

	r.buildReadWriters()
}

func (r *replicator) AddBackend(address string, backend types.Backend) {
	if _, ok := r.backends[address]; ok {
		logrus.Infof("backend: %s already part of backends", address)
		return
	}

	logrus.Infof("Adding backend: %s", address)

	if r.backends == nil {
		r.backends = map[string]backendWrapper{}
	}
	if r.quorumBackends == nil {
		r.quorumBackends = map[string]backendWrapper{}
	}

	r.backends[address] = backendWrapper{
		backend: backend,
		mode:    types.WO,
	}

	r.buildReadWriters()
}

func (r *replicator) RemoveBackend(address string) {
	backend, ok := r.backends[address]
	if !ok {
		backend, ok = r.quorumBackends[address]
		if !ok {
			logrus.Infof("RemoveBackend %v not found", address)
			return
		}
	}

	logrus.Infof("Remove backend: %s mode: %v", address, backend.mode)

	backend.backend.Close()
	delete(r.backends, address)
	r.buildReadWriters()
}

func (r *replicator) ReadAt(buf []byte, off int64) (int, error) {
	var (
		n   int
		err error
	)

	if !r.backendsAvailable {
		return 0, ErrNoBackend
	}

	readersLen := len(r.readers)
	r.next = (r.next + 1) % readersLen
	index := r.next
	retError := &BackendError{
		Errors: map[string]error{},
	}
	for i := 0; i < readersLen; i++ {
		reader := r.readers[index]
		n, err = reader.ReadAt(buf, off)
		if err == nil {
			break
		}
		logrus.Error("Replicator.ReadAt:", index, err)
		retError.Errors[r.readerIndex[index]] = err
		index = (index + 1) % readersLen
	}
	if len(retError.Errors) != 0 {
		return n, retError
	}
	return n, nil
}

func (r *replicator) WriteAt(p []byte, off int64) (int, error) {
	if !r.backendsAvailable {
		return 0, ErrNoBackend
	}

	n, err := r.writer.WriteAt(p, off)
	if err != nil {
		errors := map[string]error{}
		if mErr, ok := err.(*MultiWriterError); ok {
			for index, err := range mErr.ReplicaErrors {
				if err != nil {
					errors[r.writerIndex[index]] = err
				}
			}
		}
		return n, &BackendError{Errors: errors}
	}
	return n, err
}

func (r *replicator) buildReadWriters() {
	r.reset(false)

	readers := []io.ReaderAt{}
	writers := []io.WriterAt{}
	updaters := []io.WriterAt{}

	for address, b := range r.backends {
		if b.mode != types.ERR {
			r.writerIndex[len(writers)] = address
			writers = append(writers, b.backend)
		}
		if b.mode == types.RW {
			r.readerIndex[len(readers)] = address
			readers = append(readers, b.backend)
		}
	}
	for address, b := range r.quorumBackends {
		if b.mode != types.ERR {
			r.updaterIndex[len(updaters)] = address
			updaters = append(updaters, b.backend)
		}
	}

	var prevwriters int
	if r.writer != nil {
		multiwriter := r.writer.(*MultiWriterAt)
		prevwriters = len(multiwriter.writers)
	} else {
		prevwriters = 0
	}
	prevReaders := len(r.readers)
	r.writer = &MultiWriterAt{
		writers:  writers,
		updaters: updaters,
	}
	r.readers = readers
	multiwriter := r.writer.(*MultiWriterAt)

	if len(r.readers) > 0 {
		r.backendsAvailable = true
	}
	logrus.Infof("buildreadwriters: prev: %d %d cur: %d %d",
		prevwriters,
		prevReaders,
		len(multiwriter.writers),
		len(r.readers))
}

func (r *replicator) SetMode(address string, mode types.Mode) {
	b, ok := r.backends[address]
	if !ok {
		logrus.Infof("addr %v m: %v not found in setmode", address, mode)
		return
	}
	logrus.Infof("addr %v m: %v prev: %v in setmode", address, mode, b.mode)
	b.mode = mode
	r.backends[address] = b
	if mode == types.ERR {
		b.backend.StopMonitoring()
	}

	r.buildReadWriters()
}

func (r *replicator) Snapshot(name string, userCreated bool, created string) error {
	retErrorLock := sync.Mutex{}
	retError := &BackendError{
		Errors: map[string]error{},
	}
	wg := sync.WaitGroup{}
	var success int

	for addr, backend := range r.backends {
		if backend.mode != types.ERR {
			wg.Add(1)
			go func(address string, backend types.Backend) {
				if err := backend.Snapshot(name, userCreated, created); err != nil {
					logrus.Infof("failed taking snapshot at %s with err %v", addr, err)
					retErrorLock.Lock()
					retError.Errors[address] = err
					retErrorLock.Unlock()
				} else {
					retErrorLock.Lock()
					success = success + 1
					retErrorLock.Unlock()
				}
				wg.Done()
			}(addr, backend.backend)
		} else {
			logrus.Infof("not taking snapshot at %s in err mode", addr)
		}
	}

	wg.Wait()

	logrus.Infof("successfully taken snapshots cnt %d", success)
	if len(retError.Errors) != 0 {
		return retError
	}
	return nil
}
func (r *replicator) Resize(name string, size string) error {
	retErrorLock := sync.Mutex{}
	retError := &BackendError{
		Errors: map[string]error{},
	}
	wg := sync.WaitGroup{}

	for addr, backend := range r.backends {
		if backend.mode != types.ERR {
			wg.Add(1)
			go func(address string, backend types.Backend) {
				if err := backend.Resize(name, size); err != nil {
					retErrorLock.Lock()
					retError.Errors[address] = err
					retErrorLock.Unlock()
				}
				wg.Done()
			}(addr, backend.backend)
		}
	}

	wg.Wait()

	if len(retError.Errors) != 0 {
		return retError
	}
	return nil
}
func (r *replicator) Close() error {
	var lastErr error
	for _, backend := range r.backends {
		if backend.mode == types.ERR {
			continue
		}
		if err := backend.backend.Close(); err != nil {
			lastErr = err
		}
	}

	for _, backend := range r.quorumBackends {
		if backend.mode == types.ERR {
			continue
		}
		if err := backend.backend.Close(); err != nil {
			lastErr = err
		}
	}

	r.reset(true)

	return lastErr
}

func (r *replicator) reset(full bool) {
	logrus.Infof("replicator reset %v", full)
	r.backendsAvailable = false
	r.writer = nil
	r.writerIndex = map[int]string{}
	r.readerIndex = map[int]string{}
	r.updaterIndex = map[int]string{}

	if full {
		r.backends = nil
	}
}

func (r *replicator) GetVolUsage() (types.VolUsage, error) {
	var (
		err      error
		volUsage types.VolUsage
	)
	for _, backend := range r.backends {
		if backend.mode == types.ERR {
			continue
		}
		if volUsage, err = backend.backend.GetVolUsage(); err != nil {
			continue
		}
		return volUsage, err
	}
	return types.VolUsage{}, err
}

type backendWrapper struct {
	backend types.Backend
	mode    types.Mode
}

func (r *replicator) RemainSnapshots() (int, error) {
	// addReplica may call here even without any backend
	if len(r.backends) == 0 {
		return 1, nil
	}

	ret := math.MaxInt32
	for _, backend := range r.backends {
		if backend.mode == types.ERR {
			logrus.Infof("backend %v is in ERR mode.. cant find RemainSnapshots",
				(backend.backend.(*remote.Remote)).Name)
			continue
		}
		// ignore error and try next one. We can deal with all
		// error situation later
		if remain, err := backend.backend.RemainSnapshots(); err == nil {
			if remain < ret {
				ret = remain
			}
		}
	}
	if ret == math.MaxInt32 {
		return 0, fmt.Errorf("Cannot get valid result for remain snapshot from %d backends",
			len(r.backends))
	}
	return ret, nil
}

func (r *replicator) SetRevisionCounter(address string, counter int64) error {
	backend, ok := r.backends[address]
	if !ok {
		return fmt.Errorf("Cannot find backend %v", address)
	}

	if err := backend.backend.SetRevisionCounter(counter); err != nil {
		return err
	}

	logrus.Infof("Set backend %s revision counter to %v", address, counter)

	return nil
}

func (r *replicator) SetQuorumRevisionCounter(address string, counter int64) error {
	backend, ok := r.quorumBackends[address]
	if !ok {
		return fmt.Errorf("Cannot find backend %v", address)
	}

	if err := backend.backend.SetRevisionCounter(counter); err != nil {
		return err
	}

	logrus.Infof("Set backend %s revision counter to %v", address, counter)

	return nil
}

func (r *replicator) SetRebuilding(address string, rebuilding bool) error {
	backend, ok := r.backends[address]
	if !ok {
		backend, ok = r.quorumBackends[address]
		if !ok {
			return fmt.Errorf("Cannot find backend %v", address)
		}
	}

	if err := backend.backend.SetRebuilding(rebuilding); err != nil {
		return err
	}

	logrus.Infof("Set backend %s rebuilding %v", address, rebuilding)

	return nil

}
func (r *replicator) GetRevisionCounter(address string) (int64, error) {
	backend, ok := r.backends[address]
	if !ok {
		return -1, fmt.Errorf("Cannot find backend %v", address)
	}

	counter, err := backend.backend.GetRevisionCounter()
	if err != nil {
		return 0, err
	}
	logrus.Infof("Get backend %s revision counter %v", address, counter)

	return counter, nil
}

func (r *replicator) GetCloneStatus(address string) (string, error) {
	backend, ok := r.backends[address]
	if !ok {
		return "", fmt.Errorf("Cannot find backend %v", address)
	}

	status, err := backend.backend.GetCloneStatus()
	if err != nil {
		return "", err
	}
	logrus.Infof("Get backend %s clone status", address)

	return status, nil
}
