package rest

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	replica_jiva "github.com/openebs/jiva/replica"
	replicaClient "github.com/openebs/jiva/replica/client"
	replica_rest "github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

const (
	volumeNotFound  = "Volume not found"
	zeroReplica     = "No replicas registered with this controller instance"
	repClientErr    = "Error in creating replica client"
	deletionSuccess = "Replica deleted successfully"
	deletionErr     = "Error deleting replica"
)

type DeletedReplica struct {
	Replica string `json:"replica"`
	Error   string `json:"error,omitempty"`
	Msg     string `json:"msg"`
}

type DeletedReplicas struct {
	DeletedReplicasInfo []DeletedReplica `json:"replicas"`
}

func (r *DeletedReplicas) appendDeletedReplicas(err, addr, msg string) {
	deletedReplica := DeletedReplica{
		Replica: addr,
		Error:   err,
		Msg:     msg,
	}
	r.DeletedReplicasInfo = append(r.DeletedReplicasInfo, deletedReplica)
}

// SetDeleteReplicaOutput returns the output containing the list
// of deleted replicas and other details related to it.
func SetDeleteReplicaOutput(deletedReplicas DeletedReplicas) *DeleteReplicaOutput {
	return &DeleteReplicaOutput{
		client.Resource{
			Type: "delete",
		},
		deletedReplicas,
	}
}

func (s *Server) delete(replicas *DeletedReplicas, wg *sync.WaitGroup) {
	s.c.Lock()
	defer s.c.Unlock()
	for _, replica := range s.c.ListReplicas() {
		addr := replica.Address
		go func(addr string) {
			defer wg.Done()
			repClient, err := replicaClient.NewReplicaClient(addr)
			if err != nil {
				logrus.Infof("Error in delete operation of replica %v , error %v", addr, err)
				replicas.appendDeletedReplicas(err.Error(), addr, repClientErr)
				return
			}
			logrus.Info("Sending delete request to replica : ", addr)
			if err := repClient.Delete("/delete"); err != nil {
				logrus.Infof("Error in delete operation of replica %v , error %v", addr, err)
				replicas.appendDeletedReplicas(err.Error(), addr, deletionErr)
				return
			}
			replicas.appendDeletedReplicas("", addr, deletionSuccess)
		}(addr)
	}
}

// DeleteVolume handles the delete request call from the controller's
// client. It checks for the replication factor before deleting the
// replicas. If the replica count is equal to the replication factor
// then it will proceed to delete. If not, it returns a response
// explaining the cause of error in response.
func (s *Server) DeleteVolume(rw http.ResponseWriter, req *http.Request) error {
	var (
		replicas DeletedReplicas
		wg       sync.WaitGroup
	)
	apiContext := api.GetApiContext(req)
	replicaCount := len(s.c.ListReplicas())
	if replicaCount == 0 {
		replicas.appendDeletedReplicas(volumeNotFound, "", zeroReplica)
		apiContext.Write(SetDeleteReplicaOutput(replicas))
		return nil
	}
	replicationFactor := util.CheckReplicationFactor()
	if replicaCount != replicationFactor {
		replicationFactorErr := fmt.Sprintf("Replication factor: %d is not equal to replica count: %d",
			replicationFactor, replicaCount)
		replicas.appendDeletedReplicas(replicationFactorErr, "", deletionErr)
		apiContext.Write(SetDeleteReplicaOutput(replicas))
		return nil
	}
	wg.Add(replicaCount)
	s.delete(&replicas, &wg)
	wg.Wait()
	apiContext.Write(SetDeleteReplicaOutput(replicas))
	return nil
}

func rmDisk(replica *types.Replica, disk string) error {
	repClient, err := replicaClient.NewReplicaClient(replica.Address)
	if err != nil {
		return err
	}
	return repClient.RemoveDisk(disk)
}

func replaceDisk(replica *types.Replica, target, source string) error {
	repClient, err := replicaClient.NewReplicaClient(replica.Address)
	if err != nil {
		return err
	}
	return repClient.ReplaceDisk(target, source)
}

