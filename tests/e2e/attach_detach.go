package main

import (
	"fmt"

	"github.com/kubernetes-csi/csi-lib-iscsi/iscsi"
)

func (config *testConfig) attachDisk() (string, error) {
	connector := iscsi.Connector{
		VolumeName: "vol",
		Targets: []iscsi.TargetInfo{
			iscsi.TargetInfo{
				Iqn:    "iqn.2016-09.com.openebs.jiva:" + config.VolumeName,
				Portal: config.ControllerIP,
			},
		},
		Lun:         0,
		Interface:   "default",
		DoDiscovery: true,
	}

	devicePath, err := iscsi.Connect(connector)
	if err != nil {
		return "", err
	}

	if devicePath == "" {
		return "", fmt.Errorf("connect reported success, but no path returned")
	}
	return devicePath, err
}

func (config *testConfig) detachDisk() error {
	return iscsi.Disconnect(
		"iqn.2016-09.com.openebs.jiva:"+config.VolumeName,
		[]string{fmt.Sprintf("%v:%v", config.ControllerIP, 3260)},
	)
}
