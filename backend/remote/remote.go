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

	inject "github.com/openebs/jiva/error-inject"
	"github.com/openebs/jiva/replica/rest"
	"github.com/openebs/jiva/rpc"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
)

var (
	pingInveral   = 2 * time.Second
	timeout       = 30 * time.Second
	requestBuffer = 1024
)

var _ types.Backend = (*Remote)(nil)

func New() types.BackendFactory {
	return &Factory{}
}

type Factory struct {
}

type Remote struct {
	types.IOs
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

	client := r.httpClient
	// timeout of zero means there is no timeout
	// Since preload can take time while opening
	// replica, we don't know the exact time elapsed
	// to complete the request.
	if action == "open" {
		client.Timeout = 0
	}

	resp, err := client.Do(req)
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
	volUsage.RevisionCounter, _ = strconv.ParseInt(Details.RevisionCounter, 10, 64)
	volUsage.UsedLogicalBlocks, _ = strconv.ParseInt(Details.UsedLogicalBlocks, 10, 64)
	volUsage.UsedBlocks, _ = strconv.ParseInt(Details.UsedBlocks, 10, 64)
	volUsage.SectorSize, _ = strconv.ParseInt(Details.SectorSize, 10, 64)

	return volUsage, err
}

// SetReplicaMode ...
func (r *Remote) SetReplicaMode(mode types.Mode) error {
	var m string
	logrus.Infof("Set replica mode of %s to : %v", r.Name, mode)
	if mode == types.RW {
		m = "RW"
	} else if mode == types.WO {
		m = "WO"
	} else {
		return fmt.Errorf("invalid mode string %v", mode)
	}
	return r.doAction("setreplicamode", &map[string]string{"mode": m})
}

func (r *Remote) SetRevisionCounter(counter int64) error {
	logrus.Infof("Set revision counter of %s to : %v", r.Name, counter)
	localRevCount := strconv.FormatInt(counter, 10)
	return r.doAction("setrevisioncounter", &map[string]string{"counter": localRevCount})
}

// SetSyncCounter set the sync count of the replica
func (r *Remote) SetSyncCounter(counter int64) error {
	logrus.Infof("Set sync counter of %s to : %v", r.Name, counter)
	localSyncCount := strconv.FormatInt(counter, 10)
	return r.doAction("setsynccounter", &map[string]string{"counter": localSyncCount})
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

	remote := rpc.NewClient(conn, r.closeChan)
	r.IOs = remote

	if err := r.open(); err != nil {
		logrus.Errorf("Failed to open replica, error: %v", err)
		remote.Close()
		return nil, err
	}

	go r.monitorPing(remote)

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
