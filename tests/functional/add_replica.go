package main

import (
	"github.com/openebs/jiva/alertlog"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
)

func addReplica(replica string) error {
	task := sync.NewTask(replica + ":9501")
	return task.AddReplica(replica, nil)
}
func AutoAddReplica(s *replica.Server, frontendIP string, replica string, replicaType string) error {
	var err error
	url := "http://" + frontendIP + ":9501"
	task := sync.NewTask(url)
	if replicaType == "quorum" {
		err = task.AddQuorumReplica(replica, s)
	} else {
		err = task.AddReplica(replica, s)
	}
	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.add.failure",
			"msg", "Failed to add Jiva volume replica",
			"rname", replica,
		)
	} else {
		alertlog.Logger.Infow("",
			"eventcode", "jiva.volume.replica.add.success",
			"msg", "Successfully added Jiva volume replica",
			"rname", replica,
		)
	}
	return err
}
