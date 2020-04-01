package app

import (
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/openebs/jiva/alertlog"
	"github.com/openebs/jiva/sync"

	"github.com/openebs/jiva/types"

	"github.com/docker/go-units"
	"github.com/openebs/jiva/controller/client"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/replica/rpc"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	maxLogFileSize         = "maxLogFileSize"
	retentionPeriod        = "retentionPeriod"
	maxBackups             = "maxBackups"
	defaultLogFileSize     = 100
	defaultRetentionPeriod = 28
	defaultMaxBackups      = 5
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
			cli.BoolFlag{
				Name:  "logtofile",
				Usage: "Logs of replica will also be written to the file along with stdout",
			},
			cli.IntFlag{
				Name:  retentionPeriod,
				Usage: "Retention period of log file in days",
				Value: defaultRetentionPeriod,
			},
			cli.IntFlag{
				Name:  maxLogFileSize,
				Usage: "Max size of log file in mb",
				Value: defaultLogFileSize,
			},
			cli.IntFlag{
				Name:  maxBackups,
				Usage: "Max number of log files to keep while creating new log file once size of log exceeds to maxLogFileSize",
				Value: defaultMaxBackups,
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
	retryCount := 10
checkagain:
	retryCount--
	state, err := CheckReplicaState(frontendIP, address)
	if err != nil {
		logrus.Warningf("Failed to check replica state, err: %v, will retry (%v retry left)", err, retryCount)
		if retryCount == 0 {
			logrus.Fatalf("Retry count exceeded, Shutting down...")
		}
		time.Sleep(5 * time.Second)
		goto checkagain
	} else if state == "ERR" {
		retryCount++
		logrus.Infof("Got replica state: %v", state)
		// replica may be in errored state marked by controller and
		// not yet removed. The cause of ERR state would be following:
		// - I/O error while R/W operations
		// - Failed to revert snapshot
		time.Sleep(5 * time.Second)
		goto checkagain
	} else {
		// Replica might be in rebuilding state, closing will change it to
		// closed state, then it can be registered, else register replica
		// will fail.
		_ = s.Close()
		// this is just to be sure that replica is not attached to
		// controller. Assumption is replica might be in RO, RW or in
		// "" state and not removed from controller.
		AutoRmReplica(frontendIP, address)
		if err := AutoAddReplica(s, frontendIP, address, replicaType); err != nil {
			s.Close()
			logrus.Fatalf("Failed to add replica to controller, err: %v, Shutting down...", err)
		}
	}
}

func CloneReplica(s *replica.Server, address string, cloneIP string, snapName string) error {
	var err error
	url := "http://" + cloneIP + ":9501"
	task := sync.NewTask(url)
	if err = task.CloneReplica(url, address, cloneIP, snapName); err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.clone.failure",
			"msg", "Failed to clone Jiva volume replica",
			"rname", snapName,
		)
		return err
	}
	if s.Replica() != nil {
		err = s.Replica().SetCloneStatus("completed")
	}

	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.clone.failure",
			"msg", "Failed to clone Jiva volume replica",
			"rname", snapName,
		)
	} else {
		alertlog.Logger.Infow("",
			"eventcode", "jiva.volume.replica.clone.success",
			"msg", "Successfully cloned Jiva volume replica",
			"rname", snapName,
		)
	}
	return err
}

func startLoggingToFile(c *cli.Context) {
	fileName := c.Args()[0] + "/replica.log"
	logrus.SetOutput(io.MultiWriter(os.Stderr, &lumberjack.Logger{
		Filename:   fileName,
		MaxSize:    c.Int(maxLogFileSize),
		MaxAge:     c.Int(retentionPeriod),
		MaxBackups: c.Int(maxBackups),
		LocalTime:  true,
	}))
	logrus.Infof("Configured logging with retentionPeriod: %v, maxLogFileSize: %v, maxBackups: %v",
		c.Int(retentionPeriod), c.Int(maxLogFileSize), c.Int(maxBackups))
}

func startReplica(c *cli.Context) error {

	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)

	if c.NArg() != 1 {
		return errors.New("directory name is required")
	}

	types.MaxChainLength, _ = strconv.Atoi(os.Getenv("MAX_CHAIN_LENGTH"))
	if types.MaxChainLength == 0 {
		logrus.Infof("MAX_CHAIN_LENGTH env not set, default value is 512")
	} else {
		logrus.Infof("MAX_CHAIN_LENGTH: %v", types.MaxChainLength)
	}

	dir := c.Args()[0]
	replicaType := c.String("type")
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		logrus.Errorf("failed to create directory: %s", dir)
		return err
	}

	if c.Bool("logtofile") {
		startLoggingToFile(c)
	}
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
			"/ping":                   {},
			"/v1/replicas/1":          {},
			"/v1/replicas/1/volusage": {},
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
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.api.exited",
			"msg", "Jiva volume replica API stopped",
			"rname", address,
		)
		logrus.Fatalf("Rest API exited: %v", resp)
	case resp = <-rpcResp:
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.rpc.exited",
			"msg", "Jiva volume replica RPC stopped",
			"rname", address,
		)
		logrus.Fatalf("RPC listen exited: %v", resp)
	case resp = <-syncResp:
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.sync.exited",
			"msg", "Jiva volume replica sync stopped",
			"rname", address,
		)
		logrus.Fatalf("Sync process exited: %v", resp)
	}

	alertlog.Logger.Infow("",
		"eventcode", "jiva.volume.replica.start.success",
		"msg", "Successfully started Jiva volume replica",
		"rname", address,
	)
	return resp
}
