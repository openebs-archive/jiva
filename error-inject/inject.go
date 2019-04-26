// +build debug

package inject

import (
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	pingTimeout = true
)

// DisablePunchHoles is used for disabling punch holes
func DisablePunchHoles() bool {
	ok := os.Getenv("DISABLE_PUNCH_HOLES")
	if ok == "True" {
		return true
	}
	return false
}

// AddTimeout add delays into the code
func AddTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("DEBUG_TIMEOUT"))
	logrus.Infof("Add timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}

// AddPingTimeout add delay in ping response
func AddPingTimeout() {
	if pingTimeout {
		timeout, _ := strconv.Atoi(os.Getenv("RPC_PING_TIMEOUT"))
		logrus.Infof("Add ping timeout of %vs for debug build", timeout)
		time.Sleep(time.Duration(timeout) * time.Second)
		pingTimeout = false
	}
}

// AddPreloadTimeout add delay in preload
func AddPreloadTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("PRELOAD_TIMEOUT"))
	logrus.Infof("Add preload timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}

// PanicAfterPrepareRebuild panic the replica just after prepare rebuild
func PanicAfterPrepareRebuild() {
	ok := os.Getenv("PANIC_AFTER_PREPARE_REBUILD")
	if ok == "TRUE" {
		time.Sleep(2 * time.Second)
		panic("panic replica after getting start signal")
	}
}

// AddPunchHoleTimeout add delay in while punching hole
func AddPunchHoleTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("PUNCH_HOLE_TIMEOUT"))
	logrus.Infof("Add punch hole timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}
