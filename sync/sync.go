package sync

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/controller/client"
	"github.com/openebs/jiva/controller/rest"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/replica"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/types"
)

var (
	RetryCounts = 3
)

type Task struct {
	client *client.ControllerClient
}

func NewTask(controller string) *Task {
	return &Task{
		client: client.NewControllerClient(controller),
	}
}

func (t *Task) DeleteSnapshot(snapshot string) error {
	var err error

	replicas, err := t.client.ListReplicas()
	if err != nil {
		return err
	}

	for _, r := range replicas {
		if ok, err := t.isRebuilding(&r); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("Can not remove a snapshot because %s is rebuilding", r.Address)
		}
	}

	ops := make(map[string][]replica.PrepareRemoveAction)
	for _, replica := range replicas {
		ops[replica.Address], err = t.prepareRemoveSnapshot(&replica, snapshot)
		if err != nil {
			return err
		}
	}

	for _, replica := range replicas {
		if err := t.processRemoveSnapshot(&replica, snapshot, ops[replica.Address]); err != nil {
			return err
		}
	}

	return nil
}

func (t *Task) rmDisk(replicaInController *rest.Replica, disk string) error {
	repClient, err := replicaClient.NewReplicaClient(replicaInController.Address)
	if err != nil {
		return err
	}

	return repClient.RemoveDisk(disk)
}

func (t *Task) replaceDisk(replicaInController *rest.Replica, target, source string) error {
	repClient, err := replicaClient.NewReplicaClient(replicaInController.Address)
	if err != nil {
		return err
	}

	return repClient.ReplaceDisk(target, source)
}

func getNameAndIndex(chain []string, snapshot string) (string, int) {
	index := find(chain, snapshot)
	if index < 0 {
		snapshot = fmt.Sprintf("volume-snap-%s.img", snapshot)
		index = find(chain, snapshot)
	}

	if index < 0 {
		return "", index
	}

	return snapshot, index
}

func (t *Task) isRebuilding(replicaInController *rest.Replica) (bool, error) {
	repClient, err := replicaClient.NewReplicaClient(replicaInController.Address)
	if err != nil {
		return false, err
	}

	replica, err := repClient.GetReplica()
	if err != nil {
		return false, err
	}

	return replica.Rebuilding, nil
}

func (t *Task) prepareRemoveSnapshot(replicaInController *rest.Replica, snapshot string) ([]replica.PrepareRemoveAction, error) {
	if replicaInController.Mode != "RW" {
		return nil, fmt.Errorf("Can only removed snapshot from replica in mode RW, got %s", replicaInController.Mode)
	}

	repClient, err := replicaClient.NewReplicaClient(replicaInController.Address)
	if err != nil {
		return nil, err
	}

	output, err := repClient.PrepareRemoveDisk(snapshot)
	if err != nil {
		return nil, err
	}

	return output.Operations, nil
}

func (t *Task) processRemoveSnapshot(replicaInController *rest.Replica, snapshot string, ops []replica.PrepareRemoveAction) error {
	if len(ops) == 0 {
		return nil
	}

	if replicaInController.Mode != "RW" {
		return fmt.Errorf("Can only removed snapshot from replica in mode RW, got %s", replicaInController.Mode)
	}

	repClient, err := replicaClient.NewReplicaClient(replicaInController.Address)
	if err != nil {
		return err
	}

	for _, op := range ops {
		switch op.Action {
		case replica.OpRemove:
			logrus.Infof("Removing %s on %s", op.Source, replicaInController.Address)
			if err := t.rmDisk(replicaInController, op.Source); err != nil {
				return err
			}
		case replica.OpCoalesce:
			logrus.Infof("Coalescing %v to %v on %v", op.Target, op.Source, replicaInController.Address)
			if err = repClient.Coalesce(op.Target, op.Source); err != nil {
				logrus.Errorf("Failed to coalesce %s on %s: %v", snapshot, replicaInController.Address, err)
				return err
			}
		case replica.OpReplace:
			logrus.Infof("Replace %v with %v on %v", op.Target, op.Source, replicaInController.Address)
			if err = t.replaceDisk(replicaInController, op.Target, op.Source); err != nil {
				logrus.Errorf("Failed to replace %v with %v on %v", op.Target, op.Source, replicaInController.Address)
				return err
			}
		}
	}

	return nil
}

