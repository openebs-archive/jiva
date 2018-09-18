package rest

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/util"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

const (
	volumeNotFound = "volume not found"
	zeroReplica    = "No replicas registered with this controller instance"
	repClientErr   = "error in creating replica client"
	deletionErr    = "Error deleting replica"
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
func SetDeleteReplicaOutput(deletedReplicas DeletedReplicas) *DeleteVolumeOutput {
	return &DeleteVolumeOutput{
		client.Resource{
			Type: "deleteVolumeOutput",
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
			status, err := repClient.Delete("/delete")
			if err != nil {
				logrus.Infof("Error in delete operation of replica %v , error %v", addr, err)
				replicas.appendDeletedReplicas(err.Error(), addr, deletionErr)
				return
			}
			replicas.appendDeletedReplicas("", addr, status)
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
	logrus.Info("Delete volume")
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]
	if v := s.getVolume(apiContext, id); v == nil {
		return fmt.Errorf("%v", volumeNotFound)
	}
	replicaCount := len(s.c.ListReplicas())
	if replicaCount == 0 {
		logrus.Error(zeroReplica)
		replicas.appendDeletedReplicas(volumeNotFound, "", zeroReplica)
		rw.WriteHeader(http.StatusNotFound)
		apiContext.Write(SetDeleteReplicaOutput(replicas))
		return nil
	}
	replicationFactor := util.CheckReplicationFactor()
	if replicaCount != replicationFactor {
		replicationFactorErr := fmt.Sprintf("replication factor: %d is not equal to replica count: %d",
			replicationFactor, replicaCount)
		logrus.Error(replicationFactorErr)
		replicas.appendDeletedReplicas(replicationFactorErr, "", deletionErr)
		rw.WriteHeader(http.StatusConflict)
		apiContext.Write(SetDeleteReplicaOutput(replicas))
		return nil
	}
	wg.Add(replicaCount)
	s.delete(&replicas, &wg)
	wg.Wait()
	apiContext.Write(SetDeleteReplicaOutput(replicas))
	return nil
}
