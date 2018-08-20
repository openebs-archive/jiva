package rest

import (
	"errors"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

const (
	zeroReplica     = "No replicas registered with this controller instance"
	repClientErr    = "Error in creating replica client"
	deletionSuccess = "Replica deleted successfully"
	deletionErr     = "Error deleting replica"
)

type DeletedReplica struct {
	Replica string `json:"replica"`
	Error   error  `json:"error,omitempty"`
	Msg     string `json:"msg"`
}

type DeletedReplicas struct {
	DeletedReplicasInfo []DeletedReplica `json:"replicas"`
}

func (r *DeletedReplicas) appendReplicas(err error, addr, msg string) {
	deletedReplica := DeletedReplica{
		Replica: addr,
		Error:   err,
		Msg:     msg,
	}
	r.DeletedReplicasInfo = append(r.DeletedReplicasInfo, deletedReplica)
}

func SetDeleteReplicaOutput(deletedReplicas DeletedReplicas) *DeleteReplicaOutput {
	return &DeleteReplicaOutput{
		client.Resource{
			Type: "delete",
		},
		deletedReplicas,
	}
}

func (s *Server) DeleteVolume(rw http.ResponseWriter, req *http.Request) error {
	var (
		volumeNotFound  = errors.New("Volume not found")
		deletedReplicas DeletedReplicas
		wg              sync.WaitGroup
	)
	apiContext := api.GetApiContext(req)
	replicaCount := len(s.c.ListReplicas())
	if replicaCount == 0 {
		deletedReplicas.appendReplicas(volumeNotFound, "", zeroReplica)
		apiContext.Write(SetDeleteReplicaOutput(deletedReplicas))
		return nil
	}
	wg.Add(replicaCount)
	for _, replica := range s.c.ListReplicas() {
		addr := replica.Address
		go func(addr string) {
			defer wg.Done()
			repClient, err := replicaClient.NewReplicaClient(addr)
			if err != nil {
				logrus.Infof("Error in delete operation of replica %v , found error %v", addr, err)
				deletedReplicas.appendReplicas(err, addr, repClientErr)
				return
			}
			logrus.Info("Sending delete request to replica : ", addr)
			if err := repClient.Delete("/delete"); err != nil {
				logrus.Infof("Error in delete operation of replica %v , found error %v", addr, err)
				deletedReplicas.appendReplicas(err, addr, deletionErr)
				return
			}
			deletedReplicas.appendReplicas(nil, addr, deletionSuccess)
		}(addr)
	}
	wg.Wait()
	apiContext.Write(SetDeleteReplicaOutput(deletedReplicas))
	return nil
}
