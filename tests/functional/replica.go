package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/go-units"
	"github.com/openebs/jiva/controller/client"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/rpc"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
)

func startLoggingToFile(dir string) error {
	// TODO Revisit
	lf := util.LogToFile{
		Enable:          true,
		MaxLogFileSize:  1024 * 1024 * 1024,
		MaxBackups:      2,
		RetentionPeriod: 500,
	}

	path := dir + "/" + util.LogInfo
	_, err := os.Stat(path)
	if err == nil {
		logrus.Info("Read log info")
		lf, err = util.ReadLogInfo(dir)
		if err != nil {
			return err
		}
	} else if err != nil && os.IsNotExist(err) {
		// do nothing as file will be created in StartLoggingToFile
	} else if err != nil {
		return err
	}

	if !lf.Enable {
		logrus.Info("Logging to file is not enabled")
		return nil
	}

	return util.StartLoggingToFile(dir, lf)
}

func checkReplicaState(frontendIP string, replicaIP string) (string, error) {
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

func autoConfigureReplica(s *replica.Server, frontendIP string, address string, replicaType string) error {
	retryCount := 10
checkagain:
	retryCount--
	state, err := checkReplicaState(frontendIP, address)
	if err != nil {
		logrus.Warningf("Failed to check replica state, err: %v, will retry (%v retry left)", err, retryCount)
		if retryCount == 0 {
			logrus.Infof("Retry count exceeded, Shutting down...")
		}
		time.Sleep(5 * time.Second)
		goto checkagain
	} else if state == "ERR" {
		retryCount++
		logrus.Infof("Got replica state: %v", state)
		time.Sleep(5 * time.Second)
		goto checkagain
	} else {
		_ = s.Close()
		if err := autoAddReplica(s, frontendIP, address, replicaType); err != nil {
			s.Close()
			return err
		}
	}
	return nil
}

func (config *testConfig) startTestReplica(replicaIP, dir string, startSyncAgent bool) error {

	close := false
	restart := true
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
		if config.Replicas[replicaIP].RPCServer != nil {
			config.Replicas[replicaIP].RPCServer.Stop()
		}
		if config.Replicas[replicaIP].RestServer != nil {
			config.Replicas[replicaIP].RestServer.Shutdown(context.TODO())
		}
		if config.Replicas[replicaIP].Server != nil {
			config.Replicas[replicaIP].Server.Close()
		}
		close = true
		if restart {
			config.AddToReplicaRestartList(replicaIP)
		}
	}()
	address := replicaIP + ":" + "9502"

	types.MaxChainLength, _ = strconv.Atoi(os.Getenv("MAX_CHAIN_LENGTH"))
	if types.MaxChainLength == 0 {
		logrus.Infof("MAX_CHAIN_LENGTH env not set, default value is 512")
	} else {
		logrus.Infof("MAX_CHAIN_LENGTH: %v", types.MaxChainLength)
	}

	replicaType := "backend"
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		logrus.Errorf("failed to create directory: %s", dir)
		return err
	}

	//if err := startLoggingToFile(dir); err != nil {
	//	return err
	//}
	s := replica.NewServer(address, dir, 512, replicaType)
	go replica.CreateHoles()
	config.Replicas[replicaIP].Server = s
	frontendIP := config.ControllerIP
	size := config.Size
	if size != "" {
		size = strings.Split(size, "i")[0]
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

	logrus.Infof("Starting replica having replicaType: %v, frontendIP: %v, size: %v, dir: %v", replicaType, frontendIP, size, dir)
	logrus.Infof("Setting replicaAddr: %v, controlAddr: %v, dataAddr: %v, syncAddr: %v", address, controlAddress, dataAddress, syncAddress)
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
		srv := &http.Server{Addr: controlAddress, Handler: router}
		config.Replicas[replicaIP].RestServer = srv
		controlResp <- srv.ListenAndServe()
	}()

	go func() {
		logrus.Infof("Listening on data %s", dataAddress)
		rpcResp <- config.ListenAndServeTest(s, dataAddress, replicaIP)
	}()

	// Sync Agent
	//exe, err := exec.LookPath(os.Args[0])
	exe, err := exec.LookPath("./jiva")
	if err != nil {
		return err
	}

	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if startSyncAgent {
		go func() {
			cmd := exec.Command(exe, "sync-agent", "--listen", syncAddress)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGKILL,
			}
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			httpTimeout := os.Getenv(types.SyncHTTPClientTimeoutKey)
			if httpTimeout != "" {
				cmd.Env = []string{types.SyncHTTPClientTimeoutKey + "=" + httpTimeout}
			}
			syncResp <- cmd.Run()
		}()
	}
	go func() {
		for s.Replica() == nil && (!close) {
			time.Sleep(2 * time.Second)
		}
		if close {
			return
		}
		if err := s.Replica().SetCloneStatus("NA"); err != nil {
			logrus.Error("Error in setting the clone status as 'NA'")
		}

	}()
	// Waiting for servers to start
	time.Sleep(5 * time.Second)
	if config.ControllerIP != "" {
		if err := autoConfigureReplica(s, frontendIP, "tcp://"+address, replicaType); err != nil {
			return err
		}
	}

	select {
	case err = <-controlResp:
		logrus.Errorf("jiva.volume.replica.api.exited rname: %v, err: %v", address, err)
	case err = <-rpcResp:
		logrus.Errorf("jiva.volume.replica.rpc.exited rname: %v, err: %v", address, err)
	case <-config.Close[replicaIP]:
		restart = false
	}
	return nil
}

