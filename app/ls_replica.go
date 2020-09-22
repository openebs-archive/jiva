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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/openebs/jiva/controller/client"
	replicaClient "github.com/openebs/jiva/replica/client"
	replicaRest "github.com/openebs/jiva/replica/rest"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func LsReplicaCmd() cli.Command {
	return cli.Command{
		Name:      "ls-replica",
		ShortName: "ls",
		Action: func(c *cli.Context) {
			if err := lsReplica(c); err != nil {
				logrus.Fatalf("Error running ls command: %v", err)
			}
		},
	}
}

func getCli(c *cli.Context) *client.ControllerClient {
	url := c.GlobalString("url")
	return client.NewControllerClient(url)

}

func lsReplica(c *cli.Context) error {
	controllerClient := getCli(c)

	reps, err := controllerClient.ListReplicas()
	if err != nil {
		return err
	}

	format := "%s	%s	%v
"
	tw := tabwriter.NewWriter(os.Stdout, 0, 20, 1, ' ', 0)
	fmt.Fprintf(tw, format, "ADDRESS", "MODE", "CHAIN")
	for _, r := range reps {
		if r.Mode == "ERR" {
			fmt.Fprintf(tw, format, r.Address, r.Mode, "")
			continue
		}
		chain := interface{}("")
		chainList, err := getChain(r.Address)
		if err == nil {
			chain = chainList
		}
		fmt.Fprintf(tw, format, r.Address, r.Mode, chain)
	}
	tw.Flush()

	return nil
}

func getChain(address string) ([]string, error) {
	repClient, err := replicaClient.NewReplicaClient(address)
	if err != nil {
		return nil, err
	}

	r, err := repClient.GetReplica()
	if err != nil {
		return nil, err
	}

	return r.Chain, err
}

func getReplica(address string) (replicaRest.Replica, error) {
	repClient, err := replicaClient.NewReplicaClient(address)
	if err != nil {
		return replicaRest.Replica{}, err
	}

	return repClient.GetReplica()
}
