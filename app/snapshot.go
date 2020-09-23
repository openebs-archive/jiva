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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/openebs/jiva/alertlog"
	"github.com/openebs/jiva/controller/rest"
	"github.com/openebs/jiva/replica"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/sync"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const VolumeHeadName = "volume-head"

var validSubCommands = map[string]bool{"ls": true, "rm": true, "info": true}

func isValidSubCommand(c *cli.Context) bool {
	args := c.Args()
	if len(args) < 1 {
		return true
	}
	ok, _ := validSubCommands[args[0]]
	if ok {
		return true
	}
	return false
}

func listSubCommands() []string {
	var cmds []string
	for k := range validSubCommands {
		cmds = append(cmds, k)
	}
	return cmds
}

func SnapshotCmd() cli.Command {
	return cli.Command{
		Name:      "snapshots",
		ShortName: "snapshot",
		Subcommands: []cli.Command{
			//			SnapshotCreateCmd(),
			//		SnapshotRevertCmd(),
			SnapshotLsCmd(),
			SnapshotRmCmd(),
			SnapshotInfoCmd(),
		},
		Action: func(c *cli.Context) {
			if !isValidSubCommand(c) {
				logrus.Fatalf("Error running snapshot command, invalid sub command: %v, supported sub commands are: %v", c.Args(), listSubCommands())
			}
			if err := lsSnapshot(c); err != nil {
				logrus.Fatalf("Error running snapshot command: %v", err)
			}
		},
	}
}

func SnapshotCreateCmd() cli.Command {
	return cli.Command{
		Name: "create",
		Action: func(c *cli.Context) {
			if err := createSnapshot(c); err != nil {
				logrus.Fatalf("Error running create snapshot command: %v", err)
			}
		},
	}
}

func SnapshotRevertCmd() cli.Command {
	return cli.Command{
		Name: "revert",
		Action: func(c *cli.Context) {
			if err := revertSnapshot(c); err != nil {
				logrus.Fatalf("Error running revert snapshot command: %v", err)
			}
		},
	}
}

func SnapshotRmCmd() cli.Command {
	return cli.Command{
		Name: "rm",
		Action: func(c *cli.Context) {
			if err := rmSnapshot(c); err != nil {
				logrus.Fatalf("Error running rm snapshot command: %v", err)
			}
		},
	}
}

func SnapshotLsCmd() cli.Command {
	return cli.Command{
		Name: "ls",
		Action: func(c *cli.Context) {
			if err := lsSnapshot(c); err != nil {
				logrus.Fatalf("Error running ls snapshot command: %v", err)
			}
		},
	}
}

func SnapshotInfoCmd() cli.Command {
	return cli.Command{
		Name: "info",
		Action: func(c *cli.Context) {
			if err := infoSnapshot(c); err != nil {
				logrus.Fatalf("Error running snapshot info command: %v", err)
			}
		},
	}
}

func createSnapshot(c *cli.Context) error {
	cli := getCli(c)

	var name string
	if len(c.Args()) > 0 {
		name = c.Args()[0]
	}
	id, err := cli.Snapshot(name)
	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.snapshot.create.failure",
			"msg", "Failed to create Jiva snapshot",
			"rname", name,
		)
		return err
	}

	fmt.Println(id)
	alertlog.Logger.Infow("",
		"eventcode", "jiva.snapshot.create.success",
		"msg", "Successfully created Jiva snapshot",
		"rname", name,
	)
	return nil
}

func revertSnapshot(c *cli.Context) error {
	cli := getCli(c)

	name := c.Args()[0]
	if name == "" {
		return fmt.Errorf("Missing parameter for snapshot")
	}

	err := cli.RevertSnapshot(name)
	if err != nil {
		alertlog.Logger.Errorw("",
			"eventcode", "jiva.snapshot.revert.failure",
			"msg", "Failed to revert Jiva snapshot",
			"rname", name,
		)
		return err
	}
	alertlog.Logger.Infow("",
		"eventcode", "jiva.snapshot.create.success",
		"msg", "Successfully reverted Jiva snapshot",
		"rname", name,
	)
	return nil
}

func rmSnapshot(c *cli.Context) error {
	var lastErr error
	url := c.GlobalString("url")
	task := sync.NewTask(url)
	if len(c.Args()) < 1 {
		return fmt.Errorf("snapshot name is empty")
	}
	for _, name := range c.Args() {
		if err := task.DeleteSnapshot(name); err == nil {
			fmt.Printf("deleted snapshot: %s\n", name)
			alertlog.Logger.Infow("",
				"eventcode", "jiva.snapshot.remove.success",
				"msg", "Successfully removed Jiva snapshot",
				"rname", name,
			)
		} else {
			lastErr = err
			fmt.Fprintf(os.Stderr, "Failed to delete snapshot: %s, error: %v\n", name, err)
			alertlog.Logger.Errorw("",
				"eventcode", "jiva.snapshot.remove.failure",
				"msg", "Failed to remove Jiva snapshot",
				"rname", name,
			)
		}
	}

	return lastErr
}

