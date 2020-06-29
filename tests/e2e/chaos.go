package main

import (
	"sync"
	"time"
)

func RestartOneReplicaTest() {
	config := buildConfig("172.18.0.10", []string{"172.18.0.11", "172.18.0.12", "172.18.0.13"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.RunIOs()
	go config.SnapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	for {
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			panic(err)
		}
		stopContainer(config.Replicas[striped(replicas[0].Address)])
		startContainer(config.Replicas[striped(replicas[0].Address)])
		config.verifyRWReplicaCount(3)
		if time.Since(startTime) > time.Minute*30 {
			break
		}
	}
	for {
		if config.ThreadCount == 0 {
			break
		}
	}
	scrap(config)
}

func RestartTwoReplicasTest() {
	config := buildConfig("172.18.0.20", []string{"172.18.0.21", "172.18.0.22", "172.18.0.23"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.RunIOs()
	go config.SnapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	for {
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			panic(err)
		}
		stopContainer(config.Replicas[striped(replicas[0].Address)])
		stopContainer(config.Replicas[striped(replicas[1].Address)])
		startContainer(config.Replicas[striped(replicas[0].Address)])
		startContainer(config.Replicas[striped(replicas[1].Address)])
		config.verifyRWReplicaCount(3)
		if time.Since(startTime) > time.Minute*30 {
			break
		}
	}
	for {
		if config.ThreadCount == 0 {
			break
		}
	}
	scrap(config)
}

func RestartThreeReplicasTest() {
	config := buildConfig("172.18.0.30", []string{"172.18.0.31", "172.18.0.32", "172.18.0.33"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.RunIOs()
	go config.SnapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	for {
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			panic(err)
		}
		stopContainer(config.Replicas[striped(replicas[0].Address)])
		stopContainer(config.Replicas[striped(replicas[1].Address)])
		stopContainer(config.Replicas[striped(replicas[2].Address)])
		startContainer(config.Replicas[striped(replicas[0].Address)])
		startContainer(config.Replicas[striped(replicas[1].Address)])
		startContainer(config.Replicas[striped(replicas[2].Address)])
		config.verifyRWReplicaCount(3)
		if time.Since(startTime) > time.Minute*30 {
			break
		}
	}
	for {
		if config.ThreadCount == 0 {
			break
		}
	}
	scrap(config)
}

func RestartControllerTest() {
	config := buildConfig("172.18.0.40", []string{"172.18.0.41", "172.18.0.42", "172.18.0.43"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.RunIOs()
	go config.SnapshotCreateDelete()
	startTime := time.Now()
	for {
		stopContainer(config.Controller[striped(config.ControllerIP)])
		startContainer(config.Controller[striped(config.ControllerIP)])
		config.verifyRWReplicaCount(3)
		if time.Since(startTime) > time.Minute*30 {
			break
		}
	}
	config.Stop = true
	for {
		if config.ThreadCount == 0 {
			break
		}
	}
	scrap(config)
}
func chaos_test() {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		RestartOneReplicaTest()
		wg.Done()
	}()
	go func() {
		RestartTwoReplicasTest()
		wg.Done()
	}()
	go func() {
		RestartThreeReplicasTest()
		wg.Done()
	}()
	go func() {
		RestartControllerTest()
		wg.Done()
	}()
	wg.Wait()
}
