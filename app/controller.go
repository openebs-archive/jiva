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
	logrus.Infof("REPLICATION_FACTOR: %v", rf)

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

	startAutoSnapDeletion := make(chan bool)
	control := controller.
		NewController(
			startAutoSnapDeletion,
			controller.WithName(name),
			controller.WithClusterIP(tgt.ClusterIP),
			controller.WithBackend(dynamic.New(
				initializeBackend(c))),
			controller.WithFrontend(frontend, tgt.FrontendIP),
			controller.WithRF(int(rf)))
	server := rest.NewServer(control)
	router := http.Handler(rest.NewRouter(server))
	go func(c *controller.Controller) {
		for <-startAutoSnapDeletion {
			go autoDeleteSnapshot(c)
		}
	}(control)

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

func autoDeleteSnapshot(c *controller.Controller) error {
	c.Lock()
	replicas := c.ListReplicas()
	if err := checkPrerequisites(c, replicas); err != nil {
		c.Unlock()
		return err
	}

	// make copy of replicas
	tmpReplicas := make([]types.Replica, len(replicas))
	copy(tmpReplicas, replicas)
	c.IsSnapDeletionInProgress = true
	c.Unlock()
	defer func() {
		c.Lock()
		c.IsSnapDeletionInProgress = false
		c.Unlock()
	}()
	reps := []rest.Replica{}
	for _, rep := range tmpReplicas {
		r := rest.Replica{
			Address: rep.Address,
			Mode:    string(rep.Mode),
		}
		reps = append(reps, r)
	}

	snapshots, err := listSnapshots(c, reps)
	if err != nil {
		return err
	}

	if len(snapshots) < 10 {
		return nil
	}

	c.SnapshotName = snapshots[len(snapshots)-1]
	logrus.Infof("Delete snapshot: %v", c.SnapshotName)
	if err := c.DeleteSnapshot(tmpReplicas); err != nil {
		return err
	}
	logrus.Infof("Deleted snapshot: %v successfully", c.SnapshotName)

	// Notify again to start snap deletion
	// if no of snapshots are greater than 10
	// this function will start deleting the snapshots
	// but if it's less then 10 goroutine will be exited
	c.StartAutoSnapDeletion <- true
	return nil
}
