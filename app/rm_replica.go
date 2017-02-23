package app

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/rancher/longhorn/controller/client"
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
	return err
}

func AutoRmReplica(frontendIP string, replica string) error {
	url := "http://" + frontendIP + ":9501"
	controllerClient := client.NewControllerClient(url)
	_, err := controllerClient.DeleteReplica(replica)
	return err
}
