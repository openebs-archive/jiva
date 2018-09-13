package types

import (
	"io"
	"time"
)

const (
	WO  = Mode("WO")
	RW  = Mode("RW")
	ERR = Mode("ERR")

	StateUp   = State("Up")
	StateDown = State("Down")
)

type ReaderWriterAt interface {
	io.ReaderAt
	io.WriterAt
}

type DiffDisk interface {
	ReaderWriterAt
	io.Closer
	Fd() uintptr
}

type MonitorChannel chan error

type Backend interface {
	ReaderWriterAt
	io.Closer
	Snapshot(name string, userCreated bool, created string) error
	Resize(name string, size string) error
	Size() (int64, error)
	SectorSize() (int64, error)
	RemainSnapshots() (int, error)
	GetRevisionCounter() (int64, error)
	GetCloneStatus() (string, error)
	GetVolUsage() (VolUsage, error)
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
	Address string
	Mode    Mode
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
	Startup(name string, frontendIP string, clusterIP string, size, sectorSize int64, rw ReaderWriterAt) error
	Shutdown() error
	State() State
	Stats() Stats
	Resize(uint64) error
}

type DataProcessor interface {
	ReaderWriterAt
	PingResponse() error
	//Update() error
}