func (config *testConfig) StopTestReplica(replicaIP string) {
	config.Close[replicaIP] <- struct{}{}
}

func (config *testConfig) RestartTestReplica(replicaIP string) {
	config.AddToReplicaRestartList(replicaIP)
}

func (config *testConfig) AddToReplicaRestartList(replicaIP string) {
	config.Lock()
	config.ReplicaRestartList[replicaIP] = time.Now()
	config.Unlock()
}

func (config *testConfig) ListenAndServeTest(replicaSrv *replica.Server, dataAddress string, replicaIP string) error {
	addr, err := net.ResolveTCPAddr("tcp", dataAddress)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}
		err = conn.SetKeepAlive(true)
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}

		err = conn.SetKeepAlivePeriod(10 * time.Second)
		if err != nil {
			logrus.Errorf("failed to accept connection %v", err)
			continue
		}

		logrus.Infof("New connection from: %v", conn.RemoteAddr())

		server := rpc.NewServer(conn, replicaSrv)
		config.Replicas[replicaIP].RPCServer = server
		if err := server.Handle(); err != nil {
			return err
		}
	}
}

func (config *testConfig) MonitorReplicas() {
	for {
		var (
			rep      string
			downTime time.Time
			found    bool
		)
		time.Sleep(5 * time.Second)
		if config.Stop {
			return
		}
		config.Lock()
		if len(config.ReplicaRestartList) == 0 {
			config.Unlock()
			continue
		}
		for rep, downTime = range config.ReplicaRestartList {
			if rep != "" && time.Since(downTime).Seconds() >= float64(20) {
				found = true
				break
			}
		}
		if !found {
			config.Unlock()
			continue
		}
		logrus.Infof("CLOSED REPLICA: %v: %v", rep, time.Since(downTime).Seconds())
		if config.isReplicaAttachedToController(rep) {
			logrus.Infof("Replica %v still connected to controller, skip restarting", rep)
			config.Unlock()
			continue
		}
		go func(replica string) {
			err := config.startTestReplica(replica, replica+"vol", false)
			if err != nil {
				logrus.Infof("ERROR: %v", err)
			}
		}(rep)
		delete(config.ReplicaRestartList, rep)
		config.Unlock()
	}
}

func (config *testConfig) startReplicas() {
	// Start 3 Replicas in debug mode
	for replica := range config.Replicas {
		rep := replica
		go func(replica string) {
			config.startTestReplica(replica, replica+"vol", true)
		}(rep)
	}
}

func (config *testConfig) stopReplicas() {
	for replica := range config.Replicas {
		rep := replica
		go func(replica string) {
			config.StopTestReplica(replica)
		}(rep)
	}
}

func (config *testConfig) cleanReplicaDirs() {
	for replica := range config.Replicas {
		rep := replica
		go func(replica string) {
			os.RemoveAll(replica + "vol")
		}(rep)
	}
}
func (config *testConfig) CreateAutoGeneratedSnapshots(count int, replica string, replicaCount int) {
	for i := 0; i < count; i++ {
		config.StopTestReplica("172.18.0.112")
		config.verifyRWReplicaCount(1)
		time.Sleep(2 * time.Second)
		go config.startTestReplica("172.18.0.112", "172.18.0.112"+"vol", true)
		config.verifyRWReplicaCount(2)
	}
}
