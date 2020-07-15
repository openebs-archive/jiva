package main

import (
	"net/http"
	"strings"
	"sync"

	controllerRest "github.com/openebs/jiva/controller/rest"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/rpc"
)

type testConfig struct {
	sync.Mutex
	ThreadCount        int
	Stop               bool
	Image              string
	VolumeName         string
	Size               string
	ReplicationFactor  int
	ControllerIP       string
	Controller         map[string]*controllerRest.Server
	Replicas           map[string]*replicaInfo
	ControllerEnvs     map[string]string
	ReplicaEnvs        map[string]string
	ReplicaRestartList []string
	Close              map[string]chan struct{}
}

type replicaInfo struct {
	Server     *replica.Server
	RestServer *http.Server
	RPCServer  *rpc.Server
}

func striped(address string) string {
	address = strings.TrimPrefix(address, "tcp://")
	address = strings.TrimSuffix(address, ":9502")
	return address
}

func buildConfig(controllerIP string, replicas []string) *testConfig {
	config := &testConfig{
		ControllerIP: controllerIP,
	}
	config.ReplicationFactor = 3
	config.VolumeName = "vol" + config.ControllerIP
	config.Size = "5G"
	config.ControllerEnvs = make(map[string]string, 3)
	config.ReplicaEnvs = make(map[string]string, 3)
	config.Controller = make(map[string]*controllerRest.Server)
	config.Replicas = make(map[string]*replicaInfo, 3)
	config.Controller[controllerIP] = nil
	config.Replicas[replicas[0]] = &replicaInfo{}
	config.Replicas[replicas[1]] = &replicaInfo{}
	config.Replicas[replicas[2]] = &replicaInfo{}
	config.Close = make(map[string]chan struct{}, 3)
	config.Close[replicas[0]] = make(chan struct{})
	config.Close[replicas[1]] = make(chan struct{})
	config.Close[replicas[2]] = make(chan struct{})
	inject.Envs = make(map[string](map[string]bool), 3)
	inject.Envs[replicas[0]+":9502"] = make(map[string]bool, 5)
	inject.Envs[replicas[1]+":9502"] = make(map[string]bool, 5)
	inject.Envs[replicas[2]+":9502"] = make(map[string]bool, 5)
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
