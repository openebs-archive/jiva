package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
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
	prometheusLock sync.Mutex
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
	logrus.Infof("List Replicas")
	apiContext := api.GetApiContext(req)
	resp := client.GenericCollection{}
	s.c.Lock()
	for _, r := range s.c.ListReplicas() {
		resp.Data = append(resp.Data, NewReplica(apiContext, r.Address, r.Mode))
	}
	s.c.Unlock()

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
		logrus.Errorf("Get Replica decodeid %v failed %v", id, err)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}
	logrus.Infof("Get Replica for id %v", id)

	r := s.getReplica(apiContext, id)
	if r == nil {
		logrus.Errorf("Get Replica failed for id %v", id)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	apiContext.Write(r)
	return nil
}

func (s *Server) RegisterReplica(rw http.ResponseWriter, req *http.Request) error {
	var (
		regReplica      RegReplica
		localRevCount   int64
		code            int
		RequestDuration *prometheus.HistogramVec
		RequestCounter  *prometheus.CounterVec
	)
	start := time.Now()
	apiContext := api.GetApiContext(req)

	if err := apiContext.Read(&regReplica); err != nil {
		logrus.Errorf("read in RegReplica failed %v", err)
		return err
	}
	logrus.Infof("Register Replica for address %v", regReplica.Address)

	localRevCount, _ = strconv.ParseInt(regReplica.RevCount, 10, 64)
	local := types.RegReplica{
		Address:  regReplica.Address,
		RevCount: localRevCount,
		RepType:  regReplica.RepType,
		UpTime:   regReplica.UpTime,
		RepState: regReplica.RepState,
	}
	code = http.StatusOK
	rw.WriteHeader(code)
	defer func() {
		RequestDuration = OpenEBSJivaRegestrationRequestDuration
		RequestCounter = OpenEBSJivaRegestrationRequestCounter
		prometheusLock.Lock()
		// This will Display the metrics something similar to
		// the examples given below
		// exp: openebs_jiva_registration_request_duration_seconds{code="200", method="POST"}
		RequestDuration.WithLabelValues(strconv.Itoa(code), req.Method).Observe(time.Since(start).Seconds())

		// This will Display the metrics something similar to
		// the examples given below
		// exp: openebs_jiva_registration_requests_total{code="200", method="POST"}
		RequestCounter.WithLabelValues(strconv.Itoa(code), req.Method).Inc()
		prometheusLock.Unlock()
	}()
	return s.c.RegisterReplica(local)

}

func (s *Server) CreateReplica(rw http.ResponseWriter, req *http.Request) error {
	var replica Replica
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&replica); err != nil {
		logrus.Errorf("read in createReplica failed %v", err)
		return err
	}
	logrus.Infof("Create Replica for address %v", replica.Address)

	if err := s.c.AddReplica(replica.Address); err != nil {
		return err
	}

	r := s.getReplica(apiContext, replica.Address)
	if r == nil {
		logrus.Errorf("createReplica failed for id %v", replica.Address)
		return fmt.Errorf("createReplica failed while getting it")
	}

	apiContext.Write(r)

	return nil
}

func (s *Server) CreateQuorumReplica(rw http.ResponseWriter, req *http.Request) error {
	var replica Replica
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&replica); err != nil {
		logrus.Errorf("read in createQuorumReplica failed %v", err)
		return err
	}
	logrus.Infof("Create QuorumReplica for address %v", replica.Address)

	if err := s.c.AddQuorumReplica(replica.Address); err != nil {
		return err
	}

	r := s.getQuorumReplica(apiContext, replica.Address)
	if r == nil {
		logrus.Errorf("createQuorumReplica failed for id %v", replica.Address)
		return fmt.Errorf("createQuorumReplica failed while getting it")
	}

	apiContext.Write(r)

	return nil
}

func (s *Server) getReplica(context *api.ApiContext, id string) *Replica {
	s.c.Lock()
	defer s.c.Unlock()
	for _, r := range s.c.ListReplicas() {
		if r.Address == id {
			return NewReplica(context, r.Address, r.Mode)
		}
	}
	return nil
}

func (s *Server) getQuorumReplica(context *api.ApiContext, id string) *Replica {
	s.c.Lock()
	defer s.c.Unlock()
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
		logrus.Errorf("Getting ID in DeleteReplica failed %v", err)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}
	logrus.Infof("Delete Replica for id %v", id)

	return s.c.RemoveReplica(id)
}

func (s *Server) UpdateReplica(rw http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	id, err := DencodeID(vars["id"])
	if err != nil {
		logrus.Errorf("Getting ID in UpdateReplica failed %v", err)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}
	logrus.Infof("Update Replica for id %v", id)

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
		logrus.Errorf("Getting ID in PrepareRebuildReplica failed %v", err)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}
	logrus.Infof("Prepare Rebuild Replica for id %v", id)

	disks, err := s.c.PrepareRebuildReplica(id)
	if err != nil {
		logrus.Errorf("Prepare Rebuild Replica failed %v for id %v", err, id)
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
		logrus.Errorf("Error %v in getting id while verifyrebuildreplica", err)
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}
	logrus.Infof("Verify Rebuild Replica for id %v", id)

	if err := s.c.VerifyRebuildReplica(id); err != nil {
		logrus.Errorf("Err %v in verifyrebuildreplica for id %v", err, id)
		return err
	}

	return s.GetReplica(rw, req)
}
