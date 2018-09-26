package replica

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/frostschutz/go-fibmap"
	"github.com/openebs/jiva/types"
)

type UsedGenerator struct {
	err  error
	disk types.DiffDisk
	d    *diffDisk
}

func newGenerator(diffDisk *diffDisk, disk types.DiffDisk) *UsedGenerator {
	return &UsedGenerator{
		disk: disk,
		d:    diffDisk,
	}
}

func (u *UsedGenerator) Err() error {
	return u.err
}

func (u *UsedGenerator) Generate() <-chan int64 {
	c := make(chan int64)
	go u.findExtents(c)
	return c
}

func (u *UsedGenerator) findExtents(c chan<- int64) {
	defer close(c)

	fd := u.disk.Fd()

	// The backing file will have a Fd of 0
	if fd == 0 {
		return
	}

	start := uint64(0)
	end := uint64(len(u.d.location)) * uint64(u.d.sectorSize)
	for {
		extents, errno := fibmap.Fiemap(fd, start, end-start, 1024)
		if errno != 0 {
			u.err = errno
			return
		}

		if len(extents) == 0 {
			return
		}

		for _, extent := range extents {
			start = extent.Logical + extent.Length
			for i := int64(0); i < int64(extent.Length); i += u.d.sectorSize {
				c <- (int64(extent.Logical) + i) / u.d.sectorSize
			}
			if extent.Flags&fibmap.FIEMAP_EXTENT_LAST != 0 {
				return
			}
		}
	}
}

func createTempFile(path string) error {
	var file *os.File
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(path)
			if err != nil {
				logrus.Errorf("failed to create tmp file %s, error: %v", path, err.Error())
				return err
			}
		}
	}
	defer file.Close()
	if _, err := file.WriteString("This is temp file\n"); err != nil {
		return err
	}
	return file.Sync()
}

func isExtentSupported(dir string) error {
	path := dir + "/tmpFile.tmp"
	if err := createTempFile(path); err != nil {
		return err
	}
	defer os.Remove(path)
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	fiemapFile := fibmap.NewFibmapFile(file)
	if _, err := fiemapFile.Fiemap(uint32(fileInfo.Size())); err != 0 {
		// verify is FIBMAP is supported incase FIEMAP failed
		if _, err := fiemapFile.FibmapExtents(); err != 0 {
			return fmt.Errorf("failed to find extents, error: %v", err.Error())
		}
	}
	return nil
}
