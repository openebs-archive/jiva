/*
 Copyright © 2020 The OpenEBS Authors

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

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/openebs/jiva/backend/dynamic"
	"github.com/openebs/jiva/backend/remote"
	"github.com/openebs/jiva/controller"
	"github.com/openebs/jiva/controller/rest"
	"github.com/openebs/jiva/rpc"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
)

var (
	frontends = map[string]types.Frontend{}
)

func initializeBackend() map[string]types.BackendFactory {
	factories := map[string]types.BackendFactory{}
	factories["tcp"] = remote.New()
	return factories
}

func initializeFrontend(controllerIP string) (types.Frontend, types.Target, error) {
	var target types.Target
	target = types.Target{
		FrontendIP: controllerIP,
		ClusterIP:  controllerIP,
	}

	var frontend types.Frontend
	f, ok := frontends["gotgt"]
	if !ok {
		return nil, types.Target{}, fmt.Errorf("Failed to find frontend: %s", "gotgt")
	}
	frontend = f

	if frontend == nil {
		return nil, target, fmt.Errorf("frontend is nil")
	}
	return frontend, target, nil
}

func (config *testConfig) startTestController(controllerIP string) error {
	name := controllerIP + "vol"
	rf := config.ReplicationFactor
	types.RPCReadTimeout = util.GetReadTimeout()
	types.RPCWriteTimeout = util.GetWriteTimeout()
	rpc.SetRPCTimeout()
	logrus.Infof("REPLICATION_FACTOR: %v, RPC_READ_TIMEOUT: %v, RPC_WRITE_TIMEOUT: %v", rf, types.RPCReadTimeout, types.RPCWriteTimeout)

	if !util.ValidVolumeName(name) {
		return errors.New("invalid target name")
	}
	controlListener := controllerIP + ":9501"
	frontend, tgt, err := initializeFrontend(controllerIP)
	if err != nil {
		return err
	}
	logrus.Infof("Starting controller with frontendIP: %v, and clusterIP: %v", tgt.FrontendIP, tgt.ClusterIP)

	control := controller.
		NewController(
			controller.WithName(name),
			controller.WithClusterIP(tgt.ClusterIP),
			controller.WithBackend(dynamic.New(initializeBackend())),
			controller.WithFrontend(frontend, tgt.FrontendIP),
			controller.WithRF(int(rf)))
	server := rest.NewServer(control)
	config.Controller[controllerIP] = server
	router := http.Handler(rest.NewRouter(server))

	router = util.FilteredLoggingHandler(map[string]struct{}{
		"/v1/volumes":  {},
		"/v1/replicas": {},
		"/v1/stats":    {},
	}, os.Stdout, router)
	router = handlers.ProxyHeaders(router)

	logrus.Infof("Listening on %s", controlListener)
	return http.ListenAndServe(controlListener, router)
}
func (config *testConfig) verifyRWReplicaCount(count int) {
	controller := config.Controller[config.ControllerIP].GetController()
	for controller.RWReplicaCount != count {
		logrus.Infof("Sleep while verifyRWReplicaCount, Actual: %v Desired: %v", controller.RWReplicaCount, count)
		time.Sleep(2 * time.Second)
	}
}
func (config *testConfig) verifyCheckpoint(set bool) {
	controller := config.Controller[config.ControllerIP].GetController()
	for (controller.Checkpoint != "") != set {
		logrus.Infof("Sleep while VerifyCheckpointSet, Actual: %v, Desired: %v", controller.Checkpoint != "", set)
		time.Sleep(2 * time.Second)
		controller = config.Controller[config.ControllerIP].GetController()
	}
}
func (config *testConfig) verifyCheckpointSameAtReplicas(replicas []string) bool {
reverify:
	controller := config.Controller[config.ControllerIP].GetController()
	checkpoint := controller.Checkpoint
	time.Sleep(5 * time.Second)
	for _, rep := range replicas {
		if config.Replicas[rep].Server.Replica() == nil {
			config.verifyRWReplicaCount(3)
			goto reverify
		}
		if config.Replicas[rep].Server.Replica().Info().Checkpoint != checkpoint {
			return false
		}
	}
	return true
}

func (config *testConfig) isReplicaAttachedToController(replicaIP string) bool {
	replicas := config.Controller[config.ControllerIP].GetController().ListReplicas()
	for _, rep := range replicas {
		if rep.Address == "tcp://"+replicaIP+":9502" {
			return true
		}
	}
	return false
}

func (config *testConfig) updateCheckpoint() {
	controller := config.Controller[config.ControllerIP].GetController()
	controller.UpdateCheckpoint()
}

func (config *testConfig) createSnapshot(snapshot string) error {
	controller := config.Controller[config.ControllerIP].GetController()
	_, err := controller.Snapshot(snapshot)
	return err
}

func (config *testConfig) DeleteSnapshot(snapshot string) error {
	controller := config.Controller[config.ControllerIP].GetController()
	return controller.DeleteSnapshot(snapshot, controller.ListReplicas())
}

func (config *testConfig) GetCheckpoint() string {
	controller := config.Controller[config.ControllerIP].GetController()
	return controller.Checkpoint
}
