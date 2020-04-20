package replica

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	units "github.com/docker/go-units"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/openebs/sparse-tools/sparse"
	"github.com/sirupsen/logrus"
)

const (
	metadataSuffix     = ".meta"
	imgSuffix          = ".img"
	volumeMetaData     = "volume.meta"
	defaultSectorSize  = 4096
	headPrefix         = "volume-head-"
	headSuffix         = ".img"
	headName           = headPrefix + "%03d" + headSuffix
	diskPrefix         = "volume-snap-"
	diskSuffix         = ".img"
	diskName           = diskPrefix + "%s" + diskSuffix
	maximumChainLength = 512
)

var (
	diskPattern = regexp.MustCompile(`volume-head-(\d)+.img`)
)

type Replica struct {
	sync.RWMutex
	volume           diffDisk
	dir              string
	ReplicaStartTime time.Time
	ReplicaType      string
	info             Info
	// diskData is mapping of disk (i., Head or snapshot files)
	// with their parent, name and other info.
	// For exp: H->S3->S2->S1->S0
	// diskData[S3] = {Name: S3, Parent: S2}
	diskData map[string]*disk
	// diskChildrenMap is mapping of disks with the respective
	// childrens if exists.
	diskChildrenMap map[string]map[string]bool
	// list of active snapshots and head at the last index
	// len(activeDiskData) - 2 as latest snapshot.
	activeDiskData []*disk
	readOnly       bool
	mode           types.Mode

	revisionLock  sync.Mutex
	revisionCache int64
	revisionFile  *sparse.DirectFileIoProcessor

	peerLock  sync.Mutex
	peerCache types.PeerDetails
	peerFile  *sparse.DirectFileIoProcessor

	cloneStatus   string
	CloneSnapName string
	Clone         bool
	// used for draining the HoleCreatorChan also useful for mocking
	holeDrainer func()
}

type Info struct {
	Size            int64
	Head            string
	Dirty           bool
	Rebuilding      bool
	Parent          string
	SectorSize      int64
	BackingFileName string
	CloneStatus     string
	BackingFile     *BackingFile `json:"-"`
	RevisionCounter int64
}

type disk struct {
	Name            string
	Parent          string
	Removed         bool
	UserCreated     bool
	Created         string
	RevisionCounter int64
}

type BackingFile struct {
	Size       int64
	SectorSize int64
	Name       string
	Disk       types.DiffDisk
}

type PrepareRemoveAction struct {
	Action string `json:"action"`
	Source string `json:"source"`
	Target string `json:"target"`
}

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

const (
	OpCoalesce = "coalesce" // Source is parent, target is child
	OpRemove   = "remove"
	OpReplace  = "replace"
)

func CreateTempReplica() (*Replica, error) {
	if err := os.Mkdir(Dir, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	r := &Replica{
		dir:              Dir,
		ReplicaStartTime: StartTime,
		mode:             types.INIT,
	}
	if err := r.initRevisionCounter(); err != nil {
		logrus.Errorf("Error in initializing revision counter while creating temp replica")
		return nil, err
	}
	return r, nil
}

func CreateTempServer() (*Server, error) {
	return &Server{
		dir: Dir,
	}, nil
}

func ReadInfo(dir string) (Info, error) {
	var info Info
	err := (&Replica{dir: dir}).unmarshalFile(volumeMetaData, &info)
	return info, err
}

func New(size, sectorSize int64, dir string, backingFile *BackingFile, replicaType string) (*Replica, error) {
	return construct(false, size, sectorSize, dir, "", backingFile, replicaType)
}

func NewReadOnly(dir, head string, backingFile *BackingFile) (*Replica, error) {
	// size and sectorSize don't matter because they will be read from metadata
	return construct(true, 0, 512, dir, head, backingFile, "")
}

func construct(readonly bool, size, sectorSize int64, dir, head string, backingFile *BackingFile, replicaType string) (*Replica, error) {
	if size%sectorSize != 0 {
		return nil, fmt.Errorf("Size %d not a multiple of sector size %d", size, sectorSize)
	}

	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		logrus.Errorf("failed to create directory: %s", dir)
		return nil, err
	}

	r := &Replica{
		dir:             dir,
		activeDiskData:  make([]*disk, 1),
		diskData:        make(map[string]*disk),
		diskChildrenMap: map[string]map[string]bool{},
		mode:            types.INIT,
		holeDrainer: func() {
			// this is just initializing function,
			// actual excution will be done by r.holeDrainer()
			holeDrainer()
		},
	}
	r.info.Size = size
	r.info.SectorSize = sectorSize
	r.info.BackingFile = backingFile
	if backingFile != nil {
		r.info.BackingFileName = backingFile.Name
	}
	r.volume.sectorSize = defaultSectorSize

	if err := r.initRevisionCounter(); err != nil {
		return nil, err
	}

	// Scan all the disks to build the disk map
	exists, err := r.readMetadata()
	if err != nil {
		return nil, err
	}
	// Reference r.info.Size because it may have changed from reading
	// metadata
	locationSize := r.info.Size / r.volume.sectorSize
	if size%defaultSectorSize != 0 {
		locationSize++
	}
	r.volume.location = make([]uint16, locationSize)
	r.volume.files = []types.DiffDisk{nil}
	r.volume.UserCreatedSnap = []bool{false}
	r.volume.rmLock = &sync.Mutex{}

	if r.readOnly && !exists {
		return nil, os.ErrNotExist
	}

	if head != "" {
		r.info.Head = head
	}

	if exists {
		if err := r.openLiveChain(); err != nil {
			return nil, err
		}
	} else if size <= 0 {
		return nil, os.ErrNotExist
	} else {
		if err := r.createDisk("000", false, util.Now()); err != nil {
			return nil, err
		}
	}
	r.info.Parent = r.diskData[r.info.Head].Parent

	r.insertBackingFile()
	r.ReplicaType = replicaType
	return r, r.writeVolumeMetaData(true, r.info.Rebuilding)
}

