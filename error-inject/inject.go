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

func IsDebugBuild() bool {
	ok := os.Getenv("IS_DEBUG_BUILD")
	if ok == "True" {
		return true
	}
	return false
}

func AddTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("DEBUG_TIMEOUT"))
	logrus.Infof("Add timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}

func AddPingTimeout() {
	if pingTimeout {
		timeout, _ := strconv.Atoi(os.Getenv("RPC_PING_TIMEOUT"))
		logrus.Infof("Add ping timeout of %vs for debug build", timeout)
		time.Sleep(time.Duration(timeout) * time.Second)
		pingTimeout = false
	}
}

func AddPreloadTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("PRELOAD_TIMEOUT"))
	logrus.Infof("Add preload timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}

func PanicAfterPrepareRebuild() {
	ok := os.Getenv("PANIC_AFTER_PREPARE_REBUILD")
	if ok == "TRUE" {
		time.Sleep(2 * time.Second)
		panic("panic replica after getting start signal")
	}
}

func AddPunchHoleTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("PUNCH_HOLE_TIMEOUT"))
	logrus.Infof("Add punch hole timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}
