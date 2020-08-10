package main

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var TestRunTime time.Duration = 60

func restartOneReplicaTest() {
	config := buildConfig("172.18.0.10", []string{"172.18.0.11", "172.18.0.12", "172.18.0.13"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	go config.testSequentialData()
	go config.snapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	iteration := 1
	for {
		logrus.Infof("Iteration number %v for restartOneReplicaTest", iteration)
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			panic(err)
		}
		stopContainer(config.Replicas[striped(replicas[0].Address)])
		startContainer(config.Replicas[striped(replicas[0].Address)])
		config.verifyRWReplicaCount(3)
		time.Sleep(65 * time.Second) // 65 so that auto-snapshot delete is triggered
		if time.Since(startTime) > time.Minute*TestRunTime {
			break
		}
		iteration++
	}
	config.Stop = true
	for range time.NewTicker(5 * time.Second).C {
		if config.ThreadCount == 0 {
			break
		}
		logrus.Infof(
			"RestartControllerTest Completed, waiting for threads to exit, pending tc:%v",
			config.ThreadCount,
		)
	}
	logrus.Infof("Exiting RestartOneReplicaTest, threads exited successfully")
	scrap(config)
}

func restartTwoReplicasTest() {
	config := buildConfig("172.18.0.20", []string{"172.18.0.21", "172.18.0.22", "172.18.0.23"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	go config.testSequentialData()
	go config.snapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	iteration := 1
	for {
		logrus.Infof("Iteration number %v for restartTwoReplicasTest", iteration)
		replicas, err := ctrlClient.ListReplicas()
		if err != nil {
			panic(err)
		}
		stopContainer(config.Replicas[striped(replicas[0].Address)])
		stopContainer(config.Replicas[striped(replicas[1].Address)])
		startContainer(config.Replicas[striped(replicas[0].Address)])
		startContainer(config.Replicas[striped(replicas[1].Address)])
		config.verifyRWReplicaCount(3)
		time.Sleep(65 * time.Second) // 65 so that auto-snapshot delete is triggered
		if time.Since(startTime) > time.Minute*TestRunTime {
			break
		}
		iteration++
	}
	config.Stop = true
	for range time.NewTicker(5 * time.Second).C {
		if config.ThreadCount == 0 {
			break
		}
		logrus.Infof(
			"RestartControllerTest Completed, waiting for threads to exit, pending tc:%v",
			config.ThreadCount,
		)
	}
	logrus.Infof("Exiting RestartTwoReplicaTest, threads exited successfully")
	scrap(config)
}

func restartThreeReplicasTest() {
	config := buildConfig("172.18.0.30", []string{"172.18.0.31", "172.18.0.32", "172.18.0.33"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	go config.testSequentialData()
	go config.snapshotCreateDelete()
	ctrlClient := getControllerClient(config.ControllerIP)
	startTime := time.Now()
	iteration := 1
	for {
		logrus.Infof("Iteration number %v for restartThreeReplicasTest", iteration)
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
		time.Sleep(65 * time.Second) // 65 so that auto-snapshot delete is triggered
		if time.Since(startTime) > time.Minute*TestRunTime {
			break
		}
		iteration++
	}
	config.Stop = true
	for range time.NewTicker(5 * time.Second).C {
		if config.ThreadCount == 0 {
			break
		}
		logrus.Infof(
			"RestartControllerTest Completed, waiting for threads to exit, pending tc:%v",
			config.ThreadCount,
		)
	}
	logrus.Infof("Exiting RestartThreeReplicaTest, threads exited successfully")
	scrap(config)
}

func restartControllerTest() {
	config := buildConfig("172.18.0.40", []string{"172.18.0.41", "172.18.0.42", "172.18.0.43"})
	setupTest(config)
	config.verifyRWReplicaCount(3)
	go config.testSequentialData()
	go config.snapshotCreateDelete()
	startTime := time.Now()
	iteration := 1
	for {
		logrus.Infof("Iteration number %v for restartControllerTest", iteration)
		stopContainer(config.Controller[striped(config.ControllerIP)])
		startContainer(config.Controller[striped(config.ControllerIP)])
		config.verifyRWReplicaCount(3)
		time.Sleep(65 * time.Second) // 65 so that auto-snapshot delete is triggered
		if time.Since(startTime) > time.Minute*TestRunTime {
			break
		}
		iteration++
	}
	config.Stop = true
	for range time.NewTicker(5 * time.Second).C {
		if config.ThreadCount == 0 {
			break
		}
		logrus.Infof(
			"RestartControllerTest Completed, waiting for threads to exit, pending tc:%v",
			config.ThreadCount,
		)
	}
	logrus.Infof("Exiting RestartControllerTest, threads exited successfully")
	scrap(config)
}
func chaosTest() {
	logrus.Infof("Start chaos Test")
	var wg sync.WaitGroup
	wg.Add(1)

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
	logrus.Infof("Chaos Test completed successfully")
}