func GenerateSnapshotDiskName(name string) string {
	return fmt.Sprintf(diskName, name)
}

func GetSnapshotNameFromDiskName(diskName string) (string, error) {
	if !strings.HasPrefix(diskName, diskPrefix) || !strings.HasSuffix(diskName, diskSuffix) {
		return "", fmt.Errorf("Invalid snapshot disk name %v", diskName)
	}
	result := strings.TrimPrefix(diskName, diskPrefix)
	result = strings.TrimSuffix(result, diskSuffix)
	return result, nil
}

func IsHeadDisk(diskName string) bool {
	if strings.HasPrefix(diskName, headPrefix) && strings.HasSuffix(diskName, headSuffix) {
		return true
	}
	return false
}

func (r *Replica) diskPath(name string) string {
	return path.Join(r.dir, name)
}

func (r *Replica) insertBackingFile() {
	if r.info.BackingFile == nil {
		return
	}

	d := disk{Name: r.info.BackingFile.Name}
	r.activeDiskData = append([]*disk{{}, &d}, r.activeDiskData[1:]...)
	r.volume.files = append([]types.DiffDisk{nil, r.info.BackingFile.Disk}, r.volume.files[1:]...)
	r.volume.UserCreatedSnap = append([]bool{false, false}, r.volume.UserCreatedSnap[1:]...)
	r.diskData[d.Name] = &d
}

func (r *Replica) SetRebuilding(rebuilding bool) error {
	err := r.writeVolumeMetaData(true, rebuilding)
	if err != nil {
		return err
	}
	r.info.Rebuilding = rebuilding
	return nil
}

func (r *Replica) GetUsage() (*types.VolUsage, error) {
	return &types.VolUsage{
		RevisionCounter:   r.revisionCache,
		UsedLogicalBlocks: r.volume.UsedLogicalBlocks,
		UsedBlocks:        r.volume.UsedBlocks,
		SectorSize:        r.volume.sectorSize,
	}, nil
}

func (r *Replica) Resize(obj interface{}) error {
	var sizeInBytes int64
	chain, err := r.Chain()
	if err != nil {
		return err
	}
	switch obj.(type) {
	case string:
		if obj != "" {
			sizeInBytes, err = units.RAMInBytes(obj.(string))
			if err != nil {
				return err
			}
		}
	case int64:
		if obj != 0 {
			sizeInBytes = obj.(int64)
		} else {
			return nil
		}
	}
	r.Lock()
	defer r.Unlock()
	if r.info.Size > sizeInBytes {
		return fmt.Errorf("Previous size %d is greater than %d", r.info.Size, sizeInBytes)
	}
	for _, file := range chain {
		if err := syscall.Truncate(r.diskPath(file), sizeInBytes); err != nil {
			return err
		}
	}
	byteArray := make([]uint16, (sizeInBytes-r.info.Size)/4096)
	r.volume.location = append(r.volume.location, byteArray...)
	r.info.Size = sizeInBytes
	return r.encodeToFile(&r.info, volumeMetaData)
}

// Reload reopens the files and recreates the file chain after syncing
func (r *Replica) Reload() (*Replica, error) {
	newReplica, err := New(r.info.Size, r.info.SectorSize, r.dir, r.info.BackingFile, r.ReplicaType)
	if err != nil {
		return nil, err
	}
	newReplica.mode = r.mode
	newReplica.info.Dirty = r.info.Dirty
	return newReplica, nil
}

// UpdateCloneInfo update the clone information such as snapshot name and
// revisionCount
func (r *Replica) UpdateCloneInfo(snapName, revCount string) error {
	r.info.Parent = "volume-snap-" + snapName + ".img"
	if err := r.encodeToFile(&r.info, volumeMetaData); err != nil {
		return err
	}

	revisionCount, err := strconv.ParseInt(revCount, 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse revision count %v, err: %v", revCount, err)
	}

	if err := r.SetRevisionCounterCloneReplica(revisionCount); err != nil {
		return fmt.Errorf("Failed to set revision counter, err: %v", err)
	}

	r.diskData[r.info.Head].Parent = r.info.Parent
	r.diskData[r.info.Head].RevisionCounter = revisionCount
	return r.encodeToFile(r.diskData[r.info.Head], r.info.Head+metadataSuffix)
}

