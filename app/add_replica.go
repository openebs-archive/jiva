/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package app

import (
	"errors"

	"github.com/openebs/jiva/alertlog"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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
