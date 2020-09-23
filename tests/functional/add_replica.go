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
	"github.com/openebs/jiva/replica"
	"github.com/openebs/jiva/sync"
	"github.com/sirupsen/logrus"
)

func addReplica(replica string) error {
	task := sync.NewTask(replica + ":9501")
	return task.AddReplica(replica, nil)
}
func autoAddReplica(s *replica.Server, frontendIP string, replica string, replicaType string) error {
	var err error
	url := "http://" + frontendIP + ":9501"
	task := sync.NewTask(url)
	if err = task.AddReplica(replica, s); err != nil {
		logrus.Errorf("jiva.volume.replica.add.failure rname:%v err: %v", replica, err)
		return err
	}
	logrus.Infof("jiva.volume.replica.add.success rname: %v", replica)
	return nil
}
