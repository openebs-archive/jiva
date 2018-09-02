package rest

import (
	"strconv"

	"github.com/openebs/jiva/replica"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

type Replica struct {
	client.Resource
	Dirty             bool                        `json:"dirty"`
	Rebuilding        bool                        `json:"rebuilding"`
	Head              string                      `json:"head"`
	Parent            string                      `json:"parent"`
	Size              string                      `json:"size"`
	SectorSize        int64                       `json:"sectorSize"`
	State             string                      `json:"state"`
	Chain             []string                    `json:"chain"`
	Disks             map[string]replica.DiskInfo `json:"disks"`
	RemainSnapshots   int                         `json:"remainsnapshots"`
	RevisionCounter   string                      `json:"revisioncounter"`
	ReplicaCounter    int64                       `json:"replicacounter"`
	UsedLogicalBlocks string                      `json:"usedlogicalblocks"`
	UsedBlocks        string                      `json:"usedblocks"`
	CloneStatus       string                      `json:"clonestatus"`
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

type SnapshotInput struct {
	client.Resource
	Name        string `json:"name"`
	UserCreated bool   `json:"usercreated"`
	Created     string `json:"created"`
}

type CloneUpdateInput struct {
	client.Resource
	SnapName string `json:"snapname"`
}

type RemoveDiskInput struct {
	client.Resource
	Name string `json:"name"`
}

type VolUsage struct {
	client.Resource
	UsedLogicalBlocks string `json:"usedlogicalblocks"`
	UsedBlocks        string `json:"usedblocks"`
	SectorSize        string `json:"sectorSize"`
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
		actions["resize"] = true
		actions["setrebuilding"] = true
		actions["snapshot"] = true
		actions["reload"] = true
		actions["removedisk"] = true
		actions["replacedisk"] = true
		actions["revert"] = true
		actions["prepareremovedisk"] = true
		actions["setrevisioncounter"] = true
		actions["updatecloneinfo"] = true
		actions["setreplicacounter"] = true
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
		actions["close"] = true
		actions["snapshot"] = true
		actions["reload"] = true
		actions["removedisk"] = true
		actions["replacedisk"] = true
		actions["revert"] = true
		actions["prepareremovedisk"] = true
		actions["setreplicacounter"] = true
		actions["updatecloneinfo"] = true
	case replica.Rebuilding:
		actions["start"] = true
		actions["resize"] = true
		actions["snapshot"] = true
		actions["setrebuilding"] = true
		actions["close"] = true
		actions["reload"] = true
		actions["setrevisioncounter"] = true
		actions["setreplicacounter"] = true
		actions["updatecloneinfo"] = true
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
	r.Size = strconv.FormatInt(info.Size, 10)

	if rep != nil {
		r.Chain, _ = rep.DisplayChain()
		r.Disks = rep.ListDisks()
		r.RemainSnapshots = rep.GetRemainSnapshotCounts()
		r.RevisionCounter = strconv.FormatInt(rep.GetRevisionCounter(), 10)
		r.CloneStatus = rep.GetCloneStatus()
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
	schemas.AddType("snapshotInput", SnapshotInput{})
	schemas.AddType("removediskInput", RemoveDiskInput{})
	schemas.AddType("revertInput", RevertInput{})
	schemas.AddType("prepareRemoveDiskInput", PrepareRemoveDiskInput{})
	schemas.AddType("prepareRemoveDiskOutput", PrepareRemoveDiskOutput{})
	schemas.AddType("revisionCounter", RevisionCounter{})
	schemas.AddType("replicaCounter", ReplicaCounter{})
	schemas.AddType("replacediskInput", ReplaceDiskInput{})

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
