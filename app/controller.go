package app

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/gorilla/handlers"
	"github.com/openebs/jiva/backend/dynamic"
	"github.com/openebs/jiva/backend/file"
	"github.com/openebs/jiva/backend/remote"
	"github.com/openebs/jiva/controller"
	"github.com/openebs/jiva/controller/rest"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
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

func startController(c *cli.Context) error {
	var frontendIP, clusterIP string
	var controlListener string
	if c.NArg() == 0 {
		return errors.New("volume name is required")
	}
	name := c.Args()[0]
	replicationFactor, _ := strconv.ParseInt(os.Getenv("REPLICATION_FACTOR"), 10, 32)
	if replicationFactor == 0 {
		logrus.Infof("REPLICATION_FACTOR env not set")
	} else {
		logrus.Infof("REPLICATION_FACTOR: %v", replicationFactor)
	}

	if !util.ValidVolumeName(name) {
		return errors.New("invalid target name")
	}

	backends := c.StringSlice("enable-backend")
	replicas := c.StringSlice("replica")
	frontendName := c.String("frontend")
	if frontendName == "gotgt" {
		frontendIP = c.String("frontendIP")
		clusterIP = c.String("clusterIP")
		logrus.Infof("Starting controller with frontendIP: %v, and clusterIP: %v", frontendIP, clusterIP)
	}
	controlListener = c.String("listen")
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

	var frontend types.Frontend
	if frontendName != "" {
		f, ok := frontends[frontendName]
		if !ok {
			return fmt.Errorf("Failed to find frontend: %s", frontendName)
		}
		frontend = f
	}

	control := controller.NewController(name, frontendIP, clusterIP, dynamic.New(factories), frontend, int(replicationFactor))
	server := rest.NewServer(control)
	router := http.Handler(rest.NewRouter(server))

	router = util.FilteredLoggingHandler(map[string]struct{}{
		"/v1/volumes":  {},
		"/v1/replicas": {},
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