func find(list []string, item string) int {
	for i, val := range list {
		if val == item {
			return i
		}
	}
	return -1
}

func (t *Task) AddQuorumReplica(replicaAddress string, _ *replica.Server) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
Register:
	volume, err := t.client.GetVolume()
	if err != nil {
		return err
	}
	addr := strings.Split(replicaAddress, "://")
	parts := strings.Split(addr[1], ":")
	Replica, _ := replica.CreateTempReplica()
	server, _ := replica.CreateTempServer()

	if volume.ReplicaCount == 0 {
		revisionCount := Replica.GetRevisionCounter()
		replicaType := "quorum"
		upTime := time.Since(Replica.ReplicaStartTime)
		state, _ := server.PrevStatus()
		_ = t.client.Register(parts[0], revisionCount, replicaType, upTime, string(state))
		select {
		case <-ticker.C:
			goto Register
		case _ = <-replica.ActionChannel:
		}
	}

	logrus.Infof("Adding quorum replica %s in WO mode", replicaAddress)
	_, err = t.client.CreateQuorumReplica(replicaAddress)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) CloneReplica(url string, address string, cloneIP string, snapName string) error {
	var (
		fromReplica rest.Replica
		snapFound   bool
	)
	for {
		ControllerClient := client.NewControllerClient(url)
		reps, err := ControllerClient.ListReplicas()
		if err != nil {
			logrus.Errorf("Failed to get replica list from %s, retry after 2s, error: %s", url, err.Error())
			time.Sleep(2 * time.Second)
			continue
		}

		for _, rep := range reps {
			if rep.Mode == "RW" {
				fromReplica = rep
				break
			}
		}
		if fromReplica.Mode == "" {
			logrus.Errorf("No RW replica found at %s, retry after 2s", url)
			time.Sleep(2 * time.Second)
			continue
		}
		fromClient, err := replicaClient.NewReplicaClient(fromReplica.Address)
		if err != nil {
			return fmt.Errorf("Failed to create source replica client, error: %s", err.Error())
		}
		repl, err := fromClient.GetReplica()
		if err != nil {
			return fmt.Errorf("Failed to get replica info of the source, error: %s", err.Error())
		}
		chain := repl.Chain

		logrus.Infof("Using replica %s as the source for rebuild ", fromReplica.Address)

		toClient, err := replicaClient.NewReplicaClient(address)
		if err != nil {
			return fmt.Errorf("Failed to create client of the clone replica, error: %s", err.Error())
		}
		for i, name := range chain {
			if name == "volume-snap-"+snapName+".img" {
				snapFound = true
				chain = chain[i:]
				break
			}
		}
		if snapFound == false {
			return fmt.Errorf("Snapshot Not found at source")
		}
		if err := toClient.SetRebuilding(true); err != nil {
			return fmt.Errorf("Failed to setRebuilding = true, %s", err)
		}
		if err = t.syncFiles(fromClient, toClient, chain); err != nil {
			logrus.Errorf("Sync failed, retry after 2s, error: %s", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		toClient.UpdateCloneInfo(snapName)

		_, err = toClient.ReloadReplica()
		if err != nil {
			return fmt.Errorf("Failed to reload clone replica, error: %s", err.Error())
		}
		if err := toClient.SetRebuilding(false); err != nil {
			return fmt.Errorf("Failed to setRebuilding = false, error: %s", err.Error())
		}
		return err
	}
}

func (t *Task) AddReplica(replicaAddress string, s *replica.Server) error {
	var action string

	if s == nil {
		return fmt.Errorf("Server not present for %v, Add replica using CLI not supported", replicaAddress)
	}
	types.ShouldPunchHoles = false
	logrus.Infof("Addreplica %v", replicaAddress)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	Replica, err := replica.CreateTempReplica()
	if err != nil {
		return fmt.Errorf("failed to create temp replica, error: %s", err.Error())
	}
	server, err := replica.CreateTempServer()
	if err != nil {
		return fmt.Errorf("failed to create temp server, error: %s", err.Error())
	}
Register:
	logrus.Infof("Get Volume info from controller")
	volume, err := t.client.GetVolume()
	if err != nil {
		return fmt.Errorf("failed to get volume info, error: %s", err.Error())
	}
	addr := strings.Split(replicaAddress, "://")
	parts := strings.Split(addr[1], ":")
	if volume.ReplicaCount == 0 {
		revisionCount := Replica.GetRevisionCounter()
		replicaType := "Backend"
		upTime := time.Since(Replica.ReplicaStartTime)
		state, _ := server.PrevStatus()
		logrus.Infof("Register replica at controller")
		err := t.client.Register(parts[0], revisionCount, replicaType, upTime, string(state))
		if err != nil {
			logrus.Errorf("Error in sending register command, error: %s", err)
		}
		select {
		case <-ticker.C:
			logrus.Info("Timed out waiting for response from controller, will retry")
			goto Register
		case action = <-replica.ActionChannel:
		}
	}
	if action == "start" {
		logrus.Infof("Received start from controller")
		types.ShouldPunchHoles = true
		if err := t.client.Start(replicaAddress); err != nil {
			types.ShouldPunchHoles = false
			return err
		}
		return nil
	}
	logrus.Infof("CheckAndResetFailedRebuild %v", replicaAddress)
	if err := t.checkAndResetFailedRebuild(replicaAddress, s); err != nil {
		return fmt.Errorf("CheckAndResetFailedRebuild failed, error: %s", err.Error())
	}

	logrus.Infof("Adding replica %s in WO mode", replicaAddress)
	_, err = t.client.CreateReplica(replicaAddress)
	if err != nil {
		logrus.Errorf("Failed to create replica, error: %v", err)
		// cases for above failure:
		// - controller is not reachable
		// - replica is already added (remove replica in progress)
		// - error while creating snapshot, to start fresh
		// - replica might be in errored state
		// - other replica might be in WO mode, and
		// we can only have one WO replica at a time
		// Adding sleep so that, it doesn't get restart very frequently
		time.Sleep(5 * time.Second)
		return err
	}

	logrus.Infof("getTransferClients %v", replicaAddress)
	fromClient, toClient, err := t.getTransferClients(replicaAddress)
	if err != nil {
		return fmt.Errorf("failed to get transfer clients, error: %s", err.Error())
	}

	logrus.Infof("SetRebuilding to true in %v", replicaAddress)
	if err := toClient.SetRebuilding(true); err != nil {
		return fmt.Errorf("failed to set rebuilding: true, error: %s", err.Error())
	}

	logrus.Infof("PrepareRebuild %v", replicaAddress)
	output, err := t.client.PrepareRebuild(rest.EncodeID(replicaAddress))
	if err != nil {
		return fmt.Errorf("failed to prepare rebuild, error: %s", err.Error())
	}
	inject.PanicAfterPrepareRebuild()

	ok, err := t.isRevisionCountAndChainSame(fromClient, toClient)
	if err != nil {
		return err
	}

	if !ok {
		logrus.Infof("syncFiles from:%v to:%v", fromClient, toClient)
		if err = t.syncFiles(fromClient, toClient, output.Disks); err != nil {
			return err
		}
	}

	logrus.Infof("reloadAndVerify %v", replicaAddress)
	return t.reloadAndVerify(replicaAddress, toClient)
}

func (t *Task) isRevisionCountAndChainSame(fromClient, toClient *replicaClient.ReplicaClient) (bool, error) {
	rwReplica, err := fromClient.GetReplica()
	if err != nil {
		return false, err
	}

	curReplica, err := toClient.GetReplica()
	if err != nil {
		return false, err
	}

	if rwReplica.RevisionCounter == curReplica.RevisionCounter {
		// ignoring Chain[0] since it's head file and it is opened for writing the latest data.
		if !reflect.DeepEqual(rwReplica.Chain[1:], curReplica.Chain[1:]) {
			logrus.Warningf("Replica %v's chain not equal to RW replica %v's chain", toClient.GetAddress(), fromClient.GetAddress())
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

// checkAndResetFailedRebuild set the rebuilding to false if
// it is true.This is required since volume.meta files
// may not be updated with it's correct rebuilding state.
func (t *Task) checkAndResetFailedRebuild(address string, server *replica.Server) error {

	state, info := server.Status()

	if state == "closed" && info.Rebuilding {
		if err := server.Open(); err != nil {
			logrus.Errorf("Error during open in checkAndResetFailedRebuild")
			return err
		}
		if err := server.SetRebuilding(false); err != nil {
			logrus.Errorf("Error during setRebuilding in checkAndResetFailedRebuild")
			return err
		}
		return server.Close()
	}
	return nil
}

func (t *Task) reloadAndVerify(address string, repClient *replicaClient.ReplicaClient) error {
	_, err := repClient.ReloadReplica()
	if err != nil {
		logrus.Errorf("Error in reloadreplica %s", address)
		return err
	}

	if err := t.client.VerifyRebuildReplica(rest.EncodeID(address)); err != nil {
		logrus.Errorf("Error in verifyRebuildReplica %s", address)
		return err
	}

	if err = repClient.SetRebuilding(false); err != nil {
		logrus.Errorf("Error in setRebuilding %s", address)
	}
	return err
}

func (t *Task) syncFiles(fromClient *replicaClient.ReplicaClient, toClient *replicaClient.ReplicaClient, disks []string) error {
	for i := range disks {
		//We are syncing from the oldest snapshots to newer ones
		disk := disks[len(disks)-1-i]
		if strings.Contains(disk, "volume-head") {
			return fmt.Errorf("Disk list shouldn't contain volume-head")
		}
		if err := t.syncFile(disk, "", fromClient, toClient); err != nil {
			return err
		}

		if err := t.syncFile(disk+".meta", "", fromClient, toClient); err != nil {
			return err
		}

	}

	return nil
}

func (t *Task) syncFile(from, to string, fromClient *replicaClient.ReplicaClient, toClient *replicaClient.ReplicaClient) error {
	if to == "" {
		to = from
	}

	host, port, err := toClient.LaunchReceiver(to)
	if err != nil {
		return err
	}

	logrus.Infof("Synchronizing %s to %s@%s:%d", from, to, host, port)
	err = fromClient.SendFile(from, host, port)
	if err != nil {
		logrus.Errorf("Failed synchronizing %s to %s@%s:%d: %v", from, to, host, port, err)
	} else {
		logrus.Infof("Done synchronizing %s to %s@%s:%d", from, to, host, port)
	}

	return err
}

func (t *Task) getTransferClients(address string) (*replicaClient.ReplicaClient, *replicaClient.ReplicaClient, error) {
	from, err := t.getFromReplica()
	if err != nil {
		return nil, nil, err
	}
	logrus.Infof("Using replica %s as the source for rebuild ", from.Address)

	fromClient, err := replicaClient.NewReplicaClient(from.Address)
	if err != nil {
		return nil, nil, err
	}

	to, err := t.getToReplica(address)
	if err != nil {
		return nil, nil, err
	}
	logrus.Infof("Using replica %s as the target for rebuild ", to.Address)

	toClient, err := replicaClient.NewReplicaClient(to.Address)
	if err != nil {
		return nil, nil, err
	}

	return fromClient, toClient, nil
}

func (t *Task) getFromReplica() (rest.Replica, error) {
	replicas, err := t.client.ListReplicas()
	if err != nil {
		return rest.Replica{}, err
	}

	for _, r := range replicas {
		if r.Mode == "RW" {
			return r, nil
		}
	}

	return rest.Replica{}, fmt.Errorf("Failed to find good replica to copy from")
}

func (t *Task) getToReplica(address string) (rest.Replica, error) {
	replicas, err := t.client.ListReplicas()
	if err != nil {
		return rest.Replica{}, err
	}

	for _, r := range replicas {
		if r.Address == address {
			if r.Mode != "WO" {
				return rest.Replica{}, fmt.Errorf("Replica %s is not in mode WO, got: %s", address, r.Mode)
			}
			return r, nil
		}
	}

	return rest.Replica{}, fmt.Errorf("Failed to find target replica to copy to")
}
