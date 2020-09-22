/*
 Copyright © 2020 The OpenEBS Authors

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
