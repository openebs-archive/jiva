package gotgt

import (
	"net"
	"os"

	"github.com/Sirupsen/logrus"

	"github.com/openebs/longhorn/types"
	"github.com/openebs/longhorn/util"

	"github.com/openebs/gotgt/pkg/config"
	"github.com/openebs/gotgt/pkg/port/iscsit"
	"github.com/openebs/gotgt/pkg/scsi"
	_ "github.com/openebs/gotgt/pkg/scsi/backingstore" /* init lib */
)

func New() types.Frontend {
	return &goTgt{}
}

type goTgt struct {
	Volume     string
	Size       int64
	SectorSize int

	isUp bool
	rw   types.ReaderWriterAt

	tgtName    string
	lhbsName   string
	cfg        *config.Config
	scsiDevice *util.ScsiDevice
}

func (t *goTgt) Startup(name string, size, sectorSize int64, rw types.ReaderWriterAt) error {
	/*if err := t.Shutdown(); err != nil {
		return err
	}*/

	t.tgtName = "iqn.2016-09.com.openebs.jiva:" + name
	t.lhbsName = "RemBs:" + name
	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	var ip string
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			ip = ipv4.String()
			break
			//fmt.Println("IPv4: ", ipv4)
		}
	}
	t.cfg = &config.Config{
		Storages: []config.BackendStorage{
			config.BackendStorage{
				DeviceID: 1000,
				Path:     t.lhbsName,
				Online:   true,
			},
		},
		ISCSIPortals: []config.ISCSIPortalInfo{
			config.ISCSIPortalInfo{
				ID:     0,
				Portal: ip + ":3260",
			},
		},
		ISCSITargets: map[string]config.ISCSITarget{
			t.tgtName: config.ISCSITarget{
				TPGTs: map[string][]uint64{
					"1": []uint64{0},
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
	if err := t.startScsiDevice(t.cfg); err != nil {
		return err
	}

	t.isUp = true

	return nil
}

func (t *goTgt) Shutdown() error {
	if t.Volume != "" {
		t.Volume = ""
	}

	if t.scsiDevice != nil {
		logrus.Infof("Shutdown SCSI device at %v", t.scsiDevice.Device)
		/*if err := t.scsiDevice.Shutdown(); err != nil {
			return err
		}*/
		t.scsiDevice = nil
	}
	t.stopScsiDevice()
	t.isUp = false

	return nil
}

func (t *goTgt) State() types.State {
	if t.isUp {
		return types.StateUp
	}
	return types.StateDown
}

func (t *goTgt) startScsiDevice(cfg *config.Config) error {
	scsiTarget := scsi.NewSCSITargetService()
	targetDriver, err := iscsit.NewISCSITargetService(scsiTarget)
	if err != nil {
		logrus.Errorf("iscsi target driver error")
		return err
	}
	scsi.InitSCSILUMapEx(t.tgtName, t.Volume, 1, 1, uint64(t.Size), uint64(t.SectorSize), t.rw)
	targetDriver.NewTarget(t.tgtName, cfg)
	go targetDriver.Run()

	logrus.Infof("SCSI device created")
	return nil
}

func (t *goTgt) stopScsiDevice() error {
	logrus.Infof("SCSI device stop...")
	return nil
}
