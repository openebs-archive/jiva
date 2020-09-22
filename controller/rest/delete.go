/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package rest

import (
	"fmt"
	"net/http"
	"sync"

	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/util"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/sirupsen/logrus"
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
