/*
 Copyright © 2020 The OpenEBS Authors

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

package rebuild

import (
	"path"

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
	for i, snap := range Info.Snapshots {
		if snap.Name == disk {
			Info.Snapshots[i].Status = status
		}
	}
}

// GetRebuildInfo returns the updated SyncInfo such as total
// used size of snapshots and size of individual snapshots.
func GetRebuildInfo(dir string) *types.SyncInfo {
	if Info == nil {
		return nil
	}
	var totSize int64
	for i, snap := range Info.Snapshots {
		size := util.GetFileActualSize(path.Join(dir, snap.Name))
		if size == -1 {
			Info.Snapshots[i].WOSize = "NA"
		} else {
			totSize += size
			Info.Snapshots[i].WOSize = util.ConvertHumanReadable(size)
		}
	}
	if totSize != 0 {
		Info.WOSnapshotsTotalSize = util.ConvertHumanReadable(totSize)
	} else {
		Info.WOSnapshotsTotalSize = "NA"
	}
	return Info
}
