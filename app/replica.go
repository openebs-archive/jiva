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
	logrus.Infof("Replicastate: %v err:%v", state, err)
	if err == nil && (state == "" || state == "ERR") {
		s.Close(false)
	} else {
		time.Sleep(5 * time.Second)
		goto checkagain
	}
	AutoRmReplica(frontendIP, address)
	AutoAddReplica(s, frontendIP, address, replicaType)
	logrus.Infof("Waiting on MonitorChannel")
	select {
	case <-s.MonitorChannel:
		logrus.Infof("Restart AutoConfigure Process")
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
		err = s.Replica().SetCloneStatus("completed")
	}
	return err
}

func startReplica(c *cli.Context) error {

	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)

	if c.NArg() != 1 {
		return errors.New("directory name is required")
	}

	dir := c.Args()[0]
	replicaType := c.String("type")
	s := replica.NewServer(dir, 512, replicaType)
	go replica.CreateHoles()

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

	logrus.Infof("Starting replica having replicaType: %v, frontendIP: %v, size: %v, dir: %v", replicaType, frontendIP, size, dir)
	logrus.Infof("Setting replicaAddr: %v, controlAddr: %v, dataAddr: %v, syncAddr: %v", address, controlAddress, dataAddress, syncAddress)

	var resp error
	controlResp := make(chan error)
	syncResp := make(chan error)
	rpcResp := make(chan error)

	go func() {
		server := rest.NewServer(s)
		router := http.Handler(rest.NewRouter(server))
		router = util.FilteredLoggingHandler(map[string]struct{}{
			"/ping":          {},
			"/v1/replicas/1": {},
		}, os.Stdout, router)
		logrus.Infof("Listening on control %s", controlAddress)
		controlResp <- http.ListenAndServe(controlAddress, router)
	}()

	go func() {
		rpcServer := rpc.New(dataAddress, s)
		logrus.Infof("Listening on data %s", dataAddress)
		rpcResp <- rpcServer.ListenAndServe()
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
			syncResp <- cmd.Run()
		}()
	}
	if frontendIP != "" {
		if address == ":9502" {
			address = "localhost:9502"
		}
		go AutoConfigureReplica(s, frontendIP, "tcp://"+address, replicaType)
	}
	for s.Replica() == nil {
		logrus.Infof("Waiting for s.Replica() to be non nil")
		time.Sleep(2 * time.Second)
	}
	if replicaType == "clone" && snapName != "" {
		logrus.Infof("Starting clone process\n")
		status := s.Replica().GetCloneStatus()
		if status != "completed" {
			logrus.Infof("Set clone status as inProgress")
			if err = s.Replica().SetCloneStatus("inProgress"); err != nil {
				logrus.Error("Error in setting the clone status as 'inProgress'")
				return err
			}
			if err = CloneReplica(s, "tcp://"+address, cloneIP, snapName); err != nil {
				logrus.Error("Error in cloning replica, setting clone status as 'error'")
				if statusErr := s.Replica().SetCloneStatus("error"); err != nil {
					logrus.Errorf("Error in setting the clone status as 'error', found error:%v", statusErr)
					return err
				}
				return err
			}
		}
		logrus.Infof("Set clone status as Completed")
		if err := s.Replica().SetCloneStatus("completed"); err != nil {
			logrus.Error("Error in setting the clone status as 'completed'")
			return err
		}
		logrus.Infof("Clone process completed successfully\n")
	} else {
		logrus.Infof("Set clone status as NA")
		if err := s.Replica().SetCloneStatus("NA"); err != nil {
			logrus.Error("Error in setting the clone status as 'NA'")
			return err
		}
	}
	select {
	case resp = <-controlResp:
		logrus.Fatalf("Rest API exited: %v", resp)
	case resp = <-rpcResp:
		logrus.Fatalf("RPC listen exited: %v", resp)
	case resp = <-syncResp:
		logrus.Fatalf("Sync process exited: %v", resp)
	}
	return resp
}
