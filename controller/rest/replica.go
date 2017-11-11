package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/openebs/jiva/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

var (
	// OpenEBSJivaRegestrationRequestDuration gets the response time of the
	// requested api.
	OpenEBSJivaRegestrationRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "openebs_jiva_registration_request_duration_seconds",
			Help:    "Request response time of the /v1/register to register replicas.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.5, 1, 2.5, 5, 10},
		},
		// code is http code and method is http method returned by
		// endpoint "/v1/volume"
		[]string{"code", "method"},
	)
	// OpenEBSJivaRegestrationRequestCounter Count the no of request Since a request has been made on /v1/volume
	OpenEBSJivaRegestrationRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "openebs_jiva_registration_requests_total",
			Help: "Total number of /v1/register requests to register replicas.",
		},
		[]string{"code", "method"},
	)
)

// init registers Prometheus metrics.It's good to register these varibles here
// otherwise you need to register it before you are going to use it. So you will
// have to register it everytime unnecessarily, instead initialize it once and
// use anywhere at anytime through the code.
func init() {
	prometheus.MustRegister(OpenEBSJivaRegestrationRequestDuration)
	prometheus.MustRegister(OpenEBSJivaRegestrationRequestCounter)
}

func (s *Server) ListReplicas(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	resp := client.GenericCollection{}
	for _, r := range s.c.ListReplicas() {
		resp.Data = append(resp.Data, NewReplica(apiContext, r.Address, r.Mode))
	}

	resp.ResourceType = "replica"
	resp.CreateTypes = map[string]string{
		"replica": apiContext.UrlBuilder.Collection("replica"),
	}

	apiContext.Write(&resp)
	return nil
}

func (s *Server) GetReplica(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	apiContext.Write(s.getReplica(apiContext, id))
	return nil
}

func (s *Server) RegisterReplica(rw http.ResponseWriter, req *http.Request) error {
	var (
		regReplica    RegReplica
		localRevCount int64
		code          int
	)
	start := time.Now()
	apiContext := api.GetApiContext(req)
	s.RequestDuration = OpenEBSJivaRegestrationRequestDuration
	s.RequestCounter = OpenEBSJivaRegestrationRequestCounter

	if err := apiContext.Read(&regReplica); err != nil {
		return err
	}

	localRevCount, _ = strconv.ParseInt(regReplica.RevCount, 10, 64)
	local := types.RegReplica{
		Address:    regReplica.Address,
		RevCount:   localRevCount,
		PeerDetail: regReplica.PeerDetails,
		RepType:    regReplica.RepType,
		UpTime:     regReplica.UpTime,
		RepState:   regReplica.RepState,
	}
	code = http.StatusOK
	rw.WriteHeader(code)
	defer func() {
		// This will Display the metrics something similar to
		// the examples given below
		// exp: openebs_jiva_registration_request_duration_seconds{code="200", method="POST"}
		s.RequestDuration.WithLabelValues(strconv.Itoa(code), req.Method).Observe(time.Since(start).Seconds())

		// This will Display the metrics something similar to
		// the examples given below
		// exp: openebs_jiva_registration_requests_total{code="200", method="POST"}
		s.RequestCounter.WithLabelValues(strconv.Itoa(code), req.Method).Inc()
	}()
	return s.c.RegisterReplica(local)

}

func (s *Server) CreateReplica(rw http.ResponseWriter, req *http.Request) error {
	var replica Replica
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&replica); err != nil {
		return err
	}

	if err := s.c.AddReplica(replica.Address); err != nil {
		return err
	}

	apiContext.Write(s.getReplica(apiContext, replica.Address))
	return nil
}

func (s *Server) CreateQuorumReplica(rw http.ResponseWriter, req *http.Request) error {
	var replica Replica
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&replica); err != nil {
		return err
	}

	if err := s.c.AddQuorumReplica(replica.Address); err != nil {
		return err
	}

	apiContext.Write(s.getQuorumReplica(apiContext, replica.Address))
	return nil
}

func (s *Server) getReplica(context *api.ApiContext, id string) *Replica {
	for _, r := range s.c.ListReplicas() {
		if r.Address == id {
			return NewReplica(context, r.Address, r.Mode)
		}
	}
	return nil
}

func (s *Server) getQuorumReplica(context *api.ApiContext, id string) *Replica {
	for _, r := range s.c.ListQuorumReplicas() {
		if r.Address == id {
			return NewReplica(context, r.Address, r.Mode)
		}
	}
	return nil
}

func (s *Server) DeleteReplica(rw http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	return s.c.RemoveReplica(id)
}

func (s *Server) UpdateReplica(rw http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var replica Replica
	apiContext := api.GetApiContext(req)
	apiContext.Read(&replica)

	if err := s.c.SetReplicaMode(id, types.Mode(replica.Mode)); err != nil {
		return err
	}

	return s.GetReplica(rw, req)
}

func (s *Server) PrepareRebuildReplica(rw http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	disks, err := s.c.PrepareRebuildReplica(id)
	if err != nil {
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &PrepareRebuildOutput{
		Resource: client.Resource{
			Id:   id,
			Type: "prepareRebuildOutput",
		},
		Disks: disks,
	}

	apiContext.Write(&resp)
	return nil
}

func (s *Server) VerifyRebuildReplica(rw http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	if err := s.c.VerifyRebuildReplica(id); err != nil {
		return err
	}

	return s.GetReplica(rw, req)
}
