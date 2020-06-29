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

func (config *TestConfig) StartTestController(controllerIP string) error {
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
func (c *TestConfig) verifyRWReplicaCount(count int) {
	controller := c.Controller[c.ControllerIP].GetController()
	for controller.RWReplicaCount != c.ReplicationFactor {
		logrus.Infof("Sleep while verifyRWReplicaCount, Actual: %v Desired: %v", count, controller.RWReplicaCount)
		time.Sleep(2 * time.Second)
	}
}
func (c *TestConfig) verifyCheckpoint(set bool) {
	controller := c.Controller[c.ControllerIP].GetController()
	for (controller.Checkpoint != "") != set {
		logrus.Infof("Sleep while VerifyCheckpointSet, Actual: %v, Desired: %v", controller.Checkpoint != "", set)
		time.Sleep(2 * time.Second)
		controller = c.Controller[c.ControllerIP].GetController()
	}
}
func (c *TestConfig) VerifyCheckpointSameAtReplicas(replicas []string) bool {
	controller := c.Controller[c.ControllerIP].GetController()
	checkpoint := controller.Checkpoint
	for _, rep := range replicas {
		if c.Replicas[rep].Server.Replica().Info().Checkpoint != checkpoint {
			return false
		}
	}
	return true
}

func (config *TestConfig) IsReplicaAttachedToController(replicaIP string) bool {
	replicas := config.Controller[config.ControllerIP].GetController().ListReplicas()
	for _, rep := range replicas {
		if rep.Address == "tcp://"+replicaIP+":9502" {
			return true
		}
	}
	return false
}

func (c *TestConfig) UpdateCheckpoint() {
	controller := c.Controller[c.ControllerIP].GetController()
	controller.UpdateCheckpoint()
}

func (c *TestConfig) CreateSnapshot(snapshot string) error {
	controller := c.Controller[c.ControllerIP].GetController()
	_, err := controller.Snapshot(snapshot)
	return err
}
