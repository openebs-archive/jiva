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

func (config *TestConfig) verifyRWReplicaCount(count int) {
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

func createReplicas(config *TestConfig) {
	replicas := config.Replicas
	for replicaIP, _ := range replicas {
		os.Mkdir("/tmp"+replicaIP+"vol", 0755)
		config.Replicas[replicaIP] = createReplica(replicaIP, config)
	}
}

func deleteReplicas(config *TestConfig) {
	replicas := config.Replicas
	for replicaIP, _ := range replicas {
		stopContainer(config.Replicas[replicaIP])
		removeContainer(config.Replicas[replicaIP])
		os.RemoveAll("/tmp" + replicaIP + "vol")
	}
}

func (config *TestConfig) RestartReplicas(replicaIPs ...string) {
	for _, replicaIP := range replicaIPs {
		stopContainer(config.Replicas[replicaIP])
		removeContainer(config.Replicas[replicaIP])
		config.Replicas[replicaIP] = createReplica(replicaIP, config)
	}
}
