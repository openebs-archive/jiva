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
	io.Closer
	Snapshot(name string, userCreated bool, created string) error
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

type Replica struct {
	Address string `json:"Address"`
	Mode    Mode   `json:"Mode"`
}

type RegReplica struct {
	Address  string
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
	RevisionCounter int64
	ReplicaCounter  int64
	SCSIIOCount     map[int]int64

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

type DataProcessor interface {
	IOs
	PingResponse() error
	//Update() error
}
