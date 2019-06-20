package replica

import (
	"fmt"
	"sync"
	"syscall"

	fibmap "github.com/frostschutz/go-fibmap"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/types"
	"github.com/rancher/sparse-tools/sparse"
)

type fileType struct {
	fileName string
}

type diffDisk struct {
	rmLock sync.Mutex
	// mapping of sector to index in the files array. a value of 0
	// is special meaning we don't know the location yet.
	location          []int
	UsedLogicalBlocks int64
	UsedBlocks        int64
	// list of files in grandparent, parent, child order
	// For exp: H->S4->S3->S2->S1->S0
	// H is active file, and S4 is latest snapshot and is
	// parent of H. H has no child, S0 has no parent.
	// So files = {nil, S0, S1, S2, S3, S4, H}
	// index 0 is nil and index 1 is base snapshot and last index
	// is the active write layer.
	// First index is nil for doing comparisons.
	files []types.DiffDisk
	// list representing file index marked true/false for
	// userCreated/Auto-Created accordingly. It should always be updated
	// whenever modifying above "files" variable
	UserCreatedSnap []bool
	// Index of latest user created snapshot
	SnapIndx   int
	sectorSize int64
}

// RemoveIndex remove the index from list the files
// and update the location of offsets in respective
// new files where it is merged before deletion of
// the snapshot.
//
// Let's consider this case: H->S4->US3->S2->S1->S0
// Update d.location and latest user created snapshot
// index if S2 is deleted, US3 will take position of S2
// So resultant chain is H->S4->US3->S1->S0
//
// You can notice here as position of S0 and S1 is still
// the same but we shifting the positions of the childrens.
// NOTE: UserCreatedSnap contains the truth value of
// whether snapshot is UserCreated or not in increasing
// order of index. i.e, last index will have the truthy
// value of latest snapshot
func (d *diffDisk) RemoveIndex(index int) error {
	if err := d.files[index].Close(); err != nil {
		return err
	}

	//TODO Decide if d.location should be preloaded again over here
	for i := 0; i < len(d.location); i++ {
		if d.location[i] > index && d.location[i] != 1 {
			d.location[i]--
		}
	}

	d.files = append(d.files[:index], d.files[index+1:]...)
	d.UserCreatedSnap = append(d.UserCreatedSnap[:index], d.UserCreatedSnap[index+1:]...)
	for i, userCreated := range d.UserCreatedSnap {
		if userCreated {
			d.SnapIndx = i
		}
	}

	return nil
}

func (d *diffDisk) WriteAt(buf []byte, offset int64) (int, error) {
	startOffset := offset % d.sectorSize
	startCut := d.sectorSize - startOffset
	endOffset := (int64(len(buf)) + offset) % d.sectorSize

	if len(buf) == 0 {
		return 0, nil
	}

	if startOffset == 0 && endOffset == 0 {
		return d.fullWriteAt(buf, offset)
	}

	// single block
	if startCut >= int64(len(buf)) {
		return d.readModifyWrite(buf, offset)
	}

	if _, err := d.readModifyWrite(buf[0:startCut], offset); err != nil {
		return 0, err
	}

	if _, err := d.fullWriteAt(buf[startCut:int64(len(buf))-endOffset], offset+startCut); err != nil {
		return 0, err
	}

	if _, err := d.readModifyWrite(buf[int64(len(buf))-endOffset:], offset+int64(len(buf))-endOffset); err != nil {
		return 0, err
	}

	return len(buf), nil
}

func (d *diffDisk) readModifyWrite(buf []byte, offset int64) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	d.rmLock.Lock()
	defer d.rmLock.Unlock()

	readBuf := make([]byte, d.sectorSize)
	readOffset := (offset / d.sectorSize) * d.sectorSize

	if _, err := d.fullReadAt(readBuf, readOffset); err != nil {
		return 0, err
	}

	copy(readBuf[offset%d.sectorSize:], buf)

	return d.fullWriteAt(readBuf, readOffset)
}

func (d *diffDisk) Sync() (int, error) {
	target := len(d.files) - 1
	fd := d.files[target].Fd()
	err := syscall.Fsync(int(fd))
	if err != nil {
		return -1, err
	}
	return 0, err
}

func (d *diffDisk) Unmap(offset int64, length int64) (int, error) {
	var err error
	for indx, file := range d.files {
		if indx <= d.SnapIndx || file == nil {
			continue
		}
		fd := file.Fd()
		err = syscall.Fallocate(int(fd),
			sparse.FALLOC_FL_KEEP_SIZE|sparse.FALLOC_FL_PUNCH_HOLE,
			offset, length)
		if err != nil {
			return -1, err
		}
	}
	return 0, err

}

