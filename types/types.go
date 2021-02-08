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

package types

import (
	"io"
	"time"
)

const (
	WO     = Mode("WO")
	RW     = Mode("RW")
	ERR    = Mode("ERR")
	INIT   = Mode("INIT")
	CLOSED = Mode("CLOSED")

	StateUp   = State("Up")
	StateDown = State("Down")

	RebuildPending           = "Pending"
	RebuildInProgress        = "InProgress"
	RebuildCompleted         = "Completed"
	SyncHTTPClientTimeoutKey = "SYNC_HTTP_CLIENT_TIMEOUT"
)

var (
	// RPCReadTimeout ...
	RPCReadTimeout time.Duration
	// RPCWriteTimeout ...
	RPCWriteTimeout time.Duration
)

const (
	// DrainStart flag is used to notify CreateHoles goroutine
	// for draining the data in HoleCreatorChan
	DrainStart HoleChannelOps = iota + 1
	// DrainDone flag is used to notify the s.Close that data
	// from HoleCreatorChan has been flushed
	DrainDone
)

type ReaderWriterAt interface {
	io.ReaderAt
	io.WriterAt
}

type IOs interface {
	ReaderWriterAt
	io.Closer
	Sync() (int, error)
	Unmap(int64, int64) (int, error)
}

type DiffDisk interface {
	ReaderWriterAt
	io.Closer
	Fd() uintptr
}

type MonitorChannel chan error

// HoleChannelOps is the operation that has to be performed
// on HoleCreatorChan such as DrainStart, DrainDone for draining
// the chan and notify the caller if drain is done.
type HoleChannelOps int

// DrainOps is flag used for various operations on HoleCreatorChan
var DrainOps HoleChannelOps
var MaxChainLength int

var (
	// ShouldPunchHoles is a flag used to verify if
	// we should punch holes.
	ShouldPunchHoles bool
)

type Backend interface {
	IOs
	Snapshot(name string, userCreated bool, created string) error
	GetReplicaChain() ([]string, error)
	SetCheckpoint(snapshotName string) error
	Resize(name string, size string) error
	Size() (int64, error)
	SectorSize() (int64, error)
	RemainSnapshots() (int, error)
	GetRevisionCounter() (int64, error)
	GetCloneStatus() (string, error)
	GetVolUsage() (VolUsage, error)
	SetReplicaMode(mode Mode) error
	SetRevisionCounter(counter int64) error
	SetRebuilding(rebuilding bool) error
	GetMonitorChannel() MonitorChannel
	StopMonitoring()
}

type BackendFactory interface {
	Create(address string) (Backend, error)
	SignalToAdd(string, string) error
	VerifyReplicaAlive(string) bool
}

type VolUsage struct {
	RevisionCounter   int64
	UsedLogicalBlocks int64
	UsedBlocks        int64
	SectorSize        int64
}

type Controller interface {
	AddReplica(address string) error
	RemoveReplica(address string) error
	SetReplicaMode(address string, mode Mode) error
	ListReplicas() []Replica
	Start(address ...string) error
	Shutdown() error
}

type Server interface {
	ReaderWriterAt
	Controller
}

type Mode string

type State string

type DiskInfo struct {
	Name            string   `json:"name"`
	Parent          string   `json:"parent"`
	Children        []string `json:"children"`
	Removed         bool     `json:"removed"`
	UserCreated     bool     `json:"usercreated"`
	Created         string   `json:"created"`
	Size            string   `json:"size"`
	RevisionCounter int64    `json:"revisionCount"`
}

// Snapshot holds the information of snapshot size of RW and WO
// replicas and status of rebuild progress.
type Snapshot struct {
	Name   string `json:"name,omitempty"`
	RWSize string `json:"rwsize,omitempty"`
	WOSize string `json:"wosize,omitempty"`
	Status string `json:"status,omitempty"`
}

// SyncInfo holds the information of snapshots and its progress
// during rebuilding.
type SyncInfo struct {
	// Snapshots holds the map of snapshot names and their details
	Snapshots []Snapshot `json:"snapshots,omitempty"`
	RWReplica string     `json:"rwreplica,omitempty"`
	WOReplica string     `json:"woreplica,omitempty"`
	// RWSnapshotsTotalSize holds the total size consumed by the snapshots
	// in RW replica
	RWSnapshotsTotalSize string `json:"rwreplicatotalsize,omitempty"`
	// WOSnapshotsTotalSize holds the total size consumed by the snapshots
	// in WO replica
	WOSnapshotsTotalSize string `json:"woreplicatotalsize,omitempty"`
}

type ReplicaInfo struct {
	Dirty             bool                `json:"dirty"`
	Rebuilding        bool                `json:"rebuilding"`
	Head              string              `json:"head"`
	Parent            string              `json:"parent"`
	Size              string              `json:"size"`
	SectorSize        int64               `json:"sectorSize"`
	State             string              `json:"state"`
	Chain             []string            `json:"chain"`
	Disks             map[string]DiskInfo `json:"disks"`
	RemainSnapshots   int                 `json:"remainsnapshots"`
	ReplicaMode       string              `json:"replicamode"`
	RevisionCounter   string              `json:"revisioncounter"`
	UsedLogicalBlocks string              `json:"usedlogicalblocks"`
	UsedBlocks        string              `json:"usedblocks"`
	CloneStatus       string              `json:"clonestatus"`
	Checkpoint        string              `json:"checkpoint"`
}

type Replica struct {
	Address string `json:"Address"`
	Mode    Mode   `json:"Mode"`
}

type RegReplica struct {
	Address  string
	UUID     string
	UpTime   time.Duration
	RevCount int64
	RepType  string
	RepState string
}

type IOStats struct {
	IOPS        int64
	Throughput  int64
	Latency     float32
	AvBlockSize float32
}

type Stats struct {
	IsClientConnected bool
	RevisionCounter   int64
	ReplicaCounter    int64
	SCSIIOCount       map[int]int64

	ReadIOPS            int64
	TotalReadTime       int64
	TotalReadBlockCount int64

	WriteIOPS            int64
	TotalWriteTime       int64
	TotalWriteBlockCount int64

	UsedLogicalBlocks int64
	UsedBlocks        int64
	SectorSize        int64
}

type Interface interface{}

type PeerDetails struct {
	ReplicaCount       int
	QuorumReplicaCount int
}

type Frontend interface {
	Startup(name string, frontendIP string, clusterIP string, size, sectorSize int64, rw IOs) error
	Shutdown() error
	State() State
	Stats() Stats
	Resize(uint64) error
}

// Target ...
type Target struct {
	ClusterIP  string
	FrontendIP string
}

type DataProcessor interface {
	IOs
	PingResponse() error
	//Update() error
}
