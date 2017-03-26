package replica

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/sparse-tools/sparse"
)

const (
	peerCounterFile             = "peer.counter"
	peerFileMode    os.FileMode = 0600
	peerBlockSize               = 4096
)

func (r *Replica) readPeerCounter() (int64, error) {
	if r.peerFile == nil {
		return 0, fmt.Errorf("BUG: peer file wasn't initialized")
	}

	buf := make([]byte, peerBlockSize)
	_, err := r.peerFile.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("fail to read from peer counter file: %v", err)
	}
	counter, err := strconv.ParseInt(strings.Trim(string(buf), "\x00"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("fail to parse peer counter file: %v", err)
	}
	return counter, nil
}

func (r *Replica) writePeerCounter(counter int64) error {
	if r.peerFile == nil {
		return fmt.Errorf("BUG: peer file wasn't initialized")
	}

	buf := make([]byte, peerBlockSize)
	copy(buf, []byte(strconv.FormatInt(counter, 10)))
	_, err := r.peerFile.WriteAt(buf, 0)
	if err != nil {
		return fmt.Errorf("fail to write to peer counter file: %v", err)
	}
	return nil
}

func (r *Replica) openPeerFile(isCreate bool) error {
	var err error
	r.peerFile, err = sparse.NewDirectFileIoProcessor(r.diskPath(peerCounterFile), os.O_RDWR, peerFileMode, isCreate)
	return err
}

func (r *Replica) initPeerCounter() error {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	if _, err := os.Stat(r.diskPath(peerCounterFile)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// file doesn't exist yet
		if err := r.openPeerFile(true); err != nil {
			return err
		}
		if err := r.writePeerCounter(0); err != nil {
			return err
		}
	} else if err := r.openPeerFile(false); err != nil {
		return err
	}

	counter, err := r.readPeerCounter()
	if err != nil {
		return err
	}
	// Don't use r.peerCache directly
	// r.peerCache is an internal cache, to avoid read from disk
	// everytime when counter needs to be updated.
	// And it's protected by peerLock
	r.peerCache = counter
	return nil
}

func (r *Replica) GetPeerCounter() int64 {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	counter, err := r.readPeerCounter()
	if err != nil {
		logrus.Error("Fail to get peer counter: ", err)
		// -1 will result in the replica to be discarded
		return -1
	}
	r.peerCache = counter
	return counter
}

func (r *Replica) SetPeerCounter(counter int64) error {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	if err := r.writePeerCounter(counter); err != nil {
		return err
	}

	r.peerCache = counter
	return nil
}

func (r *Replica) increasePeerCounter() error {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	if err := r.writePeerCounter(r.peerCache + 1); err != nil {
		return err
	}

	r.peerCache++
	return nil
}
