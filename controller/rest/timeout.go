package rest

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/api"
)

func (s *Server) AddTimeout(rw http.ResponseWriter, req *http.Request) error {
	var timeout Timeout
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&timeout); err != nil {
		logrus.Error("failed to read the request body, error: %v", err)
		return err
	}
	if timeout.Timeout != "" {
		logrus.Infof("Added a timeout of %vs", timeout.Timeout)
		return os.Setenv("DEBUG_TIMEOUT", timeout.Timeout)
	}
	if timeout.RPCPingTimeout != "" {
		logrus.Infof("Added a ping timeout of %vs", timeout.RPCPingTimeout)
		return os.Setenv("RPC_PING_TIMEOUT", timeout.RPCPingTimeout)
	}
	return fmt.Errorf("Error in setting timeout, received empty value")
}
