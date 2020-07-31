package main

import (
	"io/ioutil"
	"os"
	"runtime/debug"
	"time"

	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
	checkV1 "gopkg.in/check.v1"
)

func testFunctions() {
	revisionCountUpdateInParentTest()
	getDeleteCandidateChainFuncTest()
}

// revisionCountUpdateInParentTest verifies if on deleting a snapshot,
// its parent is updated with the child's revision counter
func revisionCountUpdateInParentTest() {
	var c *checkV1.C
	dir, err := ioutil.TempDir("", "replica")
	c.Assert(err, checkV1.IsNil)
	defer os.RemoveAll(dir)

	r, err := replica.New(true, 10*4096, 4096, dir, nil, "Backend")
	c.Assert(err, checkV1.IsNil)
	defer r.Close()
	err = r.SetReplicaMode("RW")
	c.Assert(err, checkV1.IsNil)

	buf := make([]byte, 4096*3)
	fill(buf, 1)
	_, err = r.WriteAt(buf, 0)
	c.Assert(err, checkV1.IsNil)

	now := getNow()
	err = r.Snapshot("000", false, now) // Revision Count = 2
	c.Assert(err, checkV1.IsNil)

	buf = make([]byte, 4096)
	fill(buf, 1)
	_, err = r.WriteAt(buf, 0)
	c.Assert(err, checkV1.IsNil)

	err = r.Snapshot("001", false, now) // Revision count = 3
	c.Assert(err, checkV1.IsNil)

	err = r.Snapshot("002", false, now) // Revision count = 3
	c.Assert(err, checkV1.IsNil)

	// Check if revision counter of snap-001
	// is copied to snap-000 when snap-001 is deleted
	disks := r.ListDisks()
	prevRevCnt := disks["volume-snap-000.img"].RevisionCounter
	desiredRevCnt := disks["volume-snap-001.img"].RevisionCounter
	c.Assert(prevRevCnt, checkV1.Not(checkV1.Equals), desiredRevCnt)
	go func() {
		time.Sleep(3 * time.Second)
		types.DrainOps = types.DrainDone
	}()
	r.RemoveDiffDisk("volume-snap-001.img")
	disks = r.ListDisks()
	actualRevCnt := disks["volume-snap-000.img"].RevisionCounter
	c.Assert(actualRevCnt, checkV1.Equals, desiredRevCnt)

	// Verify that revision counter is persisted on disk
	err = r.Close()
	c.Assert(err, checkV1.IsNil)
	r, err = replica.New(true, 10*4096, 4096, dir, nil, "Backend")
	c.Assert(err, checkV1.IsNil)
	err = r.SetReplicaMode("RW")
	c.Assert(err, checkV1.IsNil)
	disks = r.ListDisks()
	actualRevCnt = disks["volume-snap-000.img"].RevisionCounter
	c.Assert(actualRevCnt, checkV1.Equals, desiredRevCnt)
}

func getDeleteCandidateChainFuncTest() {

	var c *checkV1.C
	dir, err := ioutil.TempDir("", "replica")
	c.Assert(err, checkV1.IsNil)
	defer os.RemoveAll(dir)

	r, err := replica.New(true, 10*4096, 4096, dir, nil, "Backend")
	c.Assert(err, checkV1.IsNil)
	defer r.Close()
	err = r.SetReplicaMode("RW")
	c.Assert(err, checkV1.IsNil)

	now := getNow()
	err = r.Snapshot("000", false, now)
	c.Assert(err, checkV1.IsNil)

	err = r.Snapshot("001", false, now)
	c.Assert(err, checkV1.IsNil)

	//Chain len equals 3 test
	list, err := sync.GetDeleteCandidateChain(r, "volume-snap-001.img")
	c.Assert(err, checkV1.IsNil)
	c.Assert(list, checkV1.IsNil)

	//Empty checkpoint test
	list, err = sync.GetDeleteCandidateChain(r, "")
	c.Assert(err, checkV1.IsNil)
	c.Assert(list, checkV1.IsNil)

	//Non existent checkpoint test
	list, err = sync.GetDeleteCandidateChain(r, "fake")
	c.Assert(err, checkV1.IsNil)
	c.Assert(list, checkV1.IsNil)

	err = r.Snapshot("002", false, now)
	c.Assert(err, checkV1.IsNil)

	// Simple test
	list, err = sync.GetDeleteCandidateChain(r, "volume-snap-002.img")
	c.Assert(err, checkV1.IsNil)
	verifyList(list, []string{"volume-snap-001.img"})

	err = r.Snapshot("003", true, now)
	c.Assert(err, checkV1.IsNil)
	err = r.Snapshot("004", false, now)
	c.Assert(err, checkV1.IsNil)
	err = r.Snapshot("005", false, now)
	c.Assert(err, checkV1.IsNil)

	// User Created snapshot test
	list, err = sync.GetDeleteCandidateChain(r, "volume-snap-005.img")
	c.Assert(err, checkV1.IsNil)
	verifyList(list, []string{"volume-snap-001.img", "volume-snap-002.img", "volume-snap-004.img"})

	_, err = r.PrepareRemoveDisk("volume-snap-003.img")
	c.Assert(err, checkV1.IsNil)
	// User Created snapshot removed test
	list, err = sync.GetDeleteCandidateChain(r, "volume-snap-005.img")
	c.Assert(err, checkV1.IsNil)
	verifyList(list, []string{"volume-snap-001.img", "volume-snap-002.img", "volume-snap-003.img", "volume-snap-004.img"})

	buf := make([]byte, 4096)
	fill(buf, 1)
	_, err = r.WriteAt(buf, 0)
	c.Assert(err, checkV1.IsNil)

	err = r.Snapshot("006", false, now) // Contains 100 bytes data
	c.Assert(err, checkV1.IsNil)
	err = r.Snapshot("007", false, now)
	c.Assert(err, checkV1.IsNil)

	buf = make([]byte, 8192)
	fill(buf, 2)
	_, err = r.WriteAt(buf, 5)
	c.Assert(err, checkV1.IsNil)

	err = r.Snapshot("008", false, now)
	c.Assert(err, checkV1.IsNil)
	err = r.Snapshot("009", false, now)
	c.Assert(err, checkV1.IsNil)

	// Verify sorted chain
	list, err = sync.GetDeleteCandidateChain(r, "volume-snap-009.img")
	c.Assert(err, checkV1.IsNil)
	verifyList(list, []string{"volume-snap-001.img", "volume-snap-002.img", "volume-snap-003.img", "volume-snap-004.img",
		"volume-snap-005.img", "volume-snap-007.img", "volume-snap-006.img", "volume-snap-008.img"})
}

func verifyList(actual []string, expected []string) {
	for i, snap := range expected {
		if actual[i] != snap {
			debug.PrintStack()
			logrus.Fatalf("VerifyList() failed")
		}
	}
}

func getNow() string {
	// Make sure timestamp is unique
	time.Sleep(1 * time.Second)
	return util.Now()
}
func fill(buf []byte, val byte) {
	for i := 0; i < len(buf); i++ {
		buf[i] = val
	}
}
