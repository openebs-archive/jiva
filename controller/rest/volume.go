package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/sirupsen/logrus"
)

func (s *Server) ListVolumes(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	apiContext.Write(&client.GenericCollection{
		Data: []interface{}{
			s.listVolumes(apiContext)[0],
		},
	})
	return nil
}

func (s *Server) GetVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	apiContext.Write(v)
	return nil
}

func (s *Server) GetVolumeStats(rw http.ResponseWriter, req *http.Request) error {
	var status string
	apiContext := api.GetApiContext(req)
	stats, _ := s.c.Stats()
	replicas := s.c.ListReplicas()

	if s.c.ReadOnly == true {
		status = "RO"
	} else {
		status = "RW"
	}

	volumeStats := &VolumeStats{
		Resource:          client.Resource{Type: "stats"},
		RevisionCounter:   stats.RevisionCounter,
		ReplicaCounter:    len(replicas),
		SCSIIOCount:       stats.SCSIIOCount,
		IsClientConnected: stats.IsClientConnected,

		ReadIOPS:            strconv.FormatInt(stats.ReadIOPS, 10),
		TotalReadTime:       strconv.FormatInt(stats.TotalReadTime, 10),
		TotalReadBlockCount: strconv.FormatInt(stats.TotalReadBlockCount, 10),

		WriteIOPS:            strconv.FormatInt(stats.WriteIOPS, 10),
		TotalWriteTime:       strconv.FormatInt(stats.TotalWriteTime, 10),
		TotalWriteBlockCount: strconv.FormatInt(stats.TotalWriteBlockCount, 10),

		UsedLogicalBlocks: strconv.FormatInt(stats.UsedLogicalBlocks, 10),
		UsedBlocks:        strconv.FormatInt(stats.UsedBlocks, 10),
		SectorSize:        strconv.FormatInt(stats.SectorSize, 10),
		Size:              strconv.FormatInt(s.c.GetSize(), 10),
		UpTime:            fmt.Sprintf("%f", time.Since(s.c.StartTime).Seconds()),
		Name:              s.c.Name,
		Replica:           replicas,
		ControllerStatus:  status,
	}
	apiContext.Write(volumeStats)
	return nil
}

func (s *Server) ShutdownVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	if err := s.c.Shutdown(); err != nil {
		return err
	}

	return s.GetVolume(rw, req)
}

func (s *Server) RevertVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input RevertInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	if err := s.c.Revert(input.Name); err != nil {
		return err
	}

	return s.GetVolume(rw, req)
}

func (s *Server) ResizeVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input ResizeInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	logrus.Infof("Resize volume to %s", input.Size)
	if err := s.c.Resize(input.Name, input.Size); err != nil {
		logrus.Error(err)
		return err
	}

	return s.GetVolume(rw, req)
}

func (s *Server) SnapshotVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input SnapshotInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	name, err := s.c.Snapshot(input.Name)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Snapshot: %s created successfully", name)
	apiContext.Write(&SnapshotOutput{
		client.Resource{
			Id:   name,
			Type: "snapshotOutput",
		},
		msg,
	})
	return nil
}

func (s *Server) StartVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input StartInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	if err := s.c.Start(input.Replicas...); err != nil {
		return err
	}

	return s.GetVolume(rw, req)
}

func (s *Server) listVolumes(context *api.ApiContext) []*Volume {
	return []*Volume{
		NewVolume(context, s.c.Name, s.c.ReadOnly, len(s.c.ListReplicas())),
	}
}

func (s *Server) getVolume(context *api.ApiContext, id string) *Volume {
	for _, v := range s.listVolumes(context) {
		if v.Id == id {
			return v
		}
	}
	return nil
}

// DeleteSnapshot ...
func (s *Server) DeleteSnapshot(rw http.ResponseWriter, req *http.Request) error {
	s.c.Lock()
	if s.c.IsSnapDeletionInProgress {
		s.c.Unlock()
		return fmt.Errorf("Snapshot deletion process is in progress, %s is getting deleted", s.c.SnapshotName)
	}

	apiContext := api.GetApiContext(req)
	var input SnapshotInput
	if err := apiContext.Read(&input); err != nil {
		s.c.Unlock()
		return err
	}
	replicas := s.c.ListReplicas()
	s.c.SnapshotName = input.Name
	rf := s.c.ReplicationFactor
	if len(replicas) != rf {
		s.c.Unlock()
		return fmt.Errorf("Can not remove a snapshot because, RF: %v, replica count: %v", rf, len(replicas))
	}

	for _, rep := range replicas {
		r := rep // pin it
		if err := s.c.IsReplicaRW(&r); err != nil {
			s.c.Unlock()
			return fmt.Errorf("Can't delete snapshot %s, err: %v", s.c.SnapshotName, err)
		}
	}
	s.c.IsSnapDeletionInProgress = true
	s.c.Unlock()
	defer func() {
		s.c.Lock()
		s.c.IsSnapDeletionInProgress = false
		s.c.Unlock()
	}()

	logrus.Infof("Delete snapshot: %s", input.Name)
	err := s.c.DeleteSnapshot(replicas)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Snapshot: %s deleted successfully", input.Name)
	apiContext.Write(&SnapshotOutput{
		client.Resource{
			Id:   input.Name,
			Type: "snapshotOutput",
		},
		msg,
	})
	return nil
}
