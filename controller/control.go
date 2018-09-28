package controller

import (
	"fmt"
	"math"
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

func max(x int, y int) int {
	if x > y {
		return x
	}
	return y
}
func min(x int, y int) int {
	if x > y {
		return y
	}
	return x
}

func (c *Controller) GetSize() int64 {
	return c.size
}

func NewController(name string, frontendIP string, clusterIP string, factory types.BackendFactory, frontend types.Frontend, replicationFactor int) *Controller {
	c := &Controller{
		factory:                  factory,
		Name:                     name,
		frontend:                 frontend,
		frontendIP:               frontendIP,
		clusterIP:                clusterIP,
		RegisteredReplicas:       map[string]types.RegReplica{},
		RegisteredQuorumReplicas: map[string]types.RegReplica{},
		StartTime:                time.Now(),
		ReadOnly:                 true,
		replicaCount:             replicationFactor,
	}
	c.reset()
	return c
}

func (c *Controller) UpdateVolStatus() {
	prev := c.ReadOnly
	var rwReplicaCount int
	for _, replica := range c.replicas {
		if replica.Mode == "RW" {
			rwReplicaCount++
		}
	}
	for _, replica := range c.quorumReplicas {
		if replica.Mode == "RW" {
			rwReplicaCount++
		}
	}
	if rwReplicaCount >= (((c.replicaCount + c.quorumReplicaCount) / 2) + 1) {
		c.ReadOnly = false
	} else {
		c.ReadOnly = true
	}
	logrus.Infof("controller readonly p:%v c:%v rcount:%v rw_count:%v",
		prev, c.ReadOnly, len(c.replicas), rwReplicaCount)
	logrus.Infof("backends len: %d", len(c.backend.backends))
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

func (c *Controller) hasWOReplica() (string, bool) {
	logrus.Info("check if any WO replica available")
	for _, i := range c.replicas {
		if i.Mode == types.WO {
			return i.Address, true
		}
	}
	return "", false
}

func (c *Controller) canAdd(address string) (bool, error) {
	if c.hasReplica(address) {
		logrus.Warning("replica %s is already added with this controller instance", address)
		return false, fmt.Errorf("replica: %s is already added", address)
	}
	if woReplica, ok := c.hasWOReplica(); ok {
		logrus.Warning("can have only one WO replica at a time, found WO replica: %s", woReplica)
		return false, fmt.Errorf("can only have one WO replica at a time, found WO Replica: %s",
			woReplica)
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
		logrus.Infof("remote creation addquorum failed %v", err)
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

	if err := c.backend.SetRebuilding(address, false); err != nil {
		return fmt.Errorf("Failed to set rebuild : %v", true)
	}
	c.UpdateVolStatus()

	/*
		for _, temprep := range c.replicas {
			if err := c.backend.SetQuorumReplicaCounter(temprep.Address, int64(len(c.replicas))); err != nil {
				return fmt.Errorf("Fail to set replica counter for %v: %v", address, err)
			}
		}
	*/
	return nil
}

func (c *Controller) verifyReplicationFactor() error {
	replicationFactor := util.CheckReplicationFactor()
	if replicationFactor == 0 {
		return fmt.Errorf("REPLICATION_FACTOR not set")
	}
	if replicationFactor == len(c.replicas) {
		return fmt.Errorf("replication factor: %v, added replicas: %v", replicationFactor, len(c.replicas))
	}
	return nil
}

func (c *Controller) addReplica(address string, snapshot bool) error {
	c.Lock()
	if ok, err := c.canAdd(address); !ok {
		c.Unlock()
		return err
	}
	logrus.Info("verify replication factor")
	if err := c.verifyReplicationFactor(); err != nil {
		c.Unlock()
		return fmt.Errorf("can't add %s, error: %v", address, err)
	}
	c.Unlock()
	newBackend, err := c.factory.Create(address)
	if err != nil {
		logrus.Infof("remote creation addreplica failed %v", err)
		return err
	}

	c.Lock()
	defer c.Unlock()

	err = c.addReplicaNoLock(newBackend, address, snapshot)
	if err != nil {
		logrus.Infof("addReplicaNoLock %s from addReplica failed %v", address, err)
	} else {
		c.UpdateVolStatus()
	}
	return err
}

func (c *Controller) signalToAdd() {
	c.factory.SignalToAdd(c.MaxRevReplica, "start")
}

func (c *Controller) registerReplica(register types.RegReplica) error {
	c.Lock()
	defer c.Unlock()
	logrus.Infof("Register Replica, Address: %v Uptime: %v State: %v Type: %v RevisionCount: %v",
		register.Address, register.UpTime, register.RepState, register.RepType, register.RevCount)

	_, ok := c.RegisteredReplicas[register.Address]
	if !ok {
		_, ok = c.RegisteredQuorumReplicas[register.Address]
		if ok {
			logrus.Infof("Quorum replica Address %v already present in registered list", register.Address)
			return nil
		}
	}

	if register.RepType == "quorum" {
		c.RegisteredQuorumReplicas[register.Address] = register
		return nil
	}
	c.RegisteredReplicas[register.Address] = register

	if len(c.replicas) > 0 {
		logrus.Infof("There are already some replicas attached")
		return nil
	}
	if c.StartSignalled == true {
		if c.MaxRevReplica == register.Address {
			logrus.Infof("Replica %v signalled to start again %d:%d", c.MaxRevReplica,
				len(c.RegisteredReplicas), c.replicaCount)
			if err := c.signalReplica(); err != nil {
				c.StartSignalled = false
				return err
			}
		} else {
			logrus.Infof("Can signal only to %s ,can't signal to %s, no of registered replicas are %d and replica count is %d",
				c.MaxRevReplica, register.Address, len(c.RegisteredReplicas), c.replicaCount)
			return nil
		}
	}
	if register.RepState == "rebuilding" {
		logrus.Errorf("Cannot add replica in rebuilding state, addr: %v", register.Address)
		return nil
	}

	if c.MaxRevReplica == "" {
		c.MaxRevReplica = register.Address
	}

	if c.RegisteredReplicas[c.MaxRevReplica].RevCount < register.RevCount {
		c.MaxRevReplica = register.Address
	}

	if (len(c.RegisteredReplicas) >= ((c.replicaCount / 2) + 1)) &&
		((len(c.RegisteredReplicas) + len(c.RegisteredQuorumReplicas)) >= (((c.quorumReplicaCount + c.replicaCount) / 2) + 1)) {
		logrus.Infof("Replica %v signalled to start, no of registered replicas are %d and replica count is %d", c.MaxRevReplica,
			len(c.RegisteredReplicas), c.replicaCount)
		if err := c.signalReplica(); err != nil {
			return err
		}
		c.StartSignalled = true
		return nil
	}

	logrus.Warning("No of yet to be registered replicas are less than ", c.replicaCount,
		" , No of registered replicas: ", len(c.RegisteredReplicas))
	return nil
}

// signalReplica is a wrapper over SignalToAdd which is used as utility
// function by registerReplica. It sends a POST request to replica to
// start and delete the replica from map in case of error.
// No need to take lock as a lock has been already taken by the callee.
func (c *Controller) signalReplica() error {
	if err := c.factory.SignalToAdd(c.MaxRevReplica, "start"); err != nil {
		logrus.Errorf("Replica %v is not able to send 'start' signal, found err: %s",
			c.MaxRevReplica, err.Error())
		delete(c.RegisteredReplicas, c.MaxRevReplica)
		c.MaxRevReplica = ""
		return err
	}
	return nil
}

// IsSnapShotExist verifies whether snapshot with the given name
// already exists in the given replica.
func IsSnapShotExist(snapName string, addr string) (bool, error) {
	chain, err := getReplicaChain(addr)
	if err != nil {
		return false, fmt.Errorf("Failed to get replica chain, error: %v", err)
	}
	if len(chain) == 0 {
		return false, fmt.Errorf("No chain list found in replica")
	}
	snapshot := fmt.Sprintf("volume-snap-%s.img", snapName)
	for _, val := range chain {
		if val == snapshot {
			return true, nil
		}
	}
	return false, nil
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

	replica, err := c.getRWReplica()
	if err != nil {
		return name, err
	}

	ok, err := IsSnapShotExist(name, replica.Address)
	if err != nil {
		return name, fmt.Errorf("Failed to create snapshot, error: %v", err)
	}

	if ok {
		return name, fmt.Errorf("Snapshot: %s already exists", name)
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
	/*
	 * No need to add prints in this function.
	 * Make sure caller of this takes care of printing error
	 */
	if ok, err := c.canAdd(address); !ok {
		return err
	}

	if snapshot {
		uuid := util.UUID()
		created := util.Now()
		var remain int
		var err error

		if remain, err = c.backend.RemainSnapshots(); err != nil {
			return err
		} else if remain <= 0 {
			return fmt.Errorf("Too many %v snapshots created", remain)
		}

		if err = c.backend.Snapshot(uuid, false, created); err != nil {
			newBackend.Close()
			return err
		}
		// This replica is not added to backend yet
		if err = newBackend.Snapshot(uuid, false, created); err != nil {
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
	logrus.Infof("check if replica %s is already added", address)
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

func (c *Controller) RemoveReplicaNoLock(address string) error {
	var foundregrep int

	logrus.Infof("RemoveReplica %v ReplicasAdded:%v FrontendState:%v", address, len(c.replicas), c.frontend.State())
	if !c.hasReplica(address) {
		logrus.Infof("RemoveReplica %v not found", address)
		return nil
	}
	for i, r := range c.replicas {
		if r.Address == address {
			if len(c.replicas) == 1 && c.frontend.State() == types.StateUp {
				if c.frontend != nil {
					c.StartSignalled = false
					c.MaxRevReplica = ""
					c.frontend.Shutdown()
				}
			}
			foundregrep = 0
			for regrep := range c.RegisteredReplicas {
				logrus.Infof("RemoveReplica ToRemove: %v Found: %v", address, regrep)
				if address == "tcp://"+regrep+":9502" {
					delete(c.RegisteredReplicas, regrep)
					foundregrep = 1
					break
				}
			}
			if foundregrep == 0 {
				//We should not break if the replica is not found in registered
				//list, since all replicas are not registered.
				//if there is already one replica in RW mode then, the replica
				//registration process is avoided and same is true for quorum
				//replicas
				logrus.Infof("RemoveReplica %v not found in registered replicas", address)
			}
			c.replicas = append(c.replicas[:i], c.replicas[i+1:]...)
			c.backend.RemoveBackend(r.Address)
			break
		}
	}

	for i, r := range c.quorumReplicas {
		foundregrep = 0
		if r.Address == address {
			for regrep := range c.RegisteredQuorumReplicas {
				logrus.Infof("RemoveReplica quorum ToRemove: %v Found: %v", address, regrep)
				if address == "tcp://"+regrep+":9502" {
					delete(c.RegisteredQuorumReplicas, regrep)
					foundregrep = 1
					break
				}
			}
			if foundregrep == 0 {
				logrus.Infof("RemoveReplica %v not found in registered quorum replicas", address)
			}
			c.quorumReplicas = append(c.quorumReplicas[:i], c.quorumReplicas[i+1:]...)
			c.backend.RemoveBackend(r.Address)
			break
		}
	}
	c.UpdateVolStatus()
	return nil
}

func (c *Controller) RemoveReplica(address string) error {
	c.Lock()
	defer c.Unlock()

	return c.RemoveReplicaNoLock(address)
}

func (c *Controller) ListReplicas() []types.Replica {
	return c.replicas
}

func (c *Controller) ListQuorumReplicas() []types.Replica {
	c.Lock()
	defer c.Unlock()
	return c.quorumReplicas
}

func (c *Controller) SetReplicaMode(address string, mode types.Mode) error {
	switch mode {
	case types.ERR:
		c.Lock()
		defer c.Unlock()
	case types.RW:
		c.Lock()
		defer c.Unlock()
	default:
		return fmt.Errorf("Can not set to mode %s", mode)
	}
	c.setReplicaModeNoLock(address, mode)
	return nil
}

func (c *Controller) setReplicaModeNoLock(address string, mode types.Mode) {
	var found int
	found = 0
	for i, r := range c.replicas {
		if r.Address == address {
			found = found + 1
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
			found = found + 1
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
	if found > 1 {
		logrus.Fatalf("setReplicaModeNoLock error %d %d %s %v", len(c.replicas),
			found, address, mode)
	}
	if found == 0 {
		logrus.Infof("setReplicaModeNoLock not found %d %d %s %v", len(c.replicas),
			found, address, mode)
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
	} else {
		logrus.Infof("replicas %d is either 0 or frontend %v is nil", len(c.replicas), c.frontend)
	}
	return nil
}

func (c *Controller) addReplicaDuringStartNoLock(address string) error {
	var (
		status string
		err1   error
	)
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

	if c.size == math.MaxInt64 {
		c.size = newSize
		c.sectorSize = newSectorSize
	}

	if c.size != newSize {
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
		return fmt.Errorf("Replica clone status returned error %s", address)
	}
	c.setReplicaModeNoLock(address, types.RW)
	return nil
}

func (c *Controller) Start(addresses ...string) error {
	var (
		expectedRevision int64
		sendSignal       int
	)

	c.Lock()
	defer c.Unlock()

	if len(addresses) == 0 {
		logrus.Infof("addresses is null")
		return nil
	}

	if len(c.replicas) > 0 {
		logrus.Infof("already %d replicas are started and added", len(c.replicas))
		return nil
	}

	c.reset()

	defer c.startFrontend()

	c.size = math.MaxInt64
	for _, address := range addresses {
		err := c.addReplicaDuringStartNoLock(address)
		if err != nil {
			logrus.Errorf("err %v adding %s replica during start", err, address)
			return err
		}
	}

	revisionCounters := make(map[string]int64)
	for _, r := range c.replicas {
		counter, err := c.backend.GetRevisionCounter(r.Address)
		if err != nil {
			logrus.Errorf("GetRevisionCounter failed %s %v", r.Address, err)
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
			if tmprep.Address == "tcp://"+regrep+":9502" {
				sendSignal = 0
				break
			}
		}
		if sendSignal == 1 {
			logrus.Infof("sending add signal to %v", regrep)
			c.factory.SignalToAdd(regrep, "add")
		}
	}
	for regrep := range c.RegisteredQuorumReplicas {
		sendSignal = 1
		for _, tmprep := range c.quorumReplicas {
			if tmprep.Address == "tcp://"+regrep+":9502" {
				sendSignal = 0
				break
			}
		}
		if sendSignal == 1 {
			logrus.Infof("sending add signal to quorum %v", regrep)
			c.factory.SignalToAdd(regrep, "add")
		}
	}
	logrus.Info("Update volume status")
	c.UpdateVolStatus()

	return nil
}

// WriteAt is the interface which can be used to write data to jiva volumes
// Delaying error response by 1 second when volume is in read only state, this will avoid
// the iscsi disk at client side to go in read only mode even when IOs
// are not being served.
// Above approach can hold the the app only for small amount of time based
// on the app.
func (c *Controller) WriteAt(b []byte, off int64) (int, error) {
	c.Lock()
	if c.ReadOnly == true {
		err := fmt.Errorf("Mode: ReadOnly")
		c.Unlock()
		time.Sleep(1 * time.Second)
		return 0, err
	}
	defer c.Unlock()
	if off < 0 || off+int64(len(b)) > c.size {
		err := fmt.Errorf("EOF: Write of %v bytes at offset %v is beyond volume size %v", len(b), off, c.size)
		return 0, err
	}
	n, err := c.backend.WriteAt(b, off)
	if err != nil {
		errh := c.handleErrorNoLock(err)
		if bErr, ok := err.(*BackendError); ok {
			if len(bErr.Errors) > 0 {
				for address := range bErr.Errors {
					c.RemoveReplicaNoLock(address)
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
	c.Lock()
	defer c.Unlock()
	if off < 0 || off+int64(len(b)) > c.size {
		err := fmt.Errorf("EOF: Read of %v bytes at offset %v is beyond volume size %v", len(b), off, c.size)
		return 0, err
	}
	if len(c.replicas) == 0 {
		return 0, fmt.Errorf("No backends available")
	}
	if len(c.replicas) == 1 {
		r := c.replicas[0]
		if r.Mode == "WO" {
			return 0, fmt.Errorf("only WO replica available")
		}
	}

	n, err := c.backend.ReadAt(b, off)
	if err != nil {
		errh := c.handleErrorNoLock(err)
		if bErr, ok := err.(*BackendError); ok {
			if len(bErr.Errors) > 0 {
				for address := range bErr.Errors {
					c.RemoveReplicaNoLock(address)
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
	logrus.Infof("resetting controller")
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
		logrus.Errorf("cannot monitor %s.. chan is null", address)
		return
	}

	logrus.Infof("Start monitoring %v", address)
	err := <-monitorChan
	c.Lock()
	defer c.Unlock()
	if err != nil {
		logrus.Errorf("Backend %v monitoring failed, mark as ERR: %v", address, err)
		c.setReplicaModeNoLock(address, types.ERR)
	}
	logrus.Infof("Monitoring stopped %v", address)
	c.RemoveReplicaNoLock(address)
}
