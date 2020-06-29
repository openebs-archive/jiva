package main

import (
	"strings"
	"sync"
)

type TestConfig struct {
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

func buildConfig(controllerIP string, replicas []string) *TestConfig {
	config := &TestConfig{
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

func (config *TestConfig) InsertThread() {
	config.Lock()
	config.ThreadCount++
	config.Unlock()
}

func (config *TestConfig) ReleaseThread() {
	config.Lock()
	config.ThreadCount--
	config.Unlock()
}

func setupTest(config *TestConfig) {
	createController(config.ControllerIP, config)
	createReplicas(config)
}

func scrap(config *TestConfig) {
	deleteController(config)
	deleteReplicas(config)
}
