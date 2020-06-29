package main

import (
	"strings"
	"sync"
)

type testConfig struct {
	sync.Mutex
	ThreadCount       int
	Stop              bool
	Image             string
	VolumeName        string
	ReplicationFactor string
	ControllerIP      string
	Controller        map[string]string
	Replicas          map[string]string
	controllerEnvs    map[string]string
	ReplicaEnvs       map[string]string
}

func striped(address string) string {
	address = strings.TrimPrefix(address, "tcp://")
	address = strings.TrimSuffix(address, ":9502")
	return address
}

func buildConfig(controllerIP string, replicas []string) *testConfig {
	config := &testConfig{
		ControllerIP: controllerIP,
		Controller:   map[string]string{controllerIP: ""},
	}
	config.Replicas = make(map[string]string)
	for _, rep := range replicas {
		config.Replicas[rep] = ""
	}
	config.Image = getJivaImageID()
	config.ReplicationFactor = "3"
	config.VolumeName = "vol" + config.ControllerIP
	return config
}

func (config *testConfig) insertThread() {
	config.Lock()
	config.ThreadCount++
	config.Unlock()
}

func (config *testConfig) releaseThread() {
	config.Lock()
	config.ThreadCount--
	config.Unlock()
}

func setupTest(config *testConfig) {
	createController(config.ControllerIP, config)
	createReplicas(config)
}

func scrap(config *testConfig) {
	deleteController(config)
	deleteReplicas(config)
}