func getCommonSnapshots(replicas []rest.Replica) ([]string, error) {
	first := true
	snapshots := []string{}
	for _, r := range replicas {
		if r.Mode != "RW" {
			continue
		}

		if first {
			first = false
			chain, err := getChain(r.Address)
			if err != nil {
				return nil, err
			}
			// Replica can just started and haven't prepare the head
			// file yet
			if len(chain) == 0 {
				break
			}
			snapshots = chain[1:]
			continue
		}

		chain, err := getChain(r.Address)
		if err != nil {
			return nil, err
		}

		snapshots = util.Filter(snapshots, func(i string) bool {
			return util.Contains(chain, i)
		})
	}
	return snapshots, nil
}

func getAllSnapshots(replicas []rest.Replica) ([]string, error) {
	first := true
	finalSnapshotList := []string{}

	for _, r := range replicas {
		snapshots := []string{}
		diffSnapshots := []string{}
		if r.Mode != "RW" {
			continue
		}
		replica, err := getReplica(r.Address)
		if err != nil {
			return nil, err
		}
		// Replica has just been started and haven't prepared the head
		// file yet
		if len(replica.Chain) == 0 {
			break
		}
		for i, snap := range replica.Chain {
			if i == 0 {
				continue
			}
			if !replica.Disks[snap].Removed {
				snapshots = append(snapshots, snap)
			}
		}

		if !first {
			diffSnapshots = util.Filter(snapshots, func(i string) bool {
				return !util.Contains(finalSnapshotList, i)
			})
			finalSnapshotList = append(finalSnapshotList, diffSnapshots...)
		} else {
			finalSnapshotList = append(finalSnapshotList, snapshots...)
		}
		first = false
	}
	return finalSnapshotList, nil
}

func lsSnapshot(c *cli.Context) error {
	cli := getCli(c)

	replicas, err := cli.ListReplicas()
	if err != nil {
		return err
	}

	snapshots := []string{}
	snapshots, err = getAllSnapshots(replicas)
	if err != nil {
		return err
	}

	format := "%s\n"
	tw := tabwriter.NewWriter(os.Stdout, 0, 20, 1, ' ', 0)
	fmt.Fprintf(tw, format, "ID")
	for _, s := range snapshots {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "volume-snap-"), ".img")
		fmt.Fprintf(tw, format, s)
	}
	tw.Flush()

	return nil
}

func infoSnapshot(c *cli.Context) error {
	var output []byte

	outputDisks := make(map[string]types.DiskInfo)
	cli := getCli(c)

	replicas, err := cli.ListReplicas()
	if err != nil {
		return err
	}

	for _, r := range replicas {
		if r.Mode != "RW" {
			continue
		}

		disks, err := getDisks(r.Address)
		if err != nil {
			return err
		}

		for name, disk := range disks {
			snapshot := ""

			if !replica.IsHeadDisk(name) {
				snapshot, err = replica.GetSnapshotNameFromDiskName(name)
				if err != nil {
					return err
				}
			} else {
				snapshot = VolumeHeadName
			}
			children := []string{}
			for _, childDisk := range disk.Children {
				child := ""
				if !replica.IsHeadDisk(childDisk) {
					child, err = replica.GetSnapshotNameFromDiskName(childDisk)
					if err != nil {
						return err
					}
				} else {
					child = VolumeHeadName
				}
				children = append(children, child)
			}
			parent := ""
			if disk.Parent != "" {
				parent, err = replica.GetSnapshotNameFromDiskName(disk.Parent)
				if err != nil {
					return err
				}
			}
			info := types.DiskInfo{
				Name:            snapshot,
				Parent:          parent,
				Removed:         disk.Removed,
				UserCreated:     disk.UserCreated,
				Children:        children,
				Created:         disk.Created,
				Size:            disk.Size,
				RevisionCounter: disk.RevisionCounter,
			}
			if _, exists := outputDisks[snapshot]; !exists {
				outputDisks[snapshot] = info
			} else {
				// Consolidate the result of snapshot in removing process
				if info.Removed && !outputDisks[snapshot].Removed {
					t := outputDisks[snapshot]
					t.Removed = true
					outputDisks[snapshot] = t
				}
			}
		}

	}

	output, err = json.MarshalIndent(outputDisks, "", "	")
	if err != nil {
		return err
	}

	if output == nil {
		return fmt.Errorf("Cannot find suitable replica for snapshot info")
	}
	fmt.Println(string(output))
	return nil
}

func getDisks(address string) (map[string]types.DiskInfo, error) {
	repClient, err := replicaClient.NewReplicaClient(address)
	if err != nil {
		return nil, err
	}

	r, err := repClient.GetReplica()
	if err != nil {
		return nil, err
	}

	return r.Disks, err
}
