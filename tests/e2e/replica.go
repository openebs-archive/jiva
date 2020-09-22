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
	"os"
	"time"

	controllerClient "github.com/openebs/jiva/controller/client"
	replicaClient "github.com/openebs/jiva/replica/client"
)

func getControllerClient(address string) *controllerClient.ControllerClient {
	url := "http://" + address + ":9501"
	return controllerClient.NewControllerClient(url)
}

func getReplicaClient(address string) (*replicaClient.ReplicaClient, error) {
	return replicaClient.NewReplicaClient(address)
}

func (config *testConfig) verifyRWReplicaCount(count int) {
	var rwCount int
	ctrlClient := getControllerClient(config.ControllerIP)
	for {
		rwCount = 0
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		for _, rep := range replicas {
			if rep.Mode == "RW" {
				rwCount++
			}
		}
		if rwCount == count {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func createReplicas(config *testConfig) {
	replicas := config.Replicas
	for replicaIP := range replicas {
		os.Mkdir("/tmp1/"+replicaIP+"vol", 0755)
		config.Replicas[replicaIP] = createReplica(replicaIP, config)
	}
}

func deleteReplicas(config *testConfig) {
	replicas := config.Replicas
	for replicaIP := range replicas {
		stopContainer(config.Replicas[replicaIP])
		removeContainer(config.Replicas[replicaIP])
		os.RemoveAll("/tmp1/" + replicaIP + "vol")
	}
}

func (config *testConfig) restartReplicas(replicaIPs ...string) {
	for _, replicaIP := range replicaIPs {
		stopContainer(config.Replicas[replicaIP])
		removeContainer(config.Replicas[replicaIP])
		config.Replicas[replicaIP] = createReplica(replicaIP, config)
	}
}
