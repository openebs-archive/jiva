/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

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

package controller

import (
	"fmt"
	"strings"

	"github.com/openebs/jiva/replica/client"
	"github.com/openebs/jiva/types"
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
)

func (c *Controller) Revert(name string) error {
	doable := false
	rw := false
	wo := false
	for _, rep := range c.replicas {
		//BUG: Need to deal with replica in rebuilding(WO) mode.
		if rep.Mode == types.RW {
			rw = true
		} else if rep.Mode == types.WO {
			wo = true
		}
	}
	if rw && !wo {
		doable = true
	}

	if !doable {
		return fmt.Errorf("Not valid state to revert, rebuilding in process or all replica are in ERR state")
	}

	clients, name, err := c.clientsAndSnapshot(name)
	if err != nil {
		return err
	}

	if err := c.shutdownFrontend(); err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	minimalSuccess := false
	now := util.Now()
	for address, client := range clients {
		logrus.Infof("Reverting to snapshot %s on %s at %s", name, address, now)
		if err := client.Revert(name, now); err != nil {
			logrus.Errorf("Error on reverting to %s on %s: %v", name, address, err)
			c.setReplicaModeNoLock(address, types.ERR)
		} else {
			minimalSuccess = true
			logrus.Infof("Reverting to snapshot %s on %s successed", name, address)
		}
	}

	if !minimalSuccess {
		return fmt.Errorf("Fail to revert to %v on all replicas", name)
	}

	return c.startFrontend()
}

func (c *Controller) clientsAndSnapshot(name string) (map[string]*client.ReplicaClient, string, error) {
	clients := map[string]*client.ReplicaClient{}

	for _, replica := range c.replicas {
		if replica.Mode == types.WO {
			return nil, "", fmt.Errorf("Cannot revert %s during rebuilding process", replica.Address)
		}
		if replica.Mode != types.RW {
			continue
		}

		if !strings.HasPrefix(replica.Address, "tcp://") {
			return nil, "", fmt.Errorf("Backend %s does not support revert", replica.Address)
		}

		repClient, err := client.NewReplicaClient(replica.Address)
		if err != nil {
			return nil, "", err
		}

		_, err = repClient.GetReplica()
		if err != nil {
			return nil, "", err
		}

		/*
			found := ""
			for _, snapshot := range rep.Chain {
				if snapshot == name {
					found = name
					break
				}
				fullName := "volume-snap-" + name + ".img"
				if snapshot == fullName {
					found = fullName
					break
				}
			}

			if found == "" {
				return nil, "", fmt.Errorf("Failed to find snapshot %s on %s", name, replica)
			}

			name = found
		*/
		clients[replica.Address] = repClient
	}
	name = "volume-snap-" + name + ".img"

	return clients, name, nil
}
