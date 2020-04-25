package rebuild

import (
	"fmt"
	"path"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
)

var (
	// Info is the global variable used to
	// hold information about sync progress
	Info *types.SyncInfo
)

// SetSyncInfo initializes Info structure to hold
// rebuild info. It is queried via REST API call.
func SetSyncInfo(syncInfo *types.SyncInfo) {
	Info = syncInfo
}

// SetStatus is the status of rebuilding for a given disk
func SetStatus(disk, status string) {
	Info.Snapshots[disk].Status = status
}

// GetRebuildInfo returns the updated SyncInfo such as total
// used size of snapshots and size of individual snapshots.
func GetRebuildInfo() *types.SyncInfo {
	if Info == nil {
		return nil
	}
	var totSize int64
	for snap := range Info.Snapshots {
		size := util.GetFileActualSize(path.Join(replica.Dir, snap))
		if size == -1 {
			Info.Snapshots[snap].WOSize = "NA"
		} else {
			totSize += size
			Info.Snapshots[snap].WOSize = fmt.Sprint(size)
		}
	}
	Info.WOSnapshotsTotalSize = fmt.Sprint(totSize)
	return Info
}