func (r *Replica) findDisk(name string) int {
	for i, d := range r.activeDiskData {
		if i == 0 {
			continue
		}
		if d.Name == name {
			return i
		}
	}
	return 0
}

func (r *Replica) RemoveDiffDisk(name string) error {
	r.Lock()
	defer r.Unlock()

	if r.mode != types.RW {
		return fmt.Errorf("Can not delete snapshot, replica mode: %v", r.mode)
	}
	if name == r.info.Head {
		return fmt.Errorf("Can not delete the active differencing disk")
	}

	if r.info.Parent == name {
		return fmt.Errorf("Can't delete latest snapshot: %s", name)
	}

	// Empty the data in HoleCreatorChan send for punching
	// holes (fallocate), since it may be punching holes in
	// the file that is going to be deleted.
	r.holeDrainer()

	if err := r.removeDiskNode(name); err != nil {
		return err
	}

	return r.rmDisk(name)
}

func (r *Replica) hardlinkDisk(target, source string) error {
	if _, err := os.Stat(r.diskPath(source)); err != nil {
		return fmt.Errorf("Cannot find source of replacing: %v", source)
	}

	if _, err := os.Stat(r.diskPath(target)); err == nil {
		logrus.Infof("Old file %s exists, deleting", target)
		if err := os.Remove(r.diskPath(target)); err != nil {
			return fmt.Errorf("Fail to remove %s: %v", target, err)
		}
	}

	if err := os.Link(r.diskPath(source), r.diskPath(target)); err != nil {
		return fmt.Errorf("Fail to link %s to %s", source, target)
	}
	return r.syncDir()
}

// ReplaceDisk replace the source with target snapshot
// and remove and close both source and target and open
// new instance of file for R/W and update the file index
// with new reference.
func (r *Replica) ReplaceDisk(target, source string) error {
	r.Lock()
	defer r.Unlock()

	if r.mode != types.RW {
		return fmt.Errorf("Can not delete snapshot, replica mode: %v", r.mode)
	}

	if target == r.info.Head {
		return fmt.Errorf("Can not replace the active differencing disk")
	}

	// Empty the data in HoleCreatorChan send for punching
	// holes (fallocate), since it may be punching holes in
	// the file that is going to be deleted.
	r.holeDrainer()

	if err := r.hardlinkDisk(target, source); err != nil {
		return err
	}

	if err := r.removeDiskNode(source); err != nil {
		return err
	}

	if err := r.rmDisk(source); err != nil {
		logrus.Fatalf("Failed to remove disk: %v, err: %v", source, err)
		return err
	}

	// find the updated index of target
	index := r.findDisk(target)
	// This case is valid if revert has happened
	// For exp: H->S3->S2->S1->S0, and revert to S1
	// so resulting chain will be NH->S1->S0 and after
	// reload S3 will no longer will be in chain
	if index <= 0 {
		return nil
	}

	// Close the removed file
	if err := r.volume.files[index].Close(); err != nil {
		logrus.Fatalf("Failed to close old instance of target: %v, err: %v", target, err)
		return err
	}

	// Open for R/W
	newFile, err := r.OpenFile(r.activeDiskData[index].Name, 0)
	if err != nil {
		logrus.Fatalf("Failed to open new instance of target: %v, err: %v", target, err)
		return err
	}

	// update index with the newFile
	r.volume.files[index] = newFile
	logrus.Infof("Done replacing %v with %v", target, source)

	return nil
}

func (r *Replica) removeDiskNode(name string) error {
	// If snapshot has no child, then we can safely delete it
	// And it's definitely not in the live chain
	children, exists := r.diskChildrenMap[name]
	if !exists {
		r.updateChildDisk(name, "")
		delete(r.diskData, name)
		return nil
	}

	// If snapshot has more than one child, we cannot really delete it
	if len(children) > 1 {
		return fmt.Errorf("Cannot remove snapshot %v with %v children",
			name, len(children))
	}

	// only one child from here
	var child string
	for child = range children {
	}

	r.updateChildDisk(name, child)
	if err := r.updateParentDisk(child, name); err != nil {
		logrus.Fatalf("Failed to update parent disk: %v with child: %v", name, child)
		return err
	}
	delete(r.diskData, name)

	index := r.findDisk(name)
	// This case is valid if revert has happened
	// For exp: H->S3->S2->S1->S0, and revert to S1
	// so resulting chain will be NH->S1->S0 and after
	// reload S3 will no longer will be in chain
	if index <= 0 {
		return nil
	}
	if err := r.volume.RemoveIndex(index); err != nil {
		logrus.Fatalf("Failed to remove index for disk: %v, err: %v", name, err)
		return err
	}

	if len(r.activeDiskData)-2 == index {
		r.info.Parent = r.diskData[r.info.Head].Parent
	}

	r.activeDiskData = append(r.activeDiskData[:index], r.activeDiskData[index+1:]...)
	return nil
}

