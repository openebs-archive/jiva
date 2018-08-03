// +build !debug

package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/controller/rest"
	"github.com/openebs/jiva/replica"
)

func (t *Task) AddReplica(replicaAddress string, s *replica.Server) error {
	var action string

	if s == nil {
		return fmt.Errorf("Server not present for %v, Add replica using CLI not supported", replicaAddress)
	}
	logrus.Infof("Addreplica %v", replicaAddress)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	Replica, err := replica.CreateTempReplica()
	if err != nil {
		logrus.Errorf("CreateTempReplica failed, err: %v", err)
		return err
	}
	server, err := replica.CreateTempServer()
	if err != nil {
		logrus.Errorf("CreateTempServer failed, err: %v", err)
		return err
	}
Register:
	logrus.Infof("Get Volume info from controller")
	volume, err := t.client.GetVolume()
	if err != nil {
		logrus.Errorf("Get volume info failed, err: %v", err)
		return err
	}
	addr := strings.Split(replicaAddress, "://")
	parts := strings.Split(addr[1], ":")
	if volume.ReplicaCount == 0 {
		revisionCount := Replica.GetRevisionCounter()
		peerDetails, _ := Replica.GetPeerDetails()
		replicaType := "Backend"
		upTime := time.Since(Replica.ReplicaStartTime)
		state, _ := server.PrevStatus()
		logrus.Infof("Register replica at controller")
		err := t.client.Register(parts[0], revisionCount, peerDetails, replicaType, upTime, string(state))
		if err != nil {
			logrus.Errorf("Error in sending register command, err: %v", err)
		}
		select {
		case <-ticker.C:
			logrus.Infof("TimedOut waiting for response from controller")
			goto Register
		case action = <-replica.ActionChannel:
		}
	}
	if action == "start" {
		logrus.Infof("Received start from controller")
		return t.client.Start(replicaAddress)
	}
	logrus.Infof("Check and reset 'failed rebuild' %v", replicaAddress)
	if err := t.checkAndResetFailedRebuild(replicaAddress, s); err != nil {
		logrus.Errorf("Check and reset 'failed rebuild' failed, err:%v", err)
		return err
	}

	logrus.Infof("Adding replica %s in WO mode", replicaAddress)
	_, err = t.client.CreateReplica(replicaAddress)
	if err != nil {
		logrus.Errorf("CreateReplica failed, err:%v", err)
		return err
	}

	logrus.Infof("getTransferClients %v", replicaAddress)
	fromClient, toClient, err := t.getTransferClients(replicaAddress)
	if err != nil {
		logrus.Errorf("getTransferClients failed, err:%v", err)
		return err
	}

	logrus.Infof("SetRebuilding %v", replicaAddress)
	if err := toClient.SetRebuilding(true); err != nil {
		logrus.Errorf("SetRebuilding failed, err:%v", err)
		return err
	}

	logrus.Infof("PrepareRebuild %v", replicaAddress)
	output, err := t.client.PrepareRebuild(rest.EncodeID(replicaAddress))
	if err != nil {
		logrus.Errorf("PrepareRebuild failed, err:%v", err)
		return err
	}

	logrus.Infof("syncFiles from:%v to:%v", fromClient, toClient)
	if err = t.syncFiles(fromClient, toClient, output.Disks); err != nil {
		return err
	}

	logrus.Infof("reloadAndVerify %v", replicaAddress)
	return t.reloadAndVerify(replicaAddress, toClient)

}
