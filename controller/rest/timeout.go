package rest

import (
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
	logrus.Infof("Added a timeout of %vs", timeout.Timeout)
	return os.Setenv("DEBUG_TIMEOUT", timeout.Timeout)
}