// PrepareRemoveDisk mark and prepare the list of the disks that
// is going to be deleted.
// NOTE: We don't delete latest snapshot because the data
// needs to be merged into Head file where IO's are being
// precessed that means we need to block IO's for some
// time till this get precessed.
func (r *Replica) PrepareRemoveDisk(name string) ([]PrepareRemoveAction, error) {
	r.Lock()
	defer r.Unlock()

	if r.mode != types.RW {
		return nil, fmt.Errorf("Can not prepare remove disk, replica mode: %v", r.mode)
	}

	disk := name

	data, exists := r.diskData[disk]
	if !exists {
		disk = GenerateSnapshotDiskName(name)
		data, exists = r.diskData[disk]
		if !exists {
			return nil, fmt.Errorf("Can not find snapshot %v", disk)
		}
	}

	if disk == r.info.Head {
		return nil, fmt.Errorf("Can not delete the active differencing disk")
	}

	if r.info.Parent == disk {
		return nil, fmt.Errorf("Can't delete latest snapshot: %s", disk)
	}

	logrus.Infof("Mark disk %v as removed", disk)
	if err := r.markDiskAsRemoved(disk); err != nil {
		return nil, fmt.Errorf("Fail to mark disk %v as removed: %v", disk, err)
	}

	targetDisks := []string{}
	if data.Parent != "" {
		// check if metadata of parent exists for the snapshot
		// going to be deleted.
		_, exists := r.diskData[data.Parent]
		if !exists {
			return nil, fmt.Errorf("Can not find snapshot %v's parent %v", disk, data.Parent)
		}
	}

	targetDisks = append(targetDisks, disk)
	actions, err := r.processPrepareRemoveDisks(targetDisks)
	if err != nil {
		return nil, err
	}
	return actions, nil
}

func (r *Replica) processPrepareRemoveDisks(disks []string) ([]PrepareRemoveAction, error) {
	actions := []PrepareRemoveAction{}

	for _, disk := range disks {
		if _, exists := r.diskData[disk]; !exists {
			return nil, fmt.Errorf("Wrong disk %v doesn't exist", disk)
		}

		children := r.diskChildrenMap[disk]
		// 1) leaf node
		if children == nil {
			actions = append(actions, PrepareRemoveAction{
				Action: OpRemove,
				Source: disk,
			})
			continue
		}

		// 2) has only one child and is not head
		if len(children) == 1 {
			var child string
			// Get the only element in children
			for child = range children {
			}
			if child != r.info.Head {
				actions = append(actions,
					PrepareRemoveAction{
						Action: OpCoalesce,
						Source: disk,
						Target: child,
					},
					PrepareRemoveAction{
						Action: OpReplace,
						Source: disk,
						Target: child,
					})
				continue
			}
		}
	}

	return actions, nil
}

func (r *Replica) Info() Info {
	return r.info
}

func (r *Replica) DisplayChain() ([]string, error) {
	r.RLock()
	defer r.RUnlock()

	result := make([]string, 0, len(r.activeDiskData))

	cur := r.info.Head
	for cur != "" {
		_, ok := r.diskData[cur]
		if !ok {
			cur1 := r.info.Head
			for cur1 != "" {
				logrus.Errorf("cur1: %s", cur1)
				if _, ok1 := r.diskData[cur1]; !ok1 {
					break
				}
				cur1 = r.diskData[cur1].Parent
			}
			return nil, fmt.Errorf("Failed to find metadata for %s in DisplayChain", cur)
		}

		//		if !disk.Removed {
		result = append(result, cur)
		//		}
		cur = r.diskData[cur].Parent
	}

	return result, nil
}

//Chain returns the disk chain starting with Head(index=0),
//till the base snapshot
func (r *Replica) Chain() ([]string, error) {
	r.RLock()
	defer r.RUnlock()

	result := make([]string, 0, len(r.activeDiskData))

	cur := r.info.Head
	for cur != "" {
		result = append(result, cur)
		if _, ok := r.diskData[cur]; !ok {
			cur1 := r.info.Head
			for cur1 != "" {
				logrus.Errorf("cur1: %s", cur1)
				if _, ok1 := r.diskData[cur1]; !ok1 {
					break
				}
				cur1 = r.diskData[cur1].Parent
			}
			return nil, fmt.Errorf("Failed to find metadata for %s", cur)
		}
		cur = r.diskData[cur].Parent
	}

	return result, nil
}

func (r *Replica) writeVolumeMetaData(dirty, rebuilding bool) error {
	info := r.info
	info.Dirty = dirty
	info.Rebuilding = rebuilding
	return r.encodeToFile(&info, volumeMetaData)
}

func (r *Replica) isBackingFile(index int) bool {
	if r.info.BackingFile == nil {
		return false
	}
	return index == 1
}

func (r *Replica) closeAndSyncDir(f types.DiffDisk) error {
	err := f.Close()
	if err != nil {
		return err
	}
	if err = r.syncDir(); err != nil {
		return err
	}
	return nil
}

func (r *Replica) close() error {
	for i, f := range r.volume.files {
		if f != nil && !r.isBackingFile(i) {
			f.Close()
		}
	}

	return r.writeVolumeMetaData(false, false)
}

