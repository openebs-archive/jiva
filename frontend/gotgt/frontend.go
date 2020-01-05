package gotgt

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/openebs/jiva/controller"
	"github.com/openebs/jiva/types"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/gostor/gotgt/pkg/api"
	"github.com/gostor/gotgt/pkg/config"
	"github.com/gostor/gotgt/pkg/port/iscsit" /* init lib */
	"github.com/gostor/gotgt/pkg/scsi"
	_ "github.com/gostor/gotgt/pkg/scsi/backingstore" /* init lib */
	"github.com/gostor/gotgt/pkg/scsi/backingstore/remote"
)

/*New is called on module load */
func New() types.Frontend {
	return &goTgt{}
}

type goTgt struct {
	Volume     string
	Size       int64
	SectorSize int

	isUp bool
	rw   types.IOs

	tgtName      string
	lhbsName     string
	clusterIP    string
	cfg          *config.Config
	targetDriver scsi.SCSITargetDriver
	stats        scsi.Stats
}

var _ types.Frontend = (*goTgt)(nil)
var _ api.RemoteBackingStore = (*controller.Controller)(nil)

func initializeSCSITarget(size int64) {
	iscsit.EnableStats = true
	scsi.SCSIVendorID = "OPENEBS"
	scsi.SCSIProductID = "JIVA"
	scsi.SCSIID = "iqn.2016-09.com.jiva.openebs:iscsi-tgt"
	scsi.EnableORWrite16 = false
	scsi.EnablePersistentReservation = false
	scsi.EnableMultipath = false
	remote.Size = uint64(size)
}

// Startup starts iscsi target server
func (t *goTgt) Startup(name string, frontendIP string, clusterIP string, size, sectorSize int64, rw types.IOs) error {
	initializeSCSITarget(size)

	if frontendIP == "" {
		host, _ := os.Hostname()
		addrs, _ := net.LookupIP(host)
		for _, addr := range addrs {
			if ipv4 := addr.To4(); ipv4 != nil {
				frontendIP = ipv4.String()
				if frontendIP == "127.0.0.1" {
					continue
				}
				break
			}
		}
	}

	t.tgtName = "iqn.2016-09.com.openebs.jiva:" + name
	t.lhbsName = "RemBs:" + name
	t.cfg = &config.Config{
		Storages: []config.BackendStorage{
			{
				DeviceID: 1000,
				Path:     t.lhbsName,
				Online:   true,
			},
		},
		ISCSIPortals: []config.ISCSIPortalInfo{
			{
				ID:     0,
				Portal: frontendIP + ":3260",
			},
		},
		ISCSITargets: map[string]config.ISCSITarget{
			t.tgtName: {
				TPGTs: map[string][]uint64{
					"1": {0},
				},
				LUNs: map[string]uint64{
					"1": uint64(1000),
				},
			},
		},
	}

	t.Volume = name
	t.Size = size
	t.SectorSize = int(sectorSize)
	t.rw = rw
	t.clusterIP = clusterIP
	logrus.Info("Start SCSI target")
	if err := t.startScsiTarget(t.cfg); err != nil {
		return err
	}

	t.isUp = true

	return nil
}

// Shutdown stop scsi target
func (t *goTgt) Shutdown() error {
	if t.Volume != "" {
		t.Volume = ""
	}

	if err := t.stopScsiTarget(); err != nil {
		logrus.Fatalf("Failed to stop scsi target, err: %v", err)
	}
	t.isUp = false

	return nil
}

// State provides info whether scsi target is up or down
func (t *goTgt) State() types.State {
	if t.isUp {
		return types.StateUp
	}
	return types.StateDown
}

// Stats get target stats from the scsi target
func (t *goTgt) Stats() types.Stats {
	if !t.isUp {
		return types.Stats{}
	}
	return (types.Stats)(t.targetDriver.Stats())
}

//  Resize is called to resize the volume
func (t *goTgt) Resize(size uint64) error {
	if !t.isUp {
		return fmt.Errorf("Volume is not up")
	}
	return t.targetDriver.Resize(size)
}

func (t *goTgt) startScsiTarget(cfg *config.Config) error {
	var err error
	id := uuid.NewV4()
	uid := binary.BigEndian.Uint64(id[:8])
	err = scsi.InitSCSILUMapEx(&config.BackendStorage{
		DeviceID:         uid,
		Path:             "RemBs:" + t.tgtName,
		Online:           true,
		BlockShift:       9,
		ThinProvisioning: true,
	},
		t.tgtName, uint64(0), t.rw)
	if err != nil {
		return err
	}
	scsiTarget := scsi.NewSCSITargetService()
	t.targetDriver, err = scsi.NewTargetDriver("iscsi", scsiTarget)
	if err != nil {
		logrus.Errorf("iscsi target driver error")
		return err
	}
	t.targetDriver.NewTarget(t.tgtName, cfg)
	t.targetDriver.SetClusterIP(t.clusterIP)
	go t.targetDriver.Run()

	logrus.Infof("SCSI device created")
	return nil
}

func (t *goTgt) stopScsiTarget() error {
	if t.targetDriver == nil {
		return nil
	}
	logrus.Infof("Stopping target %v ...", t.tgtName)
	if err := t.targetDriver.Close(); err != nil {
		return err
	}
	logrus.Infof("Target %v stopped", t.tgtName)
	return nil
}
