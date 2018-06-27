package controller

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	units "github.com/docker/go-units"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
)

type Controller struct {
	sync.RWMutex
	Name                     string
	frontendIP               string
	clusterIP                string
	size                     int64
	sectorSize               int64
	replicas                 []types.Replica
	replicaCount             int
	quorumReplicas           []types.Replica
	quorumReplicaCount       int
	factory                  types.BackendFactory
	backend                  *replicator
	frontend                 types.Frontend
	RegisteredReplicas       map[string]types.RegReplica
	RegisteredQuorumReplicas map[string]types.RegReplica
	MaxRevReplica            string
	StartTime                time.Time
	StartSignalled           bool
	ReadOnly                 bool
}

func (c *Controller) GetSize() int64 {
	return c.size
}

func NewController(name string, frontendIP string, clusterIP string, factory types.BackendFactory, frontend types.Frontend) *Controller {
	c := &Controller{
		factory:                  factory,
		Name:                     name,
		frontend:                 frontend,
		frontendIP:               frontendIP,
		clusterIP:                clusterIP,
		RegisteredReplicas:       map[string]types.RegReplica{},
		RegisteredQuorumReplicas: map[string]types.RegReplica{},
		StartTime:                time.Now(),
	}
	c.reset()
	return c
}

func (c *Controller) AddQuorumReplica(address string) error {
	return c.addQuorumReplica(address, false)
}

func (c *Controller) AddReplica(address string) error {
	return c.addReplica(address, true)
}

func (c *Controller) RegisterReplica(register types.RegReplica) error {
	return c.registerReplica(register)
}

func (c *Controller) hasWOReplica() bool {
	for _, i := range c.replicas {
		if i.Mode == types.WO {
			return true
		}
	}
	return false
}

func (c *Controller) canAdd(address string) (bool, error) {
	if c.hasReplica(address) {
		return false, nil
	}
	if c.hasWOReplica() {
		return false, fmt.Errorf("Can only have one WO replica at a time")
	}
	return true, nil
}

func (c *Controller) getRWReplica() (*types.Replica, error) {
	var (
		rwReplica *types.Replica
	)

	for i := range c.replicas {
		if c.replicas[i].Mode == types.RW {
			rwReplica = &c.replicas[i]
		}
	}
	if rwReplica == nil {
		return nil, fmt.Errorf("Cannot find any healthy replica")
	}

	return rwReplica, nil
}

func (c *Controller) addQuorumReplica(address string, snapshot bool) error {
	c.Lock()
	if ok, err := c.canAdd(address); !ok {
		c.Unlock()
		return err
	}
	c.Unlock()

	newBackend, err := c.factory.Create(address)
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	err = c.addQuorumReplicaNoLock(newBackend, address, snapshot)
	if err != nil {
		return err
	}

	if err := c.backend.SetRebuilding(address, true); err != nil {
		return fmt.Errorf("Failed to set rebuild : %v", true)
	}
	rwReplica, err := c.getRWReplica()
	if err != nil {
		return err
	}

	counter, err := c.backend.GetRevisionCounter(rwReplica.Address)
	if err != nil || counter == -1 {
		return fmt.Errorf("Failed to get revision counter of RW Replica %v: counter %v, err %v",
			rwReplica.Address, counter, err)

	}

	if err := c.backend.SetQuorumRevisionCounter(address, counter); err != nil {
		return fmt.Errorf("Fail to set revision counter for %v: %v", address, err)
	}

	if err := c.backend.UpdatePeerDetails(c.replicaCount, c.quorumReplicaCount); err != nil {
		return fmt.Errorf("Fail to set revision counter for %v: %v", address, err)
	}

	if err := c.backend.SetRebuilding(address, false); err != nil {
		return fmt.Errorf("Failed to set rebuild : %v", true)
	}

	if (len(c.replicas)+len(c.quorumReplicas) > (c.replicaCount+c.quorumReplicaCount)/2) && (c.ReadOnly == true) {
		logrus.Infof("Marking volume as R/W")
		c.ReadOnly = false
	}

	/*
		for _, temprep := range c.replicas {
			if err := c.backend.SetQuorumReplicaCounter(temprep.Address, int64(len(c.replicas))); err != nil {
				return fmt.Errorf("Fail to set replica counter for %v: %v", address, err)
			}
		}
	*/
	return nil
}

