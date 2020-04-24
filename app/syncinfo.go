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
		Name:      "syncinfo",
		ShortName: "ls",
		Action: func(c *cli.Context) {
			if err := getSyncInfo(c); err != nil {
				logrus.Fatalf("Error running ls command: %v", err)
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

	var info *types.SyncInfo
	format := "%v\t%v\t%v\t%v\n"
	tw := tabwriter.NewWriter(os.Stdout, 0, 20, 1, ' ', 0)
	for _, r := range reps {
		if r.Mode == "ERR" || r.Mode == "RW" {
			continue
		}
		info, err = getRebuildInfo(r.Address)
		if err != nil {
			return err
		}
	}

	if info == nil {
		fmt.Println("No replicas are undergoing sync process")
		return nil
	} else if info.Snapshots == nil {
		fmt.Println("Sync process not yet started")
		return nil
	}

	fmt.Fprintf(tw, "%v\t%v,\t%v\t%v\n", "DegradedReplica: ", strings.
		TrimSuffix(strings.TrimPrefix(info.WOReplica, "http://"), ":9502/v1"),
		"WOSnapshotsTotalSize: ", info.WOSnapshotsTotalSize)
	fmt.Fprintf(tw, "%v\t%v,\t%v\t%v\n", "HealthyReplica: ", strings.
		TrimSuffix(strings.TrimPrefix(info.RWReplica, "http://"), ":9502/v1"),
		"RWSnapshotsTotalSize: ", info.RWSnapshotsTotalSize)
	fmt.Fprintf(tw, "%s\n", "============================================================================")
	fmt.Fprintf(tw, format, "Snapshot", "Status", "DegradedSize", "HealthySize")
	for snap, snapInfo := range info.Snapshots {
		fmt.Fprintf(tw, format, strings.TrimPrefix(snap, "volume-snap-"), snapInfo.Status, snapInfo.WOSize, snapInfo.RWSize)
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
