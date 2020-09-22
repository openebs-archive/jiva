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
	"net/http"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/sirupsen/logrus"
)

// DeleteVolume deletes all the contents of the volume. It purges all the replica
// files.
func (s *Server) DeleteVolume(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("DeleteVolume")
	apiContext := api.GetApiContext(req)
	err := s.s.DeleteAll()
	if err != nil {
		logrus.Errorf("Error in deleting the volume, error: %v", err)
		return err
	}
	apiContext.Write(&DeleteReplicaOutput{
		client.Resource{
			Type: "delete",
		},
	})
	return nil
}
