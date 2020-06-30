package main

import (
	"sync"
	"time"
)

func restartOneReplicaTest() {
	config := buildConfig("172.18.0.10", []string{"172.18.0.11", "172.18.0.12", "172.18.0.13"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.runIOs()
	go config.snapshotCreateDelete()
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
		if time.Since(startTime) > time.Minute*20 {
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

func restartTwoReplicasTest() {
	config := buildConfig("172.18.0.20", []string{"172.18.0.21", "172.18.0.22", "172.18.0.23"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.runIOs()
	go config.snapshotCreateDelete()
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
		if time.Since(startTime) > time.Minute*20 {
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

func restartThreeReplicasTest() {
	config := buildConfig("172.18.0.30", []string{"172.18.0.31", "172.18.0.32", "172.18.0.33"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.runIOs()
	go config.snapshotCreateDelete()
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
		if time.Since(startTime) > time.Minute*20 {
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

func restartControllerTest() {
	config := buildConfig("172.18.0.40", []string{"172.18.0.41", "172.18.0.42", "172.18.0.43"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	config.runIOs()
	go config.snapshotCreateDelete()
	startTime := time.Now()
	for {
		stopContainer(config.Controller[striped(config.ControllerIP)])
		startContainer(config.Controller[striped(config.ControllerIP)])
		config.verifyRWReplicaCount(3)
		if time.Since(startTime) > time.Minute*20 {
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
func chaosTest() {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		restartOneReplicaTest()
		wg.Done()
	}()
	go func() {
		restartTwoReplicasTest()
		wg.Done()
	}()
	go func() {
		restartThreeReplicasTest()
		wg.Done()
	}()
	go func() {
		restartControllerTest()
		wg.Done()
	}()
	wg.Wait()
}
