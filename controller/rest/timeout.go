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
	"os"

	"github.com/rancher/go-rancher/api"
	"github.com/sirupsen/logrus"
)

func (s *Server) AddTimeout(rw http.ResponseWriter, req *http.Request) error {
	var timeout Timeout
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&timeout); err != nil {
		logrus.Errorf("failed to read the request body, error: %v", err)
		return err
	}
	if timeout.Timeout != "" {
		logrus.Infof("Added a timeout of %vs", timeout.Timeout)
		return os.Setenv("DEBUG_TIMEOUT", timeout.Timeout)
	}
	if timeout.RPCPingTimeout != "" {
		logrus.Infof("Added a ping timeout of %vs", timeout.RPCPingTimeout)
		return os.Setenv("RPC_PING_TIMEOUT", timeout.RPCPingTimeout)
	}
	return fmt.Errorf("Error in setting timeout, received empty value")
}
