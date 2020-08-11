package main

import (
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
	"github.com/sirupsen/logrus"
)

func addReplica(replica string) error {
	task := sync.NewTask(replica + ":9501")
	return task.AddReplica(replica, nil)
}
func autoAddReplica(s *replica.Server, frontendIP string, replica string, replicaType string) error {
	var err error
	url := "http://" + frontendIP + ":9501"
	task := sync.NewTask(url)
	if err = task.AddReplica(replica, s); err != nil {
		logrus.Errorf("jiva.volume.replica.add.failure rname:%v err: %v", replica, err)
		return err
	}
	logrus.Infof("jiva.volume.replica.add.success rname: %v", replica)
	return nil
}