func (r *Replica) encodeToFile(obj interface{}, file string) error {
	if r.readOnly {
		return nil
	}

	f, err := os.Create(r.diskPath(file + ".tmp"))
	if err != nil {
		logrus.Errorf("Failed to create temp file: %s while encoding the data to file", file)
		return err
	}

	if lastErr := json.NewEncoder(f).Encode(&obj); err != nil {
		if err := f.Close(); err != nil {
			logrus.Errorf("Failed to close file: %v, err: %v", f.Name(), err)
		}
		logrus.Errorf("Failed to encode the data to file: %s", f.Name())
		return lastErr
	}

	if err := f.Close(); err != nil {
		logrus.Errorf("Failed to close file: %v after encoding", f.Name())
		return err
	}

	if err := os.Rename(r.diskPath(file+".tmp"), r.diskPath(file)); err != nil {
		return err
	}
	return r.syncDir()
}

func (r *Replica) nextFile(parsePattern *regexp.Regexp, pattern, parent string) (string, error) {
	if parent == "" {
		return fmt.Sprintf(pattern, 0), nil
	}

	matches := parsePattern.FindStringSubmatch(parent)
	if matches == nil {
		return "", fmt.Errorf("Invalid name %s does not match pattern: %v", parent, parsePattern)
	}

	index, _ := strconv.Atoi(matches[1])
	return fmt.Sprintf(pattern, index+1), nil
}

func (r *Replica) OpenFile(name string, flag int) (types.DiffDisk, error) {
	return sparse.NewDirectFileIoProcessor(r.diskPath(name), os.O_RDWR|flag, 06666, true)
}

