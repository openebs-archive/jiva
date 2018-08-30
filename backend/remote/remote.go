package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/rpc"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
)

var (
	pingInveral   = 2 * time.Second
	timeout       = 30 * time.Second
	requestBuffer = 1024
)

func New() types.BackendFactory {
	return &Factory{}
}

type Factory struct {
}

type Remote struct {
	types.ReaderWriterAt
	Name        string
	pingURL     string
	replicaURL  string
	httpClient  *http.Client
	closeChan   chan struct{}
	monitorChan types.MonitorChannel
}

func (r *Remote) Close() error {
	logrus.Infof("Closing: %s", r.Name)
	r.StopMonitoring()
	return nil
}

func (r *Remote) open() error {
	logrus.Infof("Opening: %s", r.Name)
	return r.doAction("open", nil)
}

func (r *Remote) Snapshot(name string, userCreated bool, created string) error {
	logrus.Infof("Snapshot: %s %s UserCreated %v Created at %v",
		r.Name, name, userCreated, created)
	return r.doAction("snapshot",
		&map[string]interface{}{
			"name":        name,
			"usercreated": userCreated,
			"created":     created,
		})
}

func (r *Remote) Resize(name string, size string) error {
	logrus.Infof("Resize: %s to %s",
		name, size)
	return r.doAction("resize",
		&map[string]interface{}{
			"name": name,
			"size": size,
		})
}
func (r *Remote) SetRebuilding(rebuilding bool) error {
	logrus.Infof("SetRebuilding: %v", rebuilding)
	return r.doAction("setrebuilding", &map[string]bool{"rebuilding": rebuilding})
}

func (r *Remote) doAction(action string, obj interface{}) error {
	body := io.Reader(nil)
	if obj != nil {
		buffer := &bytes.Buffer{}
		if err := json.NewEncoder(buffer).Encode(obj); err != nil {
			return err
		}
		body = buffer
	}

	req, err := http.NewRequest("POST", r.replicaURL+"?action="+action, body)
	if err != nil {
		return err
	}

	if obj != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad status: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (r *Remote) Size() (int64, error) {
	replica, err := r.info()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(replica.Size, 10, 0)
}

func (r *Remote) SectorSize() (int64, error) {
	replica, err := r.info()
	if err != nil {
		return 0, err
	}
	return replica.SectorSize, nil
}

func (r *Remote) RemainSnapshots() (int, error) {
	replica, err := r.info()
	if err != nil {
		return 0, err
	}
	if replica.State != "open" && replica.State != "dirty" && replica.State != "rebuilding" {
		return 0, fmt.Errorf("Invalid state %v for counting snapshots", replica.State)
	}
	return replica.RemainSnapshots, nil
}

func (r *Remote) GetRevisionCounter() (int64, error) {
	replica, err := r.info()
	if err != nil {
		return 0, err
	}
	if replica.State != "open" && replica.State != "dirty" {
		return 0, fmt.Errorf("Invalid state %v for getting revision counter", replica.State)
	}
	counter, _ := strconv.ParseInt(replica.RevisionCounter, 10, 64)
	return counter, nil
}

func (r *Remote) GetCloneStatus() (string, error) {
	replica, err := r.info()
	if err != nil {
		return "", err
	}
	return replica.CloneStatus, nil
}

func (r *Remote) GetVolUsage() (types.VolUsage, error) {
	var Details rest.VolUsage
	var volUsage types.VolUsage

	req, err := http.NewRequest("GET", r.replicaURL+"/volusage", nil)
	if err != nil {
		return volUsage, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return volUsage, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return volUsage, fmt.Errorf("Bad status: %d %s", resp.StatusCode, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&Details)
	volUsage.UsedLogicalBlocks, _ = strconv.ParseInt(Details.UsedLogicalBlocks, 10, 64)
	volUsage.UsedBlocks, _ = strconv.ParseInt(Details.UsedBlocks, 10, 64)
	volUsage.SectorSize, _ = strconv.ParseInt(Details.SectorSize, 10, 64)

	return volUsage, err
}

func (r *Remote) SetRevisionCounter(counter int64) error {
	logrus.Infof("Set revision counter of %s to : %v", r.Name, counter)
	localRevCount := strconv.FormatInt(counter, 10)
	return r.doAction("setrevisioncounter", &map[string]string{"counter": localRevCount})
}

func (r *Remote) UpdatePeerDetails(replicaCount int, quorumReplicaCount int) error {
	logrus.Infof("Update peer details of %s rc:%d qc:%d", r.Name, replicaCount, quorumReplicaCount)
	return r.doAction("updatepeerdetails",
		&map[string]interface{}{
			"replicacount":       replicaCount,
			"quorumreplicacount": quorumReplicaCount,
		})
}

func (r *Remote) info() (rest.Replica, error) {
	var replica rest.Replica
	req, err := http.NewRequest("GET", r.replicaURL, nil)
	if err != nil {
		return replica, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return replica, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return replica, fmt.Errorf("Bad status: %d %s", resp.StatusCode, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&replica)
	return replica, err
}

// Create Remote with address given string, returns backend and error
func (rf *Factory) Create(address string) (types.Backend, error) {
	// No need to add prints in this function.
	// Make sure caller of this takes care of printing error
	logrus.Infof("Connecting to remote: %s", address)

	controlAddress, dataAddress, _, err := util.ParseAddresses(address)
	if err != nil {
		return nil, err
	}

	r := &Remote{
		Name:       address,
		replicaURL: fmt.Sprintf("http://%s/v1/replicas/1", controlAddress),
		pingURL:    fmt.Sprintf("http://%s/ping", controlAddress),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		// We don't want sender to wait for receiver, because receiver may
		// has been already notified
		closeChan:   make(chan struct{}, 5),
		monitorChan: make(types.MonitorChannel, 5),
	}

	replica, err := r.info()
	if err != nil {
		return nil, err
	}

	if replica.State != "closed" {
		return nil, fmt.Errorf("Replica must be closed, Can not add in state: %s", replica.State)
	}

	conn, err := net.Dial("tcp", dataAddress)
	if err != nil {
		return nil, err
	}

	rpc := rpc.NewClient(conn)
	r.ReaderWriterAt = rpc

	if err := r.open(); err != nil {
		return nil, err
	}

	go r.monitorPing(rpc)

	return r, nil
}

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
	inject.AddTimeout()
	return r.doAction("start", &map[string]string{"Action": action})
}

func (r *Remote) monitorPing(client *rpc.Client) {
	ticker := time.NewTicker(pingInveral)
	defer ticker.Stop()

	for {
		select {
		case <-r.closeChan:
			r.monitorChan <- nil
			return
		case <-ticker.C:
			if err := client.Ping(); err != nil {
				client.SetError(err)
				r.monitorChan <- err
				return
			}
		}
	}
}

func (r *Remote) GetMonitorChannel() types.MonitorChannel {
	return r.monitorChan
}

func (r *Remote) StopMonitoring() {
	logrus.Infof("stopping monitoring %v", r.Name)
	r.closeChan <- struct{}{}
}
