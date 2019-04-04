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
	startCount  int
)

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

func Panic() {
	ok := os.Getenv("SHOULD_PANIC")
	if ok == "TRUE" {
		time.Sleep(2 * time.Second)
		panic("panic replica after getting start signal")
	}
}

func IncStartCountAndPanic() {
	count, _ := strconv.Atoi(os.Getenv("START_COUNT"))
	logrus.Infof("Start signal count for debug build", count)
	startCount++
	if startCount == count {
		panic("panic replica after getting start signal twice")
	}
}