func (c *Controller) addReplica(address string, snapshot bool) error {
	c.Lock()
	if ok, err := c.canAdd(address); !ok {
		c.Unlock()
		return err
	}
	c.Unlock()

	newBackend, err := c.factory.Create(address)
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	err = c.addReplicaNoLock(newBackend, address, snapshot)
	if (len(c.replicas)+len(c.quorumReplicas) > (c.replicaCount+c.quorumReplicaCount)/2) && (c.ReadOnly == true) {
		logrus.Infof("Marking volume as R/W")
		c.ReadOnly = false
	}
	return err
}

func (c *Controller) signalToAdd() {
	c.factory.SignalToAdd(c.MaxRevReplica, "start")
}

func (c *Controller) registerReplica(register types.RegReplica) error {
	c.Lock()
	defer c.Unlock()
	logrus.Infof("Register Replica, Address: %v Uptime: %v State: %v Type: %v RevisionCount: %v PeerCount: %v",
		register.Address, register.UpTime, register.RepState, register.RepType, register.RevCount, register.PeerDetail.ReplicaCount)

	_, ok := c.RegisteredReplicas[register.Address]
	if !ok {
		_, ok = c.RegisteredQuorumReplicas[register.Address]
		if ok {
			return nil
		}
	}
	if c.quorumReplicaCount < register.PeerDetail.QuorumReplicaCount {
		c.quorumReplicaCount = register.PeerDetail.QuorumReplicaCount
	}
	if c.replicaCount < register.PeerDetail.ReplicaCount {
		c.replicaCount = register.PeerDetail.ReplicaCount
	}

	if register.RepType == "quorum" {
		c.RegisteredQuorumReplicas[register.Address] = register
		return nil
	}
	c.RegisteredReplicas[register.Address] = register

	if len(c.replicas) > 0 {
		return nil
	}
	if c.StartSignalled == true {
		if c.MaxRevReplica == register.Address {
			c.signalToAdd()
		} else {
			return nil
		}
	}
	if register.RepState == "rebuilding" {
		logrus.Errorf("Cannot add replica in rebuilding state")
		return nil
	}

	if c.MaxRevReplica == "" {
		c.MaxRevReplica = register.Address
	}

	if c.RegisteredReplicas[c.MaxRevReplica].RevCount < register.RevCount {
		c.MaxRevReplica = register.Address
	}

	if ((len(c.RegisteredReplicas)) >= c.replicaCount/2) && ((len(c.RegisteredReplicas) + len(c.RegisteredQuorumReplicas)) > (c.quorumReplicaCount+c.replicaCount)/2) {
		c.signalToAdd()
		c.StartSignalled = true
		return nil
	}

	if c.RegisteredReplicas[c.MaxRevReplica].PeerDetail.ReplicaCount <= 1 {
		c.signalToAdd()
		c.StartSignalled = true
		return nil
	}

	/*
		//TODO Improve on this logic for HA
		if register.UpTime > time.Since(c.StartTime) && (c.StartSignalled == false || c.MaxRevReplica == register.Address) {
			c.signalToAdd()
			c.StartSignalled = true
			return nil
		}
	*/
	/*
		if len(c.RegisteredReplicas) >= 2 {
			for _, tmprep := range c.RegisteredReplicas {
				if revCount == 0 {
					revCount = tmprep.RevCount
				} else {
					if revCount != tmprep.RevCount {
						revisionConflict = 1
					}
				}
			}
			if revisionConflict == 0 {
				c.signalToAdd()
				c.StartSignalled = true
				return nil
			}
		}
	*/
	return nil
}

func (c *Controller) Snapshot(name string) (string, error) {
	c.Lock()
	defer c.Unlock()

	if name == "" {
		name = util.UUID()
	}

	if remain, err := c.backend.RemainSnapshots(); err != nil {
		return "", err
	} else if remain <= 0 {
		return "", fmt.Errorf("Too many snapshots created")
	}

	created := util.Now()
	return name, c.handleErrorNoLock(c.backend.Snapshot(name, true, created))
}

func (c *Controller) Resize(name string, size string) error {
	var (
		sizeInBytes int64
		err         error
	)
	c.Lock()
	defer c.Unlock()

	if name != c.Name {
		return fmt.Errorf("Volume name didn't match")
	}
	if size != "" {
		sizeInBytes, err = units.RAMInBytes(size)
		if err != nil {
			return err
		}
	}
	if sizeInBytes < c.size {
		return fmt.Errorf("Size can only be increased, not reduced")
	} else if sizeInBytes == c.size {
		return fmt.Errorf("Volume size same as size mentioned")
	}
	err = c.handleErrorNoLock(c.backend.Resize(name, size))
	if err != nil {
		return err
	}

	if c.frontend != nil {
		err = c.frontend.Resize(uint64(sizeInBytes))
		if err != nil {
			return err
		}
	}
	c.size = sizeInBytes
	return nil
}

