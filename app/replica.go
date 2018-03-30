package app

import (
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/go-units"
	"github.com/openebs/jiva/controller/client"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/replica/rpc"
	"github.com/openebs/jiva/sync"
	"github.com/openebs/jiva/util"
)

func ReplicaCmd() cli.Command {
	return cli.Command{
		Name:      "replica",
		UsageText: "longhorn controller DIRECTORY SIZE",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "listen",
				Value: ":9502",
			},
			cli.StringFlag{
				Name:  "frontendIP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "cloneIP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "snapName",
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
			cli.StringFlag{
				Name:  "type",
				Value: "",
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

func AutoConfigureReplica(s *replica.Server, frontendIP string, address string, replicaType string) {
checkagain:
	state, err := CheckReplicaState(frontendIP, address)
	if err == nil && (state == "" || state == "ERR") {
		logrus.Infof("Removing Replica")
	} else {
		time.Sleep(5 * time.Second)
		goto checkagain
	}
	s.Close()
	AutoRmReplica(frontendIP, address)
	AutoAddReplica(frontendIP, address, replicaType)
	select {
	case <-s.MonitorChannel:
		goto checkagain
	}
}

func CloneReplica(s *replica.Server, address string, cloneIP string, snapName string) error {
	var err error
	url := "http://" + cloneIP + ":9501"
	task := sync.NewTask(url)
	if err = task.CloneReplica(url, address, cloneIP, snapName); err != nil {
		return err
	}
	if s.Replica() != nil {
		s.Replica().SetCloneStatus("completed")
	}
	return err
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

	replicaType := c.String("type")
	s := replica.NewServer(dir, backingFile, 512, replicaType)

	address := c.String("listen")
	frontendIP := c.String("frontendIP")
	cloneIP := c.String("cloneIP")
	snapName := c.String("snapName")
	size := c.String("size")
	if size != "" {
		//Units bails with an error size is provided with i, like Gi
		//The following will convert - G, Gi, GiB into G
		size = strings.Split(size, "i")[0]
		size, err := units.RAMInBytes(size)
		if err != nil {
			return err
		}

		if err := s.Create(size); err != nil {
			return err
		}
	}

	if address == ":9502" {
		host, _ := os.Hostname()
		addrs, _ := net.LookupIP(host)
		for _, addr := range addrs {
			if ipv4 := addr.To4(); ipv4 != nil {
				address = ipv4.String()
				if address == "127.0.0.1" {
					address = address + ":9502"
					continue
				}
				address = address + ":9502"
				break
			}
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
		if address == ":9502" {
			address = "localhost:9502"
		}
		go AutoConfigureReplica(s, frontendIP, "tcp://"+address, replicaType)
	}
	for s.Replica() == nil {
		time.Sleep(2 * time.Second)
	}
	if replicaType == "clone" && snapName != "" {
		logrus.Infof("Starting clone process\n")
		status := s.Replica().GetCloneStatus()
		if status != "completed" {
			s.Replica().SetCloneStatus("inProgress")
			if err = CloneReplica(s, "tcp://"+address, cloneIP, snapName); err != nil {
				s.Replica().SetCloneStatus("error")
				return err
			}
		}
		s.Replica().SetCloneStatus("completed")
		logrus.Infof("Clone process completed successfully\n")
	} else {
		s.Replica().SetCloneStatus("NA")
	}

	return <-resp
}
