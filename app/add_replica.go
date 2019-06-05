package app

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
)

func AddReplicaCmd() cli.Command {
	return cli.Command{
		Name:      "add-replica",
		ShortName: "add",
		Action: func(c *cli.Context) {
			if err := addReplica(c); err != nil {
				logrus.Fatalf("Error running add replica command: %v", err)
			}
		},
	}
}

func addReplica(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("replica address is required")
	}
	replica := c.Args()[0]

	url := c.GlobalString("url")
	task := sync.NewTask(url)
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
	return err
}
