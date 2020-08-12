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
