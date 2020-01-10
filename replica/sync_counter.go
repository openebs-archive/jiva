package replica

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/openebs/jiva/types"

	"github.com/longhorn/sparse-tools/sparse"
	"github.com/sirupsen/logrus"
)

const (
	syncCounterFile             = "sync.counter"
	syncFileMode    os.FileMode = 0600
	syncBlockSize               = 4096
)

func (r *Replica) readSyncCounter() (int64, error) {
	if r.syncFile == nil {
		return 0, fmt.Errorf("BUG: sync file wasn't initialized")
	}

	buf := make([]byte, syncBlockSize)
	_, err := r.syncFile.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("fail to read from sync counter file: %v", err)
	}
	counter, err := strconv.ParseInt(strings.Trim(string(buf), "\x00"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("fail to parse sync counter file: %v", err)
	}
	return counter, nil
}

func (r *Replica) writeSyncCounter(counter int64) error {
	if r.syncFile == nil {
		return fmt.Errorf("BUG: sync file wasn't initialized")
	}

	buf := make([]byte, syncBlockSize)
	copy(buf, []byte(strconv.FormatInt(counter, 10)))
	_, err := r.syncFile.WriteAt(buf, 0)
	if err != nil {
		return fmt.Errorf("fail to write to sync counter file: %v", err)
	}

	return nil
}

func (r *Replica) openSyncFile(isCreate bool) error {
	var err error
	r.syncFile, err = sparse.NewDirectFileIoProcessor(r.diskPath(syncCounterFile), os.O_RDWR, syncFileMode, isCreate)
	return err
}

func (r *Replica) initSyncCounter() error {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()

	if _, err := os.Stat(r.diskPath(syncCounterFile)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// file doesn't exist yet
		if err := r.openSyncFile(true); err != nil {
			logrus.Errorf("failed to open sync counter file")
			return err
		}
		if err := r.writeSyncCounter(1); err != nil {
			logrus.Errorf("failed to update sync counter")
			return err
		}
	} else if err := r.openSyncFile(false); err != nil {
		logrus.Errorf("open existing sync counter file failed")
		return err
	}

	counter, err := r.readSyncCounter()
	if err != nil {
		logrus.Errorf("failed to read sync counter")
		return err
	}
	// Don't use r.syncCache directly
	// r.syncCache is an internal cache, to avoid read from disk
	// everytime when counter needs to be updated.
	// And it's protected by syncLock
	r.syncCache = counter
	return nil
}

func (r *Replica) GetSyncCounter() int64 {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()

	counter, err := r.readSyncCounter()
	if err != nil {
		logrus.Error("Fail to get sync counter: ", err)
		// -1 will result in the replica to be discarded
		return -1
	}
	r.syncCache = counter
	return counter
}

func (r *Replica) SetSyncCounter(counter int64) error {
	r.Lock()
	if r.mode != types.RW {
		r.Unlock()
		return fmt.Errorf("setting synccounter during %v mode is invalid", r.mode)
	}
	r.Unlock()
	r.syncLock.Lock()
	defer r.syncLock.Unlock()

	if err := r.writeSyncCounter(counter); err != nil {
		return err
	}

	r.syncCache = counter
	return nil
}

func (r *Replica) increaseSyncCounter() error {
	r.syncLock.Lock()
	defer r.syncLock.Unlock()

	if err := r.writeSyncCounter(r.syncCache + 1); err != nil {
		return err
	}

	r.syncCache++
	return nil
}
