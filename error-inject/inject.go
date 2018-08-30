// +build debug

package inject

import (
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
)

func AddTimeout() {
	timeout, _ := strconv.Atoi(os.Getenv("DEBUG_TIMEOUT"))
	logrus.Infof("Add timeout of %vs for debug build", timeout)
	time.Sleep(time.Duration(timeout) * time.Second)
}
