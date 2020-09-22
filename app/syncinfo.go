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
	"strings"
	"text/tabwriter"

	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func SyncInfoCmd() cli.Command {
	return cli.Command{
		Name: "syncinfo",
		Action: func(c *cli.Context) {
			if err := getSyncInfo(c); err != nil {
				logrus.Fatalf("Error running syncinfo command: %v", err)
			}
		},
	}
}

func getSyncInfo(c *cli.Context) error {
	controllerClient := getCli(c)

	reps, err := controllerClient.ListReplicas()
	if err != nil {
		return err
	}

	var (
		info      *types.SyncInfo
		woReplica string
	)
	format := "%v	%v	%v	%v
"
	tw := tabwriter.NewWriter(os.Stdout, 0, 20, 1, ' ', 0)
	for _, r := range reps {
		if r.Mode != "WO" {
			continue
		}
		woReplica = r.Address
		info, err = getRebuildInfo(r.Address)
		if err != nil {
			return err
		}
		break
	}

	if info == nil {
		fmt.Println("No replicas are undergoing sync process")
		return nil
	} else if info.Snapshots == nil {
		fmt.Println("Sync process not yet started on ", woReplica)
		return nil
	}

	fmt.Fprintf(tw, "%v	%v,	%v	%v
", "DegradedReplica: ", strings.
		TrimSuffix(strings.TrimPrefix(info.WOReplica, "http://"), ":9502/v1"),
		"WOSnapshotsTotalSizeTobeSynced: ", info.WOSnapshotsTotalSize)
	fmt.Fprintf(tw, "%v	%v,	%v	%v
", "HealthyReplica: ", strings.
		TrimSuffix(strings.TrimPrefix(info.RWReplica, "http://"), ":9502/v1"),
		"RWSnapshotsTotalSizeToBeSynced: ", info.RWSnapshotsTotalSize)
	fmt.Fprintf(tw, "%s
", "============================================================================")
	fmt.Fprintf(tw, format, "Snapshot", "Status", "DegradedSize", "HealthySize")
	for _, snapInfo := range info.Snapshots {
		fmt.Fprintf(tw, format, strings.TrimPrefix(snapInfo.Name, "volume-snap-"), snapInfo.Status, snapInfo.WOSize, snapInfo.RWSize)
	}
	tw.Flush()

	return nil
}

func getRebuildInfo(address string) (*types.SyncInfo, error) {
	repClient, err := replicaClient.NewReplicaClient(address)
	if err != nil {
		return nil, err
	}

	info, err := repClient.GetRebuildInfo()
	if err != nil {
		return nil, err
	}

	return &info.SyncInfo, err
}
