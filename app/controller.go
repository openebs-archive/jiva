/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package app

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/openebs/jiva/backend/dynamic"
	"github.com/openebs/jiva/backend/file"
	"github.com/openebs/jiva/backend/remote"
	"github.com/openebs/jiva/controller"
	"github.com/openebs/jiva/controller/rest"
	"github.com/openebs/jiva/rpc"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	frontends = map[string]types.Frontend{}
)

func ControllerCmd() cli.Command {
	return cli.Command{
		Name: "controller",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "listen",
				Value: ":9501",
			},
			cli.StringFlag{
				Name:  "frontend",
				Value: "",
			},
			cli.StringFlag{
				Name:  "frontendIP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "clusterIP",
				Value: "",
			},
			cli.StringSliceFlag{
				Name:  "enable-backend",
				Value: (*cli.StringSlice)(&[]string{"tcp"}),
			},
			cli.StringSliceFlag{
				Name: "replica",
			},
		},
		Action: func(c *cli.Context) {
			if err := startController(c); err != nil {
				logrus.Fatalf("Error running controller command: %v.", err)
			}
		},
	}
}

func initializeBackend(c *cli.Context) map[string]types.BackendFactory {
	backends := c.StringSlice("enable-backend")
	factories := map[string]types.BackendFactory{}
	for _, backend := range backends {
		switch backend {
		case "file":
			factories[backend] = file.New()
		case "tcp":
			factories[backend] = remote.New()
		default:
			logrus.Fatalf("Unsupported backend: %s", backend)
		}
	}
	return factories
}

func initializeFrontend(c *cli.Context) (types.Frontend, types.Target, error) {
	var target types.Target
	frontendName := c.String("frontend")
	if frontendName == "gotgt" {
		target = types.Target{
			FrontendIP: c.String("frontendIP"),
			ClusterIP:  c.String("clusterIP"),
		}
	}

	var frontend types.Frontend
	if frontendName != "" {
		f, ok := frontends[frontendName]
		if !ok {
			return nil, types.Target{}, fmt.Errorf("Failed to find frontend: %s", frontendName)
		}
		frontend = f
	}

	if frontend == nil {
		return nil, target, fmt.Errorf("frontend is nil")
	}
	return frontend, target, nil
}

func startController(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("volume name is required")
	}
	name := c.Args()[0]
	rf := util.CheckReplicationFactor()
	types.RPCReadTimeout = util.GetReadTimeout()
	types.RPCWriteTimeout = util.GetWriteTimeout()
	rpc.SetRPCTimeout()
	logrus.Infof("REPLICATION_FACTOR: %v, RPC_READ_TIMEOUT: %v, RPC_WRITE_TIMEOUT: %v", rf, types.RPCReadTimeout, types.RPCWriteTimeout)

	if !util.ValidVolumeName(name) {
		return errors.New("invalid target name")
	}
	controlListener := c.String("listen")
	replicas := c.StringSlice("replica")
	frontend, tgt, err := initializeFrontend(c)
	if err != nil {
		return err
	}
	logrus.Infof("Starting controller with frontendIP: %v, and clusterIP: %v", tgt.FrontendIP, tgt.ClusterIP)

	control := controller.
		NewController(
			controller.WithName(name),
			controller.WithClusterIP(tgt.ClusterIP),
			controller.WithBackend(dynamic.New(
				initializeBackend(c))),
			controller.WithFrontend(frontend, tgt.FrontendIP),
			controller.WithRF(int(rf)))
	server := rest.NewServer(control)
	router := http.Handler(rest.NewRouter(server))

	router = util.FilteredLoggingHandler(map[string]struct{}{
		"/v1/volumes":  {},
		"/v1/replicas": {},
		"/v1/stats":    {},
	}, os.Stdout, router)
	router = handlers.ProxyHeaders(router)

	if len(replicas) > 0 {
		logrus.Infof("Starting with replicas %q", replicas)
		if err := control.Start(replicas...); err != nil {
			log.Fatal(err)
		}
	}

	logrus.Infof("Listening on %s", controlListener)

	addShutdown(func() {
		control.Shutdown()
	})
	return http.ListenAndServe(controlListener, router)
}

func checkPrerequisites(c *controller.Controller, replicas []types.Replica) error {
	if c.IsSnapDeletionInProgress {
		return fmt.Errorf(
			"Snapshot deletion is in progress, %s is getting deleted",
			c.SnapshotName,
		)
	}

	rf := c.ReplicationFactor
	if len(replicas) != rf {
		return fmt.Errorf(
			"Can not remove a snapshot because, RF: %v, replica count: %v",
			rf, len(replicas),
		)
	}

	for _, rep := range replicas {
		if rep.Mode != "RW" {
			return fmt.Errorf(
				"Can't delete snapshot, Replica %s mode is %s",
				rep.Address, rep.Mode,
			)
		}
	}
	return nil
}

func listSnapshots(c *controller.Controller, replicas []rest.Replica) ([]string, error) {
	snapshots, err := getCommonSnapshots(replicas)
	if err != nil {
		return nil, err
	}
	snaps := []string{}
	for _, s := range snapshots {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "volume-snap-"), ".img")
		snaps = append(snaps, s)
	}

	return snaps, nil
}
