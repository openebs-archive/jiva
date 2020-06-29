// +build debug

package inject

import (
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	pingTimeout = true
)

var Envs map[string](map[string]bool)

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

// PanicWhileSettingCheckpoint panics the replica on receiving SetCheckpoint REST Call
func PanicWhileSettingCheckpoint(replicaIP string) {
	ok := os.Getenv("PANIC_WHILE_SETTING_CHECKPOINT")
	if ok == "TRUE" || (Envs[replicaIP])["PANIC_WHILE_SETTING_CHECKPOINT"] {
		panic("panic replica while setting checkpoint")
	}
}

// AddPunchHoleTimeout add delay in while punching hole
func AddPunchHoleTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("PUNCH_HOLE_TIMEOUT"))
	logrus.Infof("Add punch hole timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}

var UpdateLUNMapTimeoutTriggered bool

// AddUpdateLUNMapTimeout adds delay during UpdateLUNMap
func AddUpdateLUNMapTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("UpdateLUNMap_TIMEOUT"))
	logrus.Infof("AddUpdateLUNMap timeout of %vs for debug build", timeout)
	UpdateLUNMapTimeoutTriggered = true
	time.Sleep(time.Duration(timeout) * time.Second)
}
