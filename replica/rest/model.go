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

package rest

import (
	"strconv"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

type Replica struct {
	client.Resource
	types.ReplicaInfo
}

type DeleteReplicaOutput struct {
	client.Resource
}

type Stats struct {
	client.Resource
	ReplicaCounter  int64  `json:"replicacounter"`
	RevisionCounter string `json:"revisioncounter"`
}

type CreateInput struct {
	client.Resource
	Size string `json:"size"`
}

type RevertInput struct {
	client.Resource
	Name    string `json:"name"`
	Created string `json:"created"`
}

type RebuildingInput struct {
	client.Resource
	Rebuilding bool `json:"rebuilding"`
}

type LoggingInput struct {
	client.Resource
	LogToFile util.LogToFile `json:"logtofile"`
}

type SnapshotInput struct {
	client.Resource
	Name        string `json:"name"`
	UserCreated bool   `json:"usercreated"`
	Created     string `json:"created"`
}

// CloneUpdateInput is input to update clone info of cloned replica
type CloneUpdateInput struct {
	client.Resource
	SnapName      string `json:"snapname"`
	RevisionCount string `json:"revisioncounter"`
}

type RemoveDiskInput struct {
	client.Resource
	Name string `json:"name"`
}

type VolUsage struct {
	client.Resource
	RevisionCounter   string `json:"revisioncounter"`
	UsedLogicalBlocks string `json:"usedlogicalblocks"`
	UsedBlocks        string `json:"usedblocks"`
	SectorSize        string `json:"sectorSize"`
}

type RebuildInfoOutput struct {
	client.Resource
	SyncInfo types.SyncInfo `json:"syncInfo,omitempty"`
}

type ResizeInput struct {
	client.Resource
	Name string `json:"name"`
	Size string `json:"size"`
}

type ReplaceDiskInput struct {
	client.Resource
	Target string `json:"target"`
	Source string `json:"source"`
}

type PrepareRemoveDiskInput struct {
	client.Resource
	Name string `json:"name"`
}

type PrepareRemoveDiskOutput struct {
	client.Resource
	Operations []replica.PrepareRemoveAction `json:"operations"`
}

// ReplicaMode ...
type ReplicaMode struct {
	client.Resource
	Mode string `json:"mode"`
}

// Checkpoint ...
type Checkpoint struct {
	client.Resource
	SnapshotName string `json:"snapshotName"`
}

type RevisionCounter struct {
	client.Resource
	Counter string `json:"counter"`
}

type ReplicaCounter struct {
	client.Resource
	Counter int64 `json:"counter"`
}

type Action struct {
	client.Resource
	Value string `json:"Action"`
}

func NewReplica(context *api.ApiContext, state replica.State, info replica.Info, rep *replica.Replica) *Replica {
	r := &Replica{
		Resource: client.Resource{
			Type:    "replica",
			Id:      "1",
			Actions: map[string]string{},
		},
	}

	r.State = string(state)

	actions := map[string]bool{}

	// TODO Remove Invalid operations based on states
	switch state {
	case replica.Initial:
		actions["start"] = true
		actions["create"] = true
		actions["resize"] = true
		actions["updatecloneinfo"] = true
	case replica.Open:
		actions["start"] = true
		actions["resize"] = true
		actions["close"] = true
		actions["updatediskmode"] = true
		actions["resize"] = true
		actions["setrebuilding"] = true
		actions["setlogging"] = true
		actions["snapshot"] = true
		actions["reload"] = true
		actions["removedisk"] = true
		actions["replacedisk"] = true
		actions["revert"] = true
		actions["prepareremovedisk"] = true
		actions["setreplicamode"] = true
		actions["setrevisioncounter"] = true
		actions["updatecloneinfo"] = true
		actions["setreplicacounter"] = true
		actions["setcheckpoint"] = true
	case replica.Closed:
		actions["start"] = true
		actions["open"] = true
		actions["resize"] = true
		actions["removedisk"] = true
		actions["replacedisk"] = true
		actions["revert"] = true
		actions["updatecloneinfo"] = true
		actions["prepareremovedisk"] = true
		actions["setreplicacounter"] = true
	case replica.Dirty:
		actions["start"] = true
		actions["resize"] = true
		actions["setrebuilding"] = true
		actions["setlogging"] = true
		actions["close"] = true
		actions["snapshot"] = true
		actions["reload"] = true
		actions["removedisk"] = true
		actions["replacedisk"] = true
		actions["updatediskmode"] = true
		actions["revert"] = true
		actions["setreplicamode"] = true
		actions["prepareremovedisk"] = true
		actions["setreplicacounter"] = true
		actions["updatecloneinfo"] = true
		actions["setcheckpoint"] = true
	case replica.Rebuilding:
		actions["setrebuilding"] = true
		actions["setlogging"] = true
		actions["close"] = true
		actions["reload"] = true
		actions["setreplicamode"] = true
		actions["setrevisioncounter"] = true
		actions["setreplicacounter"] = true
		actions["updatecloneinfo"] = true
		actions["setcheckpoint"] = true
	case replica.Error:
	}

	for action := range actions {
		r.Actions[action] = context.UrlBuilder.ActionLink(r.Resource, action)
	}

	r.Dirty = info.Dirty
	r.Rebuilding = info.Rebuilding
	r.Head = info.Head
	r.Parent = info.Parent
	r.SectorSize = info.SectorSize
	r.Checkpoint = info.Checkpoint
	r.Size = strconv.FormatInt(info.Size, 10)
	r.RevisionCounter = strconv.FormatInt(info.RevisionCounter, 10)
	r.UsedBlocks, r.UsedLogicalBlocks = "0", "0" // replica must be initializing
	if rep != nil {
		r.Chain, _ = rep.DisplayChain()
		r.Disks = rep.ListDisks()
		r.RemainSnapshots = rep.GetRemainSnapshotCounts()
		r.RevisionCounter = strconv.FormatInt(rep.GetRevisionCounter(), 10)
		r.ReplicaMode = rep.GetReplicaMode()
		r.CloneStatus = rep.GetCloneStatus()
		r.UsedBlocks = rep.GetUsedBlocks()
		r.UsedLogicalBlocks = rep.GetUsedLogicalBlocks()
	}
	return r
}

func setReplicaResourceActions(replica *client.Schema) {
	replica.ResourceActions = map[string]client.Action{
		"close": {
			Output: "replica",
		},
		"open": {
			Output: "replica",
		},
		"reload": {
			Output: "replica",
		},
		"snapshot": {
			Input:  "snapshotInput",
			Output: "replica",
		},
		"removedisk": {
			Input:  "removediskInput",
			Output: "replica",
		},
		"setrebuilding": {
			Input:  "rebuildingInput",
			Output: "replica",
		},
		"setlogging": {
			Input:  "loggingInput",
			Output: "replica",
		},
		"create": {
			Input:  "createInput",
			Output: "replica",
		},
		"revert": {
			Input:  "revertInput",
			Output: "replica",
		},
		"prepareremovedisk": {
			Input:  "prepareRemoveDiskInput",
			Output: "prepareRemoveDiskOutput",
		},
		"setreplicamode": {
			Input: "replicaMode",
		},
		"setrevisioncounter": {
			Input: "revisionCounter",
		},
		"setreplicacounter": {
			Input: "replicaCounter",
		},
		"replacedisk": {
			Input:  "replacediskinput",
			Output: "replica",
		},
	}
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("error", client.ServerApiError{})
	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("createInput", CreateInput{})
	schemas.AddType("rebuildingInput", RebuildingInput{})
	schemas.AddType("LoggerInput", LoggingInput{})
	schemas.AddType("snapshotInput", SnapshotInput{})
	schemas.AddType("removediskInput", RemoveDiskInput{})
	schemas.AddType("revertInput", RevertInput{})
	schemas.AddType("prepareRemoveDiskInput", PrepareRemoveDiskInput{})
	schemas.AddType("prepareRemoveDiskOutput", PrepareRemoveDiskOutput{})
	schemas.AddType("replicaMode", ReplicaMode{})
	schemas.AddType("revisionCounter", RevisionCounter{})
	schemas.AddType("replicaCounter", ReplicaCounter{})
	schemas.AddType("replacediskInput", ReplaceDiskInput{})

	rebuild := schemas.AddType("rebuildinfo", RebuildInfoOutput{})
	rebuild.PluralName = ""
	rebuild.ResourceMethods = []string{"GET"}
	delete := schemas.AddType("delete", DeleteReplicaOutput{})
	delete.ResourceMethods = []string{"DELETE"}

	replica := schemas.AddType("replica", Replica{})

	replica.ResourceMethods = []string{"GET", "DELETE"}
	setReplicaResourceActions(replica)

	return schemas
}

type Server struct {
	s *replica.Server
}

func NewServer(s *replica.Server) *Server {
	return &Server{
		s: s,
	}
}
