package rest

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/openebs/jiva/util"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

func (s *Server) GetVersion(rw http.ResponseWriter, req *http.Request) error {
	logrus.Info("get version")
	details := util.GetVersionDetails()
	apiContext := api.GetApiContext(req)
	apiContext.Write(&Version{
		client.Resource{
			Id:   "details",
			Type: "version",
		},
		details.Version,
		details.GitCommitID,
		details.BuildDate,
	})
	return nil
}