func (d *diffDisk) fullWriteAt(buf []byte, offset int64) (int, error) {
	var (
		length   int64
		lOffset  int64
		file     types.DiffDisk
		fileIndx int
	)
	if int64(len(buf))%d.sectorSize != 0 || offset%d.sectorSize != 0 {
		return 0, fmt.Errorf("Write len(%d), offset %d not a multiple of %d", len(buf), offset, d.sectorSize)
	}

	target := len(d.files) - 1
	startSector := offset / d.sectorSize
	sectors := int64(len(buf)) / d.sectorSize

	c, err := d.files[target].WriteAt(buf, offset)

	// Regardless of err mark bytes as written
	for i := int64(0); i < sectors; i++ {
		offset = startSector + i
		if val := d.location[startSector+i]; val == 0 {
			d.UsedLogicalBlocks++
			d.UsedBlocks++
		} else if val != target {
			//We are looking for continuous blocks over here.
			//If the file of the next block is changed, we punch a hole
			//for the previous unpunched blocks, and reset the file and
			//fileIndx pointed to by this block
			if d.location[startSector+i] != fileIndx ||
				startSector+i != lOffset+length {
				if file != nil && int(fileIndx) > d.SnapIndx && shouldCreateHoles() && !inject.DisablePunchHoles() {
					sendToCreateHole(d.files[val], lOffset*d.sectorSize, length*d.sectorSize)
				}
				file = d.files[d.location[startSector+i]]
				fileIndx = d.location[offset]
				length = 1
				lOffset = startSector + i
			} else {
				//If this is the last block in the loop, hole for this
				//block will be punched outside the loop
				length++
			}
		}
		d.location[startSector+i] = target
	}
	//This will take care of the case when the last call in the above loop
	//enters else case
	if (file != nil) && (int(fileIndx) > d.SnapIndx) && shouldCreateHoles() && !inject.DisablePunchHoles() {
		sendToCreateHole(file, lOffset*d.sectorSize, length*d.sectorSize)
	}
	file = nil
	fileIndx = 0
	return c, err
}

func (d *diffDisk) ReadAt(buf []byte, offset int64) (int, error) {
	startOffset := offset % d.sectorSize
	startCut := d.sectorSize - startOffset
	endOffset := (int64(len(buf)) + offset) % d.sectorSize

	if len(buf) == 0 {
		return 0, nil
	}

	if startOffset == 0 && endOffset == 0 {
		return d.fullReadAt(buf, offset)
	}

	readBuf := make([]byte, d.sectorSize)
	if _, err := d.fullReadAt(readBuf, offset-startOffset); err != nil {
		return 0, err
	}

	copy(buf, readBuf[startOffset:])

	if startCut >= int64(len(buf)) {
		return len(buf), nil
	}

	if _, err := d.fullReadAt(buf[startCut:int64(len(buf))-endOffset], offset+startCut); err != nil {
		return 0, err
	}

	if endOffset > 0 {
		if _, err := d.fullReadAt(readBuf, offset+int64(len(buf))-endOffset); err != nil {
			return 0, err
		}
		copy(buf[int64(len(buf))-endOffset:], readBuf[:endOffset])
	}

	return len(buf), nil
}

func (d *diffDisk) fullReadAt(buf []byte, offset int64) (int, error) {
	var (
		err error
		c   int
	)

	if int64(len(buf))%d.sectorSize != 0 || offset%d.sectorSize != 0 {
		return 0, fmt.Errorf("Read not a multiple of %d", d.sectorSize)
	}

	if len(buf) == 0 {
		return 0, nil
	}

	count := 0
	sectors := int64(len(buf)) / d.sectorSize
	readSectors := int64(1)
	startSector := offset / d.sectorSize
	target, err := d.lookup(startSector)
	if err != nil {
		return count, err
	}

	for i := int64(1); i < sectors; i++ {
		newTarget, err := d.lookup(startSector + i)
		if err != nil {
			return count, err
		}

		if newTarget == target {
			readSectors++
		} else {
			if target == 0 {
				count += int(readSectors * d.sectorSize)
			} else {
				c, err = d.read(target, buf, offset, i-readSectors, readSectors)
				count += c
			}
			if err != nil {
				return count, err
			}
			readSectors = 1
			target = newTarget
		}
	}

	if readSectors > 0 {
		if target == 0 {
			count += int(readSectors * d.sectorSize)
		} else {
			c, err = d.read(target, buf, offset, sectors-readSectors, readSectors)
			count += c
		}
		if err != nil {
			return count, err
		}
	}

	return count, nil
}

func (d *diffDisk) read(target int, buf []byte, offset int64, startSector int64, sectors int64) (int, error) {
	bufStart := startSector * d.sectorSize
	bufEnd := sectors * d.sectorSize
	newBuf := buf[bufStart : bufStart+bufEnd]
	return d.files[target].ReadAt(newBuf, offset+bufStart)
}

func (d *diffDisk) lookup(sector int64) (int, error) {
	if sector >= int64(len(d.location)) {
		// We know the IO will result in EOF
		return len(d.files) - 1, nil
	}

	// small optimization
	if int64(len(d.files)) == 2 {
		return 1, nil
	}

	target := d.location[sector]
	if target == 0 {
		for i := len(d.files) - 1; i > 0; i-- {
			if i == 1 {
				// This is important that for index 1 we don't check Fiemap because it may be a base image file
				// Also the result has to be 1
				d.location[sector] = i
				return i, nil
			}

			e, err := fibmap.Fiemap(d.files[i].Fd(), uint64(sector*d.sectorSize), uint64(d.sectorSize), 1)
			if err != 0 {
				return 0, err
			}
			if len(e) > 0 {
				d.location[sector] = i
				return i, nil
			}
		}
		return len(d.files) - 1, nil
	}
	return target, nil
}
