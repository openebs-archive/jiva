package rest

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/openebs/jiva/types"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

func (s *Server) ListReplicas(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	resp := client.GenericCollection{}
	resp.Data = append(resp.Data, s.Replica(apiContext))

	apiContext.Write(&resp)
	return nil
}

func (s *Server) Replica(apiContext *api.ApiContext) *Replica {
	state, info := s.s.Status()
	return NewReplica(apiContext, state, info, s.s.Replica())
}

func (s *Server) GetReplica(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	r := s.Replica(apiContext)
	if mux.Vars(req)["id"] == r.Id {
		apiContext.Write(r)
	} else {
		rw.WriteHeader(http.StatusNotFound)
	}
	return nil
}

func (s *Server) GetReplicaStats(apiContext *api.ApiContext) *types.Stats {
	return s.s.Stats()
}

func (s *Server) GetUsage(apiContext *api.ApiContext) (*types.VolUsage, error) {
	return s.s.GetUsage()
}

func (s *Server) GetStats(rw http.ResponseWriter, req *http.Request) error {
	var stats *types.Stats
	apiContext := api.GetApiContext(req)
	stats = s.GetReplicaStats(apiContext)

	resp := &Stats{
		Resource: client.Resource{
			Type:    "replica",
			Id:      "1",
			Actions: map[string]string{},
			Links:   map[string]string{},
		},
		RevisionCounter: strconv.FormatInt(stats.RevisionCounter, 10),
		ReplicaCounter:  stats.ReplicaCounter,
	}
	apiContext.Write(resp)
	return nil
}

func (s *Server) GetVolUsage(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	usage, _ := s.GetUsage(apiContext)

	resp := &VolUsage{
		Resource: client.Resource{
			Type:    "replica",
			Id:      "1",
			Actions: map[string]string{},
			Links:   map[string]string{},
		},
		UsedLogicalBlocks: strconv.FormatInt(usage.UsedLogicalBlocks, 10),
		UsedBlocks:        strconv.FormatInt(usage.UsedBlocks, 10),
		SectorSize:        strconv.FormatInt(usage.SectorSize, 10),
	}
	apiContext.Write(resp)
	return nil
}

func (s *Server) doOp(req *http.Request, err error) error {
	if err != nil {
		logrus.Errorf("Error %v in doOp: %v", err, req.RequestURI)
		return err
	}

	apiContext := api.GetApiContext(req)
	apiContext.Write(s.Replica(apiContext))
	return nil
}

func (s *Server) SetRebuilding(rw http.ResponseWriter, req *http.Request) error {
	var input RebuildingInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}

	return s.doOp(req, s.s.SetRebuilding(input.Rebuilding))
}

func (s *Server) Create(rw http.ResponseWriter, req *http.Request) error {
	var input CreateInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}

	size := int64(0)
	if input.Size != "" {
		var err error
		size, err = strconv.ParseInt(input.Size, 10, 0)
		if err != nil {
			return err
		}
	}

	return s.doOp(req, s.s.Create(size))
}

func (s *Server) OpenReplica(rw http.ResponseWriter, req *http.Request) error {
	return s.doOp(req, s.s.Open())
}

func (s *Server) Resize(rw http.ResponseWriter, req *http.Request) error {
	var input ResizeInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	return s.doOp(req, s.s.Resize(input.Size))
}

func (s *Server) RemoveDisk(rw http.ResponseWriter, req *http.Request) error {
	var input RemoveDiskInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	return s.doOp(req, s.s.RemoveDiffDisk(input.Name))
}

func (s *Server) ReplaceDisk(rw http.ResponseWriter, req *http.Request) error {
	var input ReplaceDiskInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	return s.doOp(req, s.s.ReplaceDisk(input.Target, input.Source))
}

func (s *Server) PrepareRemoveDisk(rw http.ResponseWriter, req *http.Request) error {
	var input PrepareRemoveDiskInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}
	operations, err := s.s.PrepareRemoveDisk(input.Name)
	if err != nil {
		return err
	}
	apiContext.Write(&PrepareRemoveDiskOutput{
		Resource: client.Resource{
			Id:   input.Name,
			Type: "prepareRemoveDiskOutput",
		},
		Operations: operations,
	})
	return nil
}

func (s *Server) SnapshotReplica(rw http.ResponseWriter, req *http.Request) error {
	var input SnapshotInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}

	if input.Name == "" {
		return fmt.Errorf("Cannot accept empty snapshot name")
	}

	if input.Created == "" {
		return fmt.Errorf("Need to specific created time")
	}

	return s.doOp(req, s.s.Snapshot(input.Name, input.UserCreated, input.Created))
}

func (s *Server) RevertReplica(rw http.ResponseWriter, req *http.Request) error {
	var input RevertInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}

	if input.Name == "" {
		return fmt.Errorf("Cannot accept empty snapshot name")
	}

	if input.Created == "" {
		return fmt.Errorf("Need to specific created time")
	}

	return s.doOp(req, s.s.Revert(input.Name, input.Created))
}

func (s *Server) ReloadReplica(rw http.ResponseWriter, req *http.Request) error {
	var err error
	if err = s.doOp(req, s.s.Reload()); err != nil {
		logrus.Errorf("error in reloadReplica %v", err)
	}
	return err

}

func (s *Server) UpdateCloneInfo(rw http.ResponseWriter, req *http.Request) error {
	var input CloneUpdateInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}
	return s.doOp(req, s.s.UpdateCloneInfo(input.SnapName))
}

func (s *Server) CloseReplica(rw http.ResponseWriter, req *http.Request) error {
	return s.doOp(req, s.s.Close(true))
}

func (s *Server) DeleteReplica(rw http.ResponseWriter, req *http.Request) error {
	return s.doOp(req, s.s.Delete())
}

func (s *Server) StartReplica(rw http.ResponseWriter, req *http.Request) error {
	var action Action
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&action); err != nil && err != io.EOF {
		return err
	}
	return s.doOp(req, s.s.Start(action.Value))
}

func (s *Server) SetRevisionCounter(rw http.ResponseWriter, req *http.Request) error {
	var input RevisionCounter
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}
	counter, _ := strconv.ParseInt(input.Counter, 10, 64)
	return s.doOp(req, s.s.SetRevisionCounter(counter))
}

func (s *Server) UpdatePeerDetails(rw http.ResponseWriter, req *http.Request) error {
	var input PeerDetails
	var details types.PeerDetails
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil && err != io.EOF {
		return err
	}
	details.ReplicaCount = input.ReplicaCount
	details.QuorumReplicaCount = input.QuorumReplicaCount

	return s.doOp(req, s.s.UpdatePeerDetails(details))
}
