package rest

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

// DeleteVolume deletes all the contents of the volume. It purges all the replica
// files.
func (s *Server) DeleteVolume(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("DeleteVolume")
	apiContext := api.GetApiContext(req)
	err := s.s.DeleteAll()
	if err != nil {
		logrus.Errorf("Error in deleting the volume, error: %v", err)
		return err
	}
	apiContext.Write(&DeleteReplicaOutput{
		client.Resource{
			Type: "delete",
		},
	})
	return nil
}
