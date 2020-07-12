package main

import (
	"bytes"
	"math/rand"
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
)

func verifyData(fd int, tid, iter, offset int64) {
	readBuf := make([]byte, 1024)
	prevWriteBuf := make([]byte, 1024)
	for {
		_, err := syscall.Pread(fd, readBuf, offset)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	copy(prevWriteBuf, []byte(strconv.FormatInt(offset*tid*(iter-1), 10)))
	if !bytes.Equal(readBuf, prevWriteBuf) {
		panic("Data Integrity check failed")
	}
}

func writeData(fd int, tid, iter, offset int64) {
	writeBuf := []byte(strconv.FormatInt(offset*tid*iter, 10))
	for {
		_, err := syscall.Pwrite(fd, writeBuf, offset)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func (config *testConfig) readVerifyWriteTestForIter(fd int, tid int64, region []int64, iter int64) {
	for offset := region[0]; offset < region[1]; offset += 4096 {
		if config.Stop {
			return
		}
		if iter != 1 {
			verifyData(fd, tid, iter, offset)
		}
		writeData(fd, tid, iter, offset)
	}
}

func (config *testConfig) startIOs(tid int64, devPath string) {
	config.insertThread()
	defer config.releaseThread()
	var (
		err error
		fd  int
	)

	if fd, err = syscall.Open(devPath, os.O_RDWR, 0777); err != nil {
		panic(err)
	}

	region := []int64{(tid - 1) * gb, tid * gb}
	iter := int64(1)
	for {
		config.readVerifyWriteTestForIter(fd, tid, region, iter)
		if config.Stop {
			return
		}
		time.Sleep(1 * time.Second)
		iter++
	}
}

func (config *testConfig) runIOs() {
	devPath, err := config.attachDisk()
	if err != nil {
		panic(err)
	}
	for i := 1; i <= 5; i++ {
		tid := i
		go config.startIOs(int64(tid), devPath)
	}
}

func generateRandomIOTable() []int64 {
	var size, sectorSize, blockCount int64
	size = 5 * gb
	sectorSize = 4 * kb
	blockCount = size / sectorSize
	if size%sectorSize != 0 {
		blockCount++
	}
	table := make([]int64, blockCount)
	for i := 0; i <= 1*mb; i++ {
		table[rand.Int63n(blockCount)] = rand.Int63n(blockCount)
	}
	return table
}

func (config *testConfig) testSequentialData() {
	config.runIOs()
}

func (config *testConfig) writeRandomData() []int64 {
	table := generateRandomIOTable()
	devPath, err := config.attachDisk()
	if err != nil {
		panic(err)
	}
	fillBlocks(devPath, table)
	err = config.detachDisk()
	if err != nil {
		panic(err)
	}
	return table
}
func fillBlocks(devPath string, table []int64) {
	var (
		fd  int
		err error
	)
	if fd, err = syscall.Open(devPath, os.O_RDWR, 0777); err != nil {
		panic(err)
	}
	for offset, data := range table {
		writeBuf := []byte(strconv.FormatInt(data, 10))
		for {
			_, err := syscall.Pwrite(fd, writeBuf, int64(offset))
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
	if err = syscall.Close(fd); err != nil {
		panic(err)
	}
}

func (config *testConfig) verifyRandomData(table []int64) {
	devPath, err := config.attachDisk()
	if err != nil {
		panic(err)
	}
	verifyBlocks(devPath, table)
	err = config.detachDisk()
	if err != nil {
		panic(err)
	}
}
func verifyBlocks(devPath string, table []int64) {
	var (
		fd  int
		err error
	)
	if fd, err = syscall.Open(devPath, os.O_RDWR, 0777); err != nil {
		panic(err)
	}
	for offset, data := range table {
		//Skip checking empty blocks
		if data == 0 {
			continue
		}
		readBuf := make([]byte, 1024)
		prevWriteBuf := make([]byte, 1024)
		for {
			_, err := syscall.Pread(fd, readBuf, int64(offset))
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
		copy(prevWriteBuf, []byte(strconv.FormatInt(data, 10)))
		if !bytes.Equal(readBuf, prevWriteBuf) {
			panic("Data Integrity check failed")
		}
	}
	if err = syscall.Close(fd); err != nil {
		panic(err)
	}
}