// after creating or deleting the file the directory also needs to be synced
// in order to guarantee the file is visible across system crashes. See man
// page of fsync for more details.
func (r *Replica) syncDir() error {
	f, err := os.Open(r.dir)
	if err != nil {
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (r *Replica) createNewHead(oldHead, parent, created string) (types.DiffDisk, disk, error) {
	newHeadName, err := r.nextFile(diskPattern, headName, oldHead)
	if err != nil {
		return nil, disk{}, err
	}

	if _, err := os.Stat(r.diskPath(newHeadName)); err == nil {
		logrus.Warningf("Head file: %v already exists", newHeadName)
		if r.getDiskSize(newHeadName) > 0 {
			return nil, disk{}, fmt.Errorf("Can't remove head file %v as it contains some data", newHeadName)
		}
		if err = r.rmDisk(newHeadName); err != nil {
			return nil, disk{}, fmt.Errorf("Failed to remove disk, err: %v", err)
		}
	}

	f, err := r.OpenFile(newHeadName, os.O_TRUNC)
	if err != nil {
		return nil, disk{}, err
	}

	// file created before this needs to be deleted in case of error
	if err := syscall.Truncate(r.diskPath(newHeadName), r.info.Size); err != nil {
		return nil, disk{}, err
	}

	newDisk := disk{
		Parent:          parent,
		Name:            newHeadName,
		Removed:         false,
		UserCreated:     false,
		Created:         created,
		RevisionCounter: r.GetRevisionCounter(),
	}
	err = r.encodeToFile(&newDisk, newHeadName+metadataSuffix)
	return f, newDisk, err
}

func (r *Replica) linkDisk(oldname, newname string) error {
	if oldname == "" {
		return nil
	}

	dest := r.diskPath(newname)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("Old file :%v already exists", newname)
	}

	metaDest := r.diskPath(newname + metadataSuffix)
	if _, err := os.Stat(metaDest); err == nil {
		return fmt.Errorf("Old file :%v already exists", newname+metadataSuffix)
	}

	if err := os.Link(r.diskPath(oldname), dest); err != nil {
		return err
	}

	if err := os.Link(r.diskPath(oldname+metadataSuffix), r.diskPath(newname+metadataSuffix)); err != nil {
		return err
	}
	return r.syncDir()
}

func (r *Replica) markDiskAsRemoved(name string) error {
	disk, ok := r.diskData[name]
	if !ok {
		return fmt.Errorf("Cannot find disk %v", name)
	}
	if stat, err := os.Stat(r.diskPath(name)); err != nil || stat.IsDir() {
		return fmt.Errorf("Cannot find disk file %v", name)
	}
	if stat, err := os.Stat(r.diskPath(name + metadataSuffix)); err != nil || stat.IsDir() {
		return fmt.Errorf("Cannot find disk metafile %v", name+metadataSuffix)
	}
	disk.Removed = true
	r.diskData[name] = disk
	return r.encodeToFile(disk, name+metadataSuffix)
}

func (r *Replica) rmDisk(name string) error {
	if name == "" {
		return nil
	}

	if err := os.Remove(r.diskPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(r.diskPath(name + metadataSuffix)); err != nil && !os.IsNotExist(err) {
		return err
	}

	return r.syncDir()
}

func (r *Replica) revertDisk(parent, created string) (*Replica, error) {
	if _, err := os.Stat(r.diskPath(parent)); err != nil {
		return nil, err
	}

	oldHead := r.info.Head
	f, newHeadDisk, err := r.createNewHead(oldHead, parent, created)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := r.info
	info.Head = newHeadDisk.Name
	info.Dirty = true
	info.Parent = newHeadDisk.Parent

	if err := r.encodeToFile(&info, volumeMetaData); err != nil {
		r.encodeToFile(&r.info, volumeMetaData)
		return nil, err
	}

	// Need to execute before r.Reload() to update r.diskChildrenMap
	if err := r.rmDisk(oldHead); err != nil {
		return nil, err
	}

	rNew, err := r.Reload()
	if err != nil {
		return nil, err
	}
	if err = PreloadVolume(r); err != nil {
		return nil, fmt.Errorf("failed to load Lun map, error: %v", err)
	}
	types.ShouldPunchHoles = true
	return rNew, nil
}

func (r *Replica) createDisk(name string, userCreated bool, created string) error {
	if r.readOnly {
		return fmt.Errorf("Can not create disk on read-only replica")
	}

	maxChainLen := maximumChainLength
	if types.MaxChainLength != 0 {
		maxChainLen = types.MaxChainLength
	}

	if len(r.activeDiskData)+1 > maxChainLen {
		return fmt.Errorf("Too many active disks: %v", len(r.activeDiskData)+1)
	}

	done := false
	oldHead := r.info.Head
	newSnapName := GenerateSnapshotDiskName(name)

	if oldHead == "" {
		newSnapName = ""
	}

	// new head file created will be deleted in next call if replica crashes just after this
	f, newHeadDisk, err := r.createNewHead(oldHead, newSnapName, created)
	if err != nil {
		if rmDiskErr := r.rmDisk(newHeadDisk.Name); rmDiskErr != nil {
			logrus.Errorf("Failed to remove newHeadDisk: %v", rmDiskErr)
		}
		return err
	}

	defer func() {
		if !done {
			if err := r.rmDisk(newHeadDisk.Name); err != nil {
				logrus.Errorf("Failed to remove disk: %v in defer, err: %v", newHeadDisk.Name, err)
			}
			if err := r.rmDisk(newSnapName); err != nil {
				logrus.Errorf("Failed to remove disk: %v in defer, err: %v", newSnapName, err)
			}
			if err := f.Close(); err != nil {
				logrus.Errorf("Failed to close file: %v in defer, err: %v", newHeadDisk.Name, err)
			} // rm only unlink the file since fd is still open
			return
		}
		// crash at this point leads to stale old head and its meta file will remain
		// which will not be part of the chain as its already updated below.
		if err := r.rmDisk(oldHead); err != nil {
			logrus.Errorf("Failed to remove disk: %v in defer, err: %v", oldHead, err)
		}
	}()

	if err := r.linkDisk(r.info.Head, newSnapName); err != nil {
		return err
	}

	// crash at this point leads to stale snapshot files

	r.diskData[newHeadDisk.Name] = &newHeadDisk
	if newSnapName != "" {
		r.addChildDisk(newSnapName, newHeadDisk.Name)
		r.diskData[newSnapName] = r.diskData[oldHead]
		r.diskData[newSnapName].Name = newSnapName
		r.diskData[newSnapName].UserCreated = userCreated
		r.diskData[newSnapName].Created = created
		r.diskData[newSnapName].RevisionCounter = r.GetRevisionCounter()

		// create new metafile for snapshot
		if err := r.encodeToFile(r.diskData[newSnapName], newSnapName+metadataSuffix); err != nil {
			return err
		}
		// crash here will leave stale snapshot files
		size := int64(unsafe.Sizeof(r.diskData[newSnapName]))
		if size%defaultSectorSize == 0 {
			r.volume.UsedBlocks += size / defaultSectorSize
		} else {
			r.volume.UsedBlocks += (size/defaultSectorSize + 1)
		}

		r.updateChildDisk(oldHead, newSnapName)
		r.activeDiskData[len(r.activeDiskData)-1].Name = newSnapName
	}
	delete(r.diskData, oldHead)

	r.volume.files = append(r.volume.files, f)
	if userCreated {
		//Indx 0 is nil, indx 1 is base snapshot,
		//last indx (len(r.volume.files)-1) is active file
		r.volume.SnapIndx = len(r.volume.files) - 2
	}
	r.volume.UserCreatedSnap = append(r.volume.UserCreatedSnap, userCreated)
	r.activeDiskData = append(r.activeDiskData, &newHeadDisk)

	// panic only exits current goroutine, so any modification
	// to global data structure will be visible to other goroutines,
	// process also will not exit until all goroutine stops.
	// fatal exits entire process.
	info := r.info
	info.Head = newHeadDisk.Name
	info.Dirty = true
	info.Parent = newSnapName
	info.RevisionCounter = newHeadDisk.RevisionCounter
	// new head encoded to metadata
	if err := r.encodeToFile(&info, volumeMetaData); err != nil {
		return err
	}

	// crash at this point leads to stale old head and it meta file
	done = true
	// update in memory info as its persisted on the disk
	r.info = info
	return nil
}

func (r *Replica) addChildDisk(parent, child string) {
	children, exists := r.diskChildrenMap[parent]
	if !exists {
		children = map[string]bool{}
	}
	children[child] = true
	r.diskChildrenMap[parent] = children
}

func (r *Replica) rmChildDisk(parent, child string) {
	children, exists := r.diskChildrenMap[parent]
	if !exists {
		return
	}
	if _, exists := children[child]; !exists {
		return
	}
	delete(children, child)
	if len(children) == 0 {
		delete(r.diskChildrenMap, parent)
		return
	}
	r.diskChildrenMap[parent] = children
}

func (r *Replica) updateChildDisk(oldName, newName string) {
	parent := r.diskData[oldName].Parent
	r.rmChildDisk(parent, oldName)
	if newName != "" {
		r.addChildDisk(parent, newName)
	}
}

func (r *Replica) updateParentDisk(name, oldParent string) error {
	child := r.diskData[name]
	if oldParent != "" {
		child.Parent = r.diskData[oldParent].Parent
	} else {
		child.Parent = ""
	}
	r.diskData[name] = child
	return r.encodeToFile(child, child.Name+metadataSuffix)
}

func (r *Replica) openLiveChain() error {
	chain, err := r.Chain()
	if err != nil {
		return err
	}

	if len(chain) > maximumChainLength {
		return fmt.Errorf("Live chain is too long: %v", len(chain))
	}

	for i := len(chain) - 1; i >= 0; i-- {
		parent := chain[i]
		f, err := r.OpenFile(parent, 0)
		if err != nil {
			logrus.Error("failed to open live chain with existing parent: ", parent)
			return err
		}

		r.volume.files = append(r.volume.files, f)
		userCreated := r.diskData[parent].UserCreated
		r.volume.UserCreatedSnap = append(r.volume.UserCreatedSnap, userCreated)
		if userCreated {
			//This chain is the actual disk chain and does not contain the extra
			//nil at index 0, which is present in r.volume.files
			r.volume.SnapIndx = len(chain) - i
		}
		r.activeDiskData = append(r.activeDiskData, r.diskData[parent])
	}
	return nil
}

func (r *Replica) readMetadata() (bool, error) {
	r.diskData = make(map[string]*disk)

	files, err := ioutil.ReadDir(r.dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.Name() == volumeMetaData {
			if err := r.unmarshalFile(file.Name(), &r.info); err != nil {
				logrus.Errorf("failed to read metadata, error in unmarshalling file: %s", file.Name())
				return false, err
			}
			r.volume.sectorSize = defaultSectorSize
			if file.Size()%defaultSectorSize == 0 {
				r.volume.UsedBlocks += file.Size() / defaultSectorSize
			} else {
				r.volume.UsedBlocks += (file.Size()/defaultSectorSize + 1)
			}
		} else if strings.HasSuffix(file.Name(), metadataSuffix) {
			if err := r.readDiskData(file.Name()); err != nil {
				return false, err
			}
			if file.Size()%defaultSectorSize == 0 {
				r.volume.UsedBlocks += file.Size() / defaultSectorSize
			} else {
				r.volume.UsedBlocks += (file.Size()/defaultSectorSize + 1)
			}
		}
	}

	r.volume.UsedBlocks++ // for revision.counter file which is of 4k

	return len(r.diskData) > 0, nil
}

func (r *Replica) readDiskData(file string) error {
	var data disk
	if err := r.unmarshalFile(file, &data); err != nil {
		logrus.Errorf("failed to read disk data, error while unmarshalling file: %s", file)
		return err
	}

	name := file[:len(file)-len(metadataSuffix)]
	data.Name = name
	r.diskData[name] = &data
	// we are updating the revision count of snapshot with the latest
	// revision count. This is done to know how many io's have been served
	// if replica has been restarted multiple times and new snapshots have
	// been created with no data.
	// This is compared with 1 since revision.counter is initialized
	// with 1 initially.
	if r.diskData[name].RevisionCounter <= 1 {
		r.diskData[name].RevisionCounter = r.GetRevisionCounter()
		logrus.Infof("Update revison count: %v of snapshot: %v", r.diskData[name].RevisionCounter, name)
		if err := r.encodeToFile(r.diskData[name], name+metadataSuffix); err != nil {
			return err
		}
	}
	if data.Parent != "" {
		r.addChildDisk(data.Parent, data.Name)
	}
	return nil
}

func (r *Replica) unmarshalFile(file string, obj interface{}) error {
	p := r.diskPath(file)
	f, err := os.Open(p)
	if err != nil {
		return err
	}

	err = json.NewDecoder(f).Decode(obj)
	if err != nil {
		if closeErr := f.Close(); closeErr != nil {
			logrus.Errorf("Fail to close file: %v, err: %v", f.Name(), closeErr)
		}
		return err
	}

	return f.Close()
}

// Close closes the replica
// TODO Pod evictions sending signals should be catched and Close should be
// called.
func (r *Replica) Close() error {
	r.Lock()
	defer r.Unlock()

	r.mode = types.CLOSED
	return r.close()
}

func (r *Replica) Delete() error {
	r.Lock()
	defer r.Unlock()

	for name := range r.diskData {
		if name != r.info.BackingFileName {
			if err := r.rmDisk(name); err != nil {
				logrus.Error("Error in removing disk data, error : ", err.Error())
				return err
			}
		}
	}

	err := os.Remove(r.diskPath(volumeMetaData))
	if err != nil {
		logrus.Error("Error in removing volume metadata, error : ", err.Error())
		return err
	}
	err = os.Remove(r.diskPath(revisionCounterFile))
	if err != nil {
		logrus.Error("Error in removing revision counter file, error : ", err.Error())
		return err
	}
	return r.syncDir()
}

func (r *Replica) DeleteAll() error {
	r.Lock()
	defer r.Unlock()

	if err := os.RemoveAll(r.dir); err != nil {
		logrus.Error("Error in deleting the directory contents, error : ", err.Error())
		return err
	}
	return nil
}

func (r *Replica) Snapshot(name string, userCreated bool, created string) error {
	r.Lock()
	defer r.Unlock()
	return r.createDisk(name, userCreated, created)
}

func (r *Replica) Revert(name, created string) (*Replica, error) {
	r.Lock()
	defer r.Unlock()

	return r.revertDisk(name, created)
}

func (r *Replica) Sync() (int, error) {
	if r.readOnly {
		return -1, fmt.Errorf("Can not sync on read-only replica")
	}

	if r.ReplicaType != "quorum" {
		r.RLock()
		r.info.Dirty = true
		n, err := r.volume.Sync()
		r.RUnlock()
		if err != nil {
			return n, err
		}
	}
	return 0, nil
}
func (r *Replica) Unmap(offset int64, length int64) (int, error) {
	if r.readOnly {
		return -1, fmt.Errorf("Can not sync on read-only replica")
	}

	if r.ReplicaType != "quorum" {
		r.RLock()
		r.info.Dirty = true
		n, err := r.volume.Unmap(offset, length)
		r.RUnlock()
		if err != nil {
			return n, err
		}
	}
	return 0, nil
}

func (r *Replica) WriteAt(buf []byte, offset int64) (int, error) {
	var (
		c    int
		err  error
		mode types.Mode
	)
	if r.readOnly {
		return 0, fmt.Errorf("Can not write on read-only replica")
	}
	if r.ReplicaType != "quorum" {
		r.RLock()
		r.info.Dirty = true
		c, err = r.volume.WriteAt(buf, offset)
		mode = r.mode
		r.RUnlock()
		if err != nil {
			return c, err
		}
	}
	if mode == types.RW {
		if err := r.increaseRevisionCounter(); err != nil {
			return c, err
		}
	} else if mode != types.WO {
		return c, fmt.Errorf("write happening on invalid rep state %v", mode)
	}
	return c, nil
}

func (r *Replica) ReadAt(buf []byte, offset int64) (int, error) {
	r.RLock()
	c, err := r.volume.ReadAt(buf, offset)
	r.RUnlock()
	return c, err
}

func (r *Replica) GetUsedBlocks() string {
	r.RLock()
	defer r.RUnlock()
	return strconv.FormatInt(r.volume.UsedBlocks, 10)
}

func (r *Replica) GetUsedLogicalBlocks() string {
	r.RLock()
	defer r.RUnlock()
	return strconv.FormatInt(r.volume.UsedLogicalBlocks, 10)
}

func (r *Replica) ListDisks() map[string]types.DiskInfo {
	r.RLock()
	defer r.RUnlock()

	result := map[string]types.DiskInfo{}
	for _, disk := range r.diskData {
		diskSize := strconv.FormatInt(r.getDiskSize(disk.Name), 10)
		diskInfo := types.DiskInfo{
			Name:            disk.Name,
			Parent:          disk.Parent,
			Removed:         disk.Removed,
			UserCreated:     disk.UserCreated,
			Created:         disk.Created,
			Size:            diskSize,
			RevisionCounter: disk.RevisionCounter,
		}
		children := []string{}
		for child := range r.diskChildrenMap[disk.Name] {
			children = append(children, child)
		}
		diskInfo.Children = children
		result[disk.Name] = diskInfo
	}
	return result
}

func (r *Replica) GetRemainSnapshotCounts() int {
	r.RLock()
	defer r.RUnlock()
	maxChainLen := maximumChainLength
	if types.MaxChainLength != 0 {
		maxChainLen = types.MaxChainLength
	}
	return maxChainLen - len(r.activeDiskData)
}

func (r *Replica) GetCloneStatus() string {
	var info Info
	r.RLock()
	defer r.RUnlock()
	if err := r.unmarshalFile(volumeMetaData, &info); err != nil {
		return ""
	}

	return info.CloneStatus
}

func (r *Replica) SetCloneStatus(status string) error {
	r.Lock()
	defer r.Unlock()
	r.cloneStatus = status
	r.info.CloneStatus = status

	return r.encodeToFile(&r.info, volumeMetaData)
}

func (r *Replica) getDiskSize(disk string) int64 {
	return util.GetFileActualSize(r.diskPath(disk))
}

// GetReplicaMode ...
func (r *Replica) GetReplicaMode() string {
	r.Lock()
	defer r.Unlock()
	return string(r.mode)
}

// SetReplicaMode ...
func (r *Replica) SetReplicaMode(mode string) error {
	r.Lock()
	defer r.Unlock()

	if mode == "RW" {
		r.mode = types.RW
	} else if mode == "WO" {
		r.mode = types.WO
	} else {
		return fmt.Errorf("invalid mode string %s", mode)
	}
	return nil
}
