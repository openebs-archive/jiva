package gotgt

import (
	"github.com/Sirupsen/logrus"

	"github.com/rancher/longhorn/types"

	"github.com/openebs/gotgt/pkg/config"
	"github.com/openebs/gotgt/pkg/port"
	"github.com/openebs/gotgt/pkg/port/iscsit"
	"github.com/openebs/gotgt/pkg/scsi"
	_ "github.com/openebs/gotgt/pkg/scsi/backingstore" /* init lib */
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
	rw   types.ReaderWriterAt

	tgtName      string
	lhbsName     string
	cfg          *config.Config
	targetDriver port.SCSITargetService
	stats        port.Stats
}

func (t *goTgt) Startup(name string, frontendIP string, size, sectorSize int64, rw types.ReaderWriterAt) error {
	/*if err := t.Shutdown(); err != nil {
		return err
	}*/

	t.tgtName = "iqn.2016-09.com.openebs.jiva:" + name
	t.lhbsName = "RemBs:" + name
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
				Portal: frontendIP + ":3260",
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
	if err := t.startScsiTarget(t.cfg); err != nil {
		return err
	}

	t.isUp = true

	return nil
}

func (t *goTgt) Shutdown() error {
	if t.Volume != "" {
		t.Volume = ""
	}

	t.stopScsiTarget()
	t.isUp = false

	return nil
}

func (t *goTgt) State() types.State {
	if t.isUp {
		return types.StateUp
	}
	return types.StateDown
}

func (t *goTgt) Stats() types.Stats {
	if !t.isUp {
		return types.Stats{}
	}
	return (types.Stats)(t.targetDriver.Stats())
}

func (t *goTgt) startScsiTarget(cfg *config.Config) error {
	scsiTarget := scsi.NewSCSITargetService()
	var err error
	t.targetDriver, err = iscsit.NewISCSITargetService(scsiTarget)
	if err != nil {
		logrus.Errorf("iscsi target driver error")
		return err
	}
	scsi.InitSCSILUMapEx(t.tgtName, t.Volume, 1, 1, uint64(t.Size), uint64(t.SectorSize), t.rw)
	t.targetDriver.NewTarget(t.tgtName, cfg)
	go t.targetDriver.Run()

	logrus.Infof("SCSI device created")
	return nil
}

func (t *goTgt) stopScsiTarget() error {
	logrus.Infof("stopping target %v ...", t.tgtName)
	t.targetDriver.Stop()
	logrus.Infof("target %v stopped", t.tgtName)
	return nil
}