func (c *Controller) addQuorumReplicaNoLock(newBackend types.Backend, address string, snapshot bool) error {
	if ok, err := c.canAdd(address); !ok {
		return err
	}

	if snapshot {
		uuid := util.UUID()
		created := util.Now()

		if remain, err := c.backend.RemainSnapshots(); err != nil {
			return err
		} else if remain <= 0 {
			return fmt.Errorf("Too many snapshots created")
		}

		if err := c.backend.Snapshot(uuid, false, created); err != nil {
			newBackend.Close()
			return err
		}
		if err := newBackend.Snapshot(uuid, false, created); err != nil {
			newBackend.Close()
			return err
		}
	}

	c.quorumReplicas = append(c.quorumReplicas, types.Replica{
		Address: address,
		Mode:    types.WO,
	})
	c.quorumReplicaCount++

	c.backend.AddQuorumBackend(address, newBackend)

	go c.monitoring(address, newBackend)

	return nil
}

func (c *Controller) addReplicaNoLock(newBackend types.Backend, address string, snapshot bool) error {
	if ok, err := c.canAdd(address); !ok {
		return err
	}

	if snapshot {
		uuid := util.UUID()
		created := util.Now()

		if remain, err := c.backend.RemainSnapshots(); err != nil {
			return err
		} else if remain <= 0 {
			return fmt.Errorf("Too many snapshots created")
		}

		if err := c.backend.Snapshot(uuid, false, created); err != nil {
			newBackend.Close()
			return err
		}
		if err := newBackend.Snapshot(uuid, false, created); err != nil {
			newBackend.Close()
			return err
		}
	}

	c.replicas = append(c.replicas, types.Replica{
		Address: address,
		Mode:    types.WO,
	})

	c.backend.AddBackend(address, newBackend)

	go c.monitoring(address, newBackend)

	return nil
}

func (c *Controller) hasReplica(address string) bool {
	for _, i := range c.replicas {
		if i.Address == address {
			return true
		}
	}
	for _, i := range c.quorumReplicas {
		if i.Address == address {
			return true
		}
	}
	return false
}

func (c *Controller) RemoveReplica(address string) error {
	c.Lock()
	defer c.Unlock()

	if !c.hasReplica(address) {
		return nil
	}
	for i, r := range c.replicas {
		if r.Address == address {
			if len(c.replicas) == 1 && c.frontend.State() == types.StateUp {
				if r.Mode == "ERR" {
					if c.frontend != nil {
						c.StartSignalled = false
						c.MaxRevReplica = ""
						c.frontend.Shutdown()
					}
				} else {
					return fmt.Errorf("Cannot remove last replica if volume is up")
				}
			}
			for regrep := range c.RegisteredReplicas {
				if strings.Contains(address, regrep) {
					delete(c.RegisteredReplicas, regrep)
				}
			}
			c.replicas = append(c.replicas[:i], c.replicas[i+1:]...)
			c.backend.RemoveBackend(r.Address)
		}
	}

	for i, r := range c.quorumReplicas {
		if r.Address == address {
			for regrep := range c.RegisteredQuorumReplicas {
				if strings.Contains(address, regrep) {
					delete(c.RegisteredQuorumReplicas, regrep)
				}
			}
			c.quorumReplicas = append(c.quorumReplicas[:i], c.quorumReplicas[i+1:]...)
			c.backend.RemoveBackend(r.Address)
		}
	}

	if len(c.replicas)+len(c.quorumReplicas) <= (c.replicaCount+c.quorumReplicaCount)/2 {
		logrus.Infof("Marking volume as R/O")
		c.ReadOnly = true
	}
	return nil
}

func (c *Controller) ListReplicas() []types.Replica {
	return c.replicas
}

func (c *Controller) ListQuorumReplicas() []types.Replica {
	return c.quorumReplicas
}

func (c *Controller) SetReplicaMode(address string, mode types.Mode) error {
	switch mode {
	case types.ERR:
		c.Lock()
		defer c.Unlock()
	case types.RW:
		c.RLock()
		defer c.RUnlock()
	default:
		return fmt.Errorf("Can not set to mode %s", mode)
	}
	c.setReplicaModeNoLock(address, mode)
	return nil
}

