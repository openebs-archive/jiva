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

	"github.com/openebs/jiva/controller/client"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func RmReplicaCmd() cli.Command {
	return cli.Command{
		Name:      "rm-replica",
		ShortName: "rm",
		Action: func(c *cli.Context) {
			if err := rmReplica(c); err != nil {
				logrus.Fatalf("Error running rm replica command: %v", err)
			}
		},
	}
}

func rmReplica(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("replica address is required")
	}
	replica := c.Args()[0]

	controllerClient := getCli(c)
	_, err := controllerClient.DeleteReplica(replica)
	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.remove.failure",
			"msg", "Failed to remove Jiva volume replica",
			"rname", replica,
		)
	} else {
		alertlog.Logger.Infow("",
			"eventcode", "jiva.volume.replica.remove.success",
			"msg", "Successfully removed Jiva volume replica",
			"rname", replica,
		)
	}
	return err
}

func AutoRmReplica(frontendIP string, replica string) error {
	url := "http://" + frontendIP + ":9501"
	controllerClient := client.NewControllerClient(url)
	_, err := controllerClient.DeleteReplica(replica)
	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.volume.replica.remove.failure",
			"msg", "Failed to remove Jiva volume replica",
			"rname", replica,
		)
	} else {
		alertlog.Logger.Infow("",
			"eventcode", "jiva.volume.replica.remove.success",
			"msg", "Successfully removed Jiva volume replica",
			"rname", replica,
		)
	}
	return err
}
