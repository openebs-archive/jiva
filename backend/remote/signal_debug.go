// +build debug

package remote

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/util"
)

func (rf *Factory) SignalToAdd(address string, action string) error {
	controlAddress, _, _, err := util.ParseAddresses(address + ":9502")
	if err != nil {
		return err
	}
	r := &Remote{
		Name:       address,
		replicaURL: fmt.Sprintf("http://%s/v1/replicas/1", controlAddress),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
	logrus.Infof("Add sleep for 5 sec, before sending signal '%s'", action)
	time.Sleep(5 * time.Second)
	return r.doAction("start", &map[string]string{"Action": action})
}