func (c *Controller) setReplicaModeNoLock(address string, mode types.Mode) {
	for i, r := range c.replicas {
		if r.Address == address {
			if r.Mode != types.ERR {
				logrus.Infof("Set replica %v to mode %v", address, mode)
				r.Mode = mode
				c.replicas[i] = r
				c.backend.SetMode(address, mode)
			} else {
				logrus.Infof("Ignore set replica %v to mode %v due to it's ERR",
					address, mode)
			}
		}
	}
	for i, r := range c.quorumReplicas {
		if r.Address == address {
			if r.Mode != types.ERR {
				logrus.Infof("Set replica %v to mode %v", address, mode)
				r.Mode = mode
				c.quorumReplicas[i] = r
				c.backend.SetMode(address, mode)
			} else {
				logrus.Infof("Ignore set replica %v to mode %v due to it's ERR",
					address, mode)
			}
		}
	}
}

func (c *Controller) startFrontend() error {
	if len(c.replicas) > 0 && c.frontend != nil {
		if err := c.frontend.Startup(c.Name, c.frontendIP, c.clusterIP, c.size, c.sectorSize, c); err != nil {
			// FATAL
			logrus.Fatalf("Failed to start up frontend: %v", err)
			// This will never be reached
			return err
		}
	}
	return nil
}

func (c *Controller) Start(addresses ...string) error {
	var (
		expectedRevision int64
		sendSignal       int
		status           string
		err1             error
	)

	c.Lock()
	defer c.Unlock()

	if len(addresses) == 0 {
		return nil
	}

	if len(c.replicas) > 0 {
		return nil
	}

	c.reset()

	defer c.startFrontend()

	first := true
	for _, address := range addresses {
		newBackend, err := c.factory.Create(address)
		if err != nil {
			return err
		}

		newSize, err := newBackend.Size()
		if err != nil {
			return err
		}

		newSectorSize, err := newBackend.SectorSize()
		if err != nil {
			return err
		}

		if first {
			first = false
			c.size = newSize
			c.sectorSize = newSectorSize
		} else if c.size != newSize {
			return fmt.Errorf("Backend sizes do not match %d != %d", c.size, newSize)
		} else if c.sectorSize != newSectorSize {
			return fmt.Errorf("Backend sizes do not match %d != %d", c.sectorSize, newSectorSize)
		}

		if err := c.addReplicaNoLock(newBackend, address, false); err != nil {
			return err
		}
	getCloneStatus:
		if status, err1 = c.backend.GetCloneStatus(address); err1 != nil {
			return err1
		}
		if status == "" || status == "inProgress" {
			logrus.Errorf("Waiting for replica to update CloneStatus to Completed/NA, retry after 2s")
			time.Sleep(2 * time.Second)
			goto getCloneStatus
		} else if status == "error" {
			return fmt.Errorf("Replica clone status returned error")
		}
		// We will validate this later
		c.setReplicaModeNoLock(address, types.RW)
	}

	revisionCounters := make(map[string]int64)
	for _, r := range c.replicas {
		counter, err := c.backend.GetRevisionCounter(r.Address)
		if err != nil {
			return err
		}
		if counter > expectedRevision {
			expectedRevision = counter
		}
		revisionCounters[r.Address] = counter
	}

	for address, counter := range revisionCounters {
		if counter != expectedRevision {
			logrus.Errorf("Revision conflict detected! Expect %v, got %v in replica %v. Mark as ERR",
				expectedRevision, counter, address)
			c.setReplicaModeNoLock(address, types.ERR)
		}
	}
	for regrep := range c.RegisteredReplicas {
		sendSignal = 1
		for _, tmprep := range c.replicas {
			if strings.Contains(tmprep.Address, regrep) {
				sendSignal = 0
				break
			}
		}
		if sendSignal == 1 {
			c.factory.SignalToAdd(regrep, "add")
		}
	}
	for regrep := range c.RegisteredQuorumReplicas {
		sendSignal = 1
		for _, tmprep := range c.quorumReplicas {
			if strings.Contains(tmprep.Address, regrep) {
				sendSignal = 0
				break
			}
		}
		if sendSignal == 1 {
			c.factory.SignalToAdd(regrep, "add")
		}
	}
	if (len(c.replicas)+len(c.quorumReplicas) > (c.replicaCount+c.quorumReplicaCount)/2) && (c.ReadOnly == true) {
		logrus.Infof("Marking volume as R/W")
		c.ReadOnly = false
	}

	return nil
}

