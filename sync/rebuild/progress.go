package rebuild

import (
	"fmt"
	"path"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
)

var (
	Info *types.SyncInfo
)

func AddSyncInfo(syncInfo *types.SyncInfo) {
	Info = syncInfo
}

func SetStatus(disk, status string) {
	Info.Snapshots[disk].Status = status
}

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
