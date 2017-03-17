package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/go-units"
	"github.com/rancher/longhorn/controller/client"
	"github.com/rancher/longhorn/replica"
	"github.com/rancher/longhorn/replica/rest"
	"github.com/rancher/longhorn/replica/rpc"
	"github.com/rancher/longhorn/util"
)

func ReplicaCmd() cli.Command {
	return cli.Command{
		Name:      "replica",
		UsageText: "longhorn controller DIRECTORY SIZE",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "listen",
				Value: "localhost:9502",
			},
			cli.StringFlag{
				Name:  "frontendIP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "backing-file",
				Usage: "qcow file to use as the base image of this disk",
			},
			cli.BoolTFlag{
				Name: "sync-agent",
			},
			cli.StringFlag{
				Name:  "size",
				Usage: "Volume size in bytes or human readable 42kb, 42mb, 42gb",
			},
		},
		Action: func(c *cli.Context) {
			if err := startReplica(c); err != nil {
				logrus.Fatalf("Error running start replica command: %v", err)
			}
		},
	}
}

func CheckReplicaState(frontendIP string, replicaIP string) (string, error) {
	url := "http://" + frontendIP + ":9501"
	ControllerClient := client.NewControllerClient(url)
	reps, err := ControllerClient.ListReplicas()
	if err != nil {
		return "", err
	}

	for _, rep := range reps {
		if rep.Address == replicaIP {
			return rep.Mode, nil
		}
	}
	return "", err
}

func AutoConfigureReplica(s *replica.Server, frontendIP string, address string) {
checkagain:
	state, err := CheckReplicaState(frontendIP, address)
	if err == nil && (state == "" || state == "ERR") {
		fmt.Println("Removing Replica")
	} else {
		time.Sleep(5 * time.Second)
		goto checkagain
	}
	s.Close()
	AutoRmReplica(frontendIP, address)
	AutoAddReplica(frontendIP, address)
	select {
	case <-s.MonitorChannel:
		goto checkagain
	}
}

func startReplica(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("directory name is required")
	}

	dir := c.Args()[0]
	backingFile, err := openBackingFile(c.String("backing-file"))
	if err != nil {
		return err
	}

	s := replica.NewServer(dir, backingFile, 512)

	address := c.String("listen")
	frontendIP := c.String("frontendIP")
	size := c.String("size")
	if size != "" {
		size, err := units.RAMInBytes(size)
		if err != nil {
			return err
		}

		if err := s.Create(size); err != nil {
			return err
		}
	}

	controlAddress, dataAddress, syncAddress, err := util.ParseAddresses(address)
	if err != nil {
		return err
	}

	resp := make(chan error)

	go func() {
		server := rest.NewServer(s)
		router := http.Handler(rest.NewRouter(server))
		router = util.FilteredLoggingHandler(map[string]struct{}{
			"/ping":          struct{}{},
			"/v1/replicas/1": struct{}{},
		}, os.Stdout, router)
		logrus.Infof("Listening on control %s", controlAddress)
		resp <- http.ListenAndServe(controlAddress, router)
	}()

	go func() {
		rpcServer := rpc.New(dataAddress, s)
		logrus.Infof("Listening on data %s", dataAddress)
		resp <- rpcServer.ListenAndServe()
	}()

	if c.Bool("sync-agent") {
		exe, err := exec.LookPath(os.Args[0])
		if err != nil {
			return err
		}

		exe, err = filepath.Abs(exe)
		if err != nil {
			return err
		}

		go func() {
			cmd := exec.Command(exe, "sync-agent", "--listen", syncAddress)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGKILL,
			}
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			resp <- cmd.Run()
		}()
	}
	if frontendIP != "" {
		go AutoConfigureReplica(s, frontendIP, "tcp://"+address)
	}

	return <-resp
}
