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
