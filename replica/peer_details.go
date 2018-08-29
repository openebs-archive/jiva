package replica

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/types"
	"github.com/rancher/sparse-tools/sparse"
)

const (
	peerDetailsFile             = "peer.details"
	peerFilMode     os.FileMode = 0600
)

func (r *Replica) readPeerDetails() (types.PeerDetails, error) {
	if r.peerFile == nil {
		return types.PeerDetails{}, fmt.Errorf("BUG: peer file wasn't initialized")
	}
	var peerDetails types.PeerDetails

	if err := r.unmarshalFile(peerDetailsFile, &peerDetails); err != nil {
		return types.PeerDetails{}, err
	}

	return peerDetails, nil
}

func (r *Replica) writePeerDetails(peerDetails types.PeerDetails) error {
	if r.peerFile == nil {
		return fmt.Errorf("BUG: peer file wasn't initialized")
	}
	return r.encodeToFile(&peerDetails, peerDetailsFile)
}

func (r *Replica) openPeerFile(isCreate bool) error {
	var err error
	r.peerFile, err = sparse.NewDirectFileIoProcessor(r.diskPath(peerDetailsFile), os.O_RDWR, peerFilMode, isCreate)
	return err
}

func (r *Replica) initPeerDetails() error {
	var peerDetails types.PeerDetails
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	if _, err := os.Stat(r.diskPath(peerDetailsFile)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// file doesn't exist yet
		if err := r.openPeerFile(true); err != nil {
			logrus.Errorf("openPeerFile failed %v", err)
			return err
		}
		if err := r.writePeerDetails(peerDetails); err != nil {
			logrus.Errorf("writePeerDetails failed %v", err)
			return err
		}
	} else if err := r.openPeerFile(false); err != nil {
		logrus.Errorf("OpenPeerFile failed when file already exists %v", err)
		return err
	}

	/*peerDetails, err := r.readPeerDetails()
	if err != nil {
		logrus.Errorf("readPeerDetails failed %v", err)
		return err
	}
	*/
	// Don't use r.peerCache directly
	// r.peerCache is an internal cache, to avoid read from disk
	// everytime when counter needs to be updated.
	// And it's protected by peerLock
	//r.peerCache = peerDetails
	return nil
}

func (r *Replica) GetPeerDetails() (types.PeerDetails, error) {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	peerDetails, err := r.readPeerDetails()
	if err != nil {
		logrus.Error("Fail to get peerDetails: ", err)
		// -1 will result in the replica to be discarded
		return types.PeerDetails{}, err
	}
	r.peerCache = peerDetails
	return peerDetails, nil
}

/*
func (r *Replica) UpdatePeerDetails(peerDetails types.PeerDetails) error {
	r.peerLock.Lock()
	defer r.peerLock.Unlock()

	if err := r.writePeerDetails(peerDetails); err != nil {
		return err
	}

	r.peerCache = peerDetails
	return nil
}*/
