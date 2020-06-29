package main

import (
	"reflect"
	"runtime/debug"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/openebs/jiva/frontend/gotgt"
	"github.com/openebs/sparse-tools/cli/sfold"
	"github.com/openebs/sparse-tools/cli/ssync"

	"github.com/sirupsen/logrus"
)

func initializeBackendProcesses() {
	reexec.Register("ssync", ssync.Main)
	reexec.Register("sfold", sfold.Main)

}

func main() {
	frontends["gotgt"] = gotgt.New()
	initializeBackendProcesses()

	replicas := []string{"172.17.0.111", "172.17.0.112", "172.17.0.113"}
	c := buildConfig("172.17.0.110", replicas)
	// Start controller
	go func() {
		Verify("StartController", c.StartTestController(c.ControllerIP), nil)
	}()
	time.Sleep(5 * time.Second)
	// Start 3 Replicas in debug mode
	for replica, _ := range c.Replicas {
		rep := replica
		go func(replica string) {
			c.StartTestReplica(replica, replica+"vol", true)
		}(rep)
	}

	go c.MonitorReplicas()

	Verify("CheckpointTest", c.CheckpointTest(replicas), nil)
}

func Verify(msg string, x, y interface{}) {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		logrus.Errorf("Type Mismatch")
	}
	if x != y {
		debug.PrintStack()
		logrus.Fatalf("%v failed", msg)
	}
}
