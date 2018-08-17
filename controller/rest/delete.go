package rest

import (
	"errors"
	"net/http"

	"github.com/Sirupsen/logrus"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

const (
	zeroReplica     = "No of replicas are zero"
	repClientErr    = "Error in creating replica client"
	deletionSuccess = "Replica deleted successfully"
	deletionErr     = "Error in making 'Delete' request to replica"
)

type DeletedReplica struct {
	Replica string `json:"replica"`
	Error   error  `json:"error"`
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
	)
	apiContext := api.GetApiContext(req)

	if len(s.c.ListReplicas()) == 0 {
		deletedReplicas.appendReplicas(volumeNotFound, "", zeroReplica)
		apiContext.Write(SetDeleteReplicaOutput(deletedReplicas))
		return nil
	}
	for _, replica := range s.c.ListReplicas() {
		repClient, err := replicaClient.NewReplicaClient(replica.Address)
		if err != nil {
			logrus.Infof("Error in delete operation of replica %v , found error %v", replica.Address, err)
			deletedReplicas.appendReplicas(err, replica.Address, repClientErr)
			continue
		}
		logrus.Info("Sending delete request to replica : ", replica.Address)
		if err := repClient.Delete("/delete"); err != nil {
			logrus.Infof("Error in delete operation of replica %v , found error %v", replica.Address, err)
			deletedReplicas.appendReplicas(err, replica.Address, deletionErr)
			continue
		}
		deletedReplicas.appendReplicas(nil, replica.Address, deletionSuccess)
	}
	apiContext.Write(SetDeleteReplicaOutput(deletedReplicas))
	return nil
}
