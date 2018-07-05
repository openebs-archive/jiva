package app

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	if !util.ValidVolumeName(name) {
		return errors.New("invalid target name")
	}

	backends := c.StringSlice("enable-backend")
	replicas := c.StringSlice("replica")
	frontendName := c.String("frontend")
	if frontendName == "gotgt" {
		frontendIP = c.String("frontendIP")
		clusterIP = c.String("clusterIP")
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

	signalChan := make(chan os.Signal, 5)
	signal.Notify(signalChan, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for {
			s := <-signalChan
			switch s {
			case syscall.SIGUSR1:
				controller.Delay += 2
			case syscall.SIGUSR2:
				controller.Delay -= 2
			}
			logrus.Infof("controller Delay for testing changed to: %d\n", controller.Delay)
		}
	}()

	var frontend types.Frontend
	if frontendName != "" {
		f, ok := frontends[frontendName]
		if !ok {
			return fmt.Errorf("Failed to find frontend: %s", frontendName)
		}
		frontend = f
	}

	control := controller.NewController(name, frontendIP, clusterIP, dynamic.New(factories), frontend)
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
