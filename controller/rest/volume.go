package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	replicaClient "github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
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

func (s *Server) GetReplicaInfo(replicas []types.Replica) []types.ReplicaInfo {
	var (
		info []types.ReplicaInfo
		wg   sync.WaitGroup
	)
	infoLock := &sync.Mutex{}
	wg.Add(len(replicas))
	for _, replica := range replicas {
		addr := replica.Address
		go func(addr string) {
			defer wg.Done()
			repClient, err := replicaClient.NewReplicaClient(addr)
			if err != nil {
				return
			}
			repClient.SetTimeout(5 * time.Second)
			repInfo, err := repClient.GetReplica()
			if err != nil {
				logrus.Infof("Error in getting info of replica: %v , error %v", addr, err)
				return
			}
			infoLock.Lock()
			info = append(info, repInfo.ReplicaInfo)
			infoLock.Unlock()
		}(addr)
	}
	wg.Wait()
	return info
}

func appendError(errList []error, err error, lock *sync.Mutex) {
	lock.Lock()
	errList = append(errList, err)
	lock.Unlock()
}

func (s *Server) PostSetLoggingRequest(replicas []types.Replica, lf util.LogToFile, errList []error) {
	var (
		wg sync.WaitGroup
	)
	errLock := &sync.Mutex{}
	wg.Add(len(replicas))
	for _, replica := range replicas {
		addr := replica.Address
		go func(addr string) {
			defer wg.Done()
			repClient, err := replicaClient.NewReplicaClient(addr)
			if err != nil {
				appendError(errList, err, errLock)
				return
			}
			repClient.SetTimeout(5 * time.Second)
			err = repClient.SetLogging(lf)
			if err != nil {
				appendError(errList, err, errLock)
				logrus.Infof("Error in setting logging to replica: %v , error %v", addr, err)
				return
			}
		}(addr)
	}
	wg.Wait()
}

func (s *Server) GetVolumeStats(rw http.ResponseWriter, req *http.Request) error {
	var (
		status   string
		replicas []types.Replica
	)
	apiContext := api.GetApiContext(req)
	stats, _ := s.c.Stats()
	s.c.RLock()
	replicas = append(replicas, s.c.ListReplicas()...)
	s.c.RUnlock()
	if s.c.ReadOnly == true {
		status = "RO"
	} else {
		status = "RW"
	}

	replicaInfo := s.GetReplicaInfo(replicas)

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
		ReplicaInfo:       replicaInfo,
		ControllerStatus:  status,
	}
	apiContext.Write(volumeStats)
	return nil
}

func (s *Server) GetCheckpoint(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	s.c.RLock()
	checkpoint := &Checkpoint{
		Resource: client.Resource{Type: "checkpoint"},
		Snapshot: s.c.Checkpoint,
	}
	s.c.RUnlock()
	apiContext.Write(checkpoint)
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

func (s *Server) SetLogging(rw http.ResponseWriter, req *http.Request) error {
	var replicas []types.Replica
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input LoggingInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	s.c.RLock()
	replicas = append(replicas, s.c.ListReplicas()...)
	s.c.RUnlock()

	if len(replicas) != s.c.ReplicationFactor {
		return fmt.Errorf("Can't set logging, replication factor: %v, replica count: %v", s.c.ReplicationFactor, len(replicas))
	}

	var errList []error
	s.PostSetLoggingRequest(replicas, input.LogToFile, errList)
	if errList != nil {
		return fmt.Errorf("Fail to post set logging request, err: %v", errList)
	}

	logrus.Infof("Set logging in replicas to %+v", input.LogToFile)
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
	defer s.c.Unlock()

	apiContext := api.GetApiContext(req)
	var input SnapshotInput
	if err := apiContext.Read(&input); err != nil {
		s.c.Unlock()
		return err
	}
	replicas := s.c.ListReplicas()
	rwCount := 0
	for _, rep := range replicas {
		if rep.Mode == "RW" {
			rwCount++
		}
	}

	if rwCount != s.c.ReplicationFactor {
		return fmt.Errorf(
			"Can't delete snapshot, rwReplicaCount:%v != ReplicationFactor:%v",
			rwCount, s.c.ReplicationFactor,
		)
	}

	if s.c.Checkpoint == "" {
		return fmt.Errorf(
			"Can't delete snapshot, checkpoint not set at controller",
		)
	}
	if strings.Contains(s.c.Checkpoint, input.Name) {
		return fmt.Errorf(
			"Can't delete snapshot, snapshotName same as checkpoint",
		)
	}
	logrus.Infof("Delete snapshot: %s", input.Name)
	err := s.c.DeleteSnapshot(input.Name, replicas)
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