// WriteAt is the interface which can be used to write data to jiva volumes
// Delaying error response by 1 second when volume is in read only state, this will avoid
// the iscsi disk at client side to go in read only mode even when IOs
// are not being served.
// Above approach can hold the the app only for small amount of time based
// on the app.
func (c *Controller) WriteAt(b []byte, off int64) (int, error) {
	c.RLock()
	if c.ReadOnly == true {
		err := fmt.Errorf("Mode: ReadOnly")
		c.RUnlock()
		time.Sleep(1 * time.Second)
		return 0, err
	}
	if off < 0 || off+int64(len(b)) > c.size {
		err := fmt.Errorf("EOF: Write of %v bytes at offset %v is beyond volume size %v", len(b), off, c.size)
		c.RUnlock()
		return 0, err
	}
	n, err := c.backend.WriteAt(b, off)
	c.RUnlock()
	if err != nil {
		errh := c.handleError(err)
		if bErr, ok := err.(*BackendError); ok {
			if len(bErr.Errors) > 0 {
				for address := range bErr.Errors {
					c.RemoveReplica(address)
				}
			}
		}
		if n == len(b) && errh == nil {
			return n, nil
		}
		return n, errh
	}
	return n, err
}

func (c *Controller) ReadAt(b []byte, off int64) (int, error) {
	c.RLock()
	if off < 0 || off+int64(len(b)) > c.size {
		err := fmt.Errorf("EOF: Read of %v bytes at offset %v is beyond volume size %v", len(b), off, c.size)
		c.RUnlock()
		return 0, err
	}
	n, err := c.backend.ReadAt(b, off)
	c.RUnlock()
	if err != nil {
		errh := c.handleError(err)
		if bErr, ok := err.(*BackendError); ok {
			if len(bErr.Errors) > 0 {
				for address := range bErr.Errors {
					c.RemoveReplica(address)
				}
			}
		}
		return n, errh
	}
	return n, err
}

func (c *Controller) handleErrorNoLock(err error) error {
	if bErr, ok := err.(*BackendError); ok {
		if len(bErr.Errors) > 0 {
			for address, replicaErr := range bErr.Errors {
				logrus.Errorf("Setting replica %s to ERR due to: %v", address, replicaErr)
				c.setReplicaModeNoLock(address, types.ERR)
			}
			// if we still have a good replica, do not return error
			for _, r := range c.replicas {
				if r.Mode == types.RW {
					logrus.Errorf("Ignoring error because %s is mode RW: %v", r.Address, err)
					err = nil
					break
				}
			}
		}
	}
	if err != nil {
		logrus.Errorf("I/O error: %v", err)
	}
	return err
}

func (c *Controller) handleError(err error) error {
	c.Lock()
	defer c.Unlock()
	return c.handleErrorNoLock(err)
}

func (c *Controller) reset() {
	c.replicas = []types.Replica{}
	c.quorumReplicas = []types.Replica{}
	c.backend = &replicator{}
}

func (c *Controller) Close() error {
	return c.Shutdown()
}

func (c *Controller) shutdownFrontend() error {
	// Make sure writing data won't be blocked
	c.RLock()
	defer c.RUnlock()

	if c.frontend != nil {
		return c.frontend.Shutdown()
	}
	return nil
}

func (c *Controller) Stats() (types.Stats, error) {
	var err error
	// Make sure writing data won't be blocked
	c.RLock()
	defer c.RUnlock()

	if c.frontend != nil {
		stats := (types.Stats)(c.frontend.Stats())
		volUsage, err := c.backend.GetVolUsage()
		stats.UsedLogicalBlocks = volUsage.UsedLogicalBlocks
		stats.UsedBlocks = volUsage.UsedBlocks
		stats.SectorSize = volUsage.SectorSize
		return stats, err
	}
	return types.Stats{}, err
}

func (c *Controller) shutdownBackend() error {
	c.Lock()
	defer c.Unlock()

	err := c.backend.Close()
	c.reset()

	return err
}

func (c *Controller) Shutdown() error {
	/*
		Need to shutdown frontend first because it will write
		the final piece of data to backend
	*/
	err := c.shutdownFrontend()
	if err != nil {
		logrus.Error("Error when shutting down frontend:", err)
	}
	err = c.shutdownBackend()
	if err != nil {
		logrus.Error("Error when shutting down backend:", err)
	}
	return nil
}

func (c *Controller) Size() (int64, error) {
	return c.size, nil
}

func (c *Controller) monitoring(address string, backend types.Backend) {
	monitorChan := backend.GetMonitorChannel()

	if monitorChan == nil {
		return
	}

	logrus.Infof("Start monitoring %v", address)
	err := <-monitorChan
	if err != nil {
		logrus.Errorf("Backend %v monitoring failed, mark as ERR: %v", address, err)
		c.SetReplicaMode(address, types.ERR)
	}
	logrus.Infof("Monitoring stopped %v", address)
	c.RemoveReplica(address)
}