func (s *Server) processRemoveSnapshot(replica *types.Replica, snapshot string, ops []replica_jiva.PrepareRemoveAction) error {
	if len(ops) == 0 {
		return nil
	}
	if replica.Mode != "RW" {
		return fmt.Errorf("Can only removed snapshot from replica in mode RW, got %s", replica.Mode)
	}
	repClient, err := replicaClient.NewReplicaClient(replica.Address)
	if err != nil {
		return err
	}
	for _, op := range ops {
		switch op.Action {
		case replica_jiva.OpRemove:
			logrus.Infof("Removing %s on %s", op.Source, replica.Address)
			if err := rmDisk(replica, op.Source); err != nil {
				return err
			}
		case replica_jiva.OpCoalesce:
			logrus.Infof("Coalescing %v to %v on %v", op.Target, op.Source, replica.Address)
			if err = repClient.Coalesce(op.Target, op.Source); err != nil {
				logrus.Errorf("Failed to coalesce %s on %s: %v", snapshot, replica.Address, err)
				return err
			}
		case replica_jiva.OpReplace:
			logrus.Infof("Replace %v with %v on %v", op.Target, op.Source, replica.Address)
			if err = replaceDisk(replica, op.Target, op.Source); err != nil {
				logrus.Errorf("Failed to replace %v with %v on %v", op.Target, op.Source, replica.Address)
				return err
			}
		}
	}
	return nil
}

func (s *Server) prepareRemoveSnapshot(replica *types.Replica, snapshot string) ([]replica_jiva.PrepareRemoveAction, error) {
	if replica.Mode != "RW" {
		return nil, fmt.Errorf("Can only removed snapshot from replica in mode RW, got %s", replica.Mode)
	}
	client, err := replicaClient.NewReplicaClient(replica.Address)
	if err != nil {
		return nil, err
	}
	output, err := client.PrepareRemoveDisk(snapshot)
	if err != nil {
		return nil, err
	}
	return output.Operations, nil
}
func (s *Server) checkPrerequisits(replicas []types.Replica) error {
	for _, replica := range replicas {
		replicaInfo, err := getReplicaInfo(replica.Address)
		if err != nil {
			return err
		}
		if replicaInfo.Rebuilding {
			return fmt.Errorf("Replica %s is rebuilding", replica.Address)
		}
	}
	return nil
}

func getReplicaInfo(addr string) (replica_rest.Replica, error) {
	repClient, err := replicaClient.NewReplicaClient(addr)
	if err != nil {
		return replica_rest.Replica{}, err
	}
	replicaInfo, err := repClient.GetReplica()
	if err != nil {
		return replica_rest.Replica{}, err
	}
	return replicaInfo, nil
}

func isRemovable(replicas []types.Replica, name string) error {
	var replica types.Replica
	for _, replica = range replicas {
		if replica.Mode == "RW" {
			break
		}
	}
	info, err := getReplicaInfo(replica.Address)
	if err != nil {
		return err
	}
	snapName := fmt.Sprintf("volume-snap-%s.img", name)
	if info.Chain[1] == snapName {
		return fmt.Errorf("can't delete latest snapshot %s", snapName)
	}
	return nil
}

func (s *Server) deleteSnapshot(replicas []types.Replica, name string) error {
	s.c.Lock()
	defer s.c.Unlock()
	logrus.Infof("check if snapshot %s is removable", name)
	if err := isRemovable(replicas, name); err != nil {
		return err
	}
	logrus.Info("check prerequisits")
	err := s.checkPrerequisits(replicas)
	if err != nil {
		return err
	}
	ops := make(map[string][]replica_jiva.PrepareRemoveAction)
	for _, replica := range replicas {
		ops[replica.Address], err = s.prepareRemoveSnapshot(&replica, name)
		if err != nil {
			return err
		}
	}
	for _, replica := range replicas {
		if err := s.processRemoveSnapshot(&replica, name, ops[replica.Address]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) DeleteSnapshot(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	var input SnapshotInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}
	replicas := s.c.ListReplicas()
	replicaCount := len(replicas)
	if replicaCount == 0 {
		return fmt.Errorf("Number of registered replicas with this controller instance is zero")
	}
	replicationFactor := util.CheckReplicationFactor()
	if replicaCount != replicationFactor {
		return fmt.Errorf("Replica count: %v is not equal to replication factor: %v", replicaCount, replicationFactor)
	}
	logrus.Infof("Delete snapshot: %s", input.Name)
	err := s.deleteSnapshot(replicas, input.Name)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Snapshot: %s deleted successfully", input.Name)
	apiContext.Write(&SnapshotOutput{
		client.Resource{
			Id:   input.Name,
			Type: "snapshotOutput",
		},
		msg,
	})
	return nil
}
