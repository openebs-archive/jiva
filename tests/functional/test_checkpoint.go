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
	"time"

	inject "github.com/openebs/jiva/error-inject"
	"github.com/sirupsen/logrus"
)

func testCheckpoint() {
	logrus.Infof("Test Checkpoint")
	replicas := []string{"172.18.0.111", "172.18.0.112", "172.18.0.113"}
	c := buildConfig("172.18.0.110", replicas)
	// Start controller
	go func() {
		c.startTestController(c.ControllerIP)
	}()
	time.Sleep(5 * time.Second)
	c.startReplicas()

	go c.MonitorReplicas()

	verify("CheckpointTest", c.checkpointTest(replicas), nil)

	c.Stop = true
	c.stopReplicas()
	c.cleanReplicaDirs()
}

func (c *testConfig) checkpointTest(replicas []string) error {
	c.verifyRWReplicaCount(3)
	c.verifyCheckpoint(true)
	// Check if checkpoint is set on all the replicas
	verify("VerifyCheckpointSameAtReplicas", c.verifyCheckpointSameAtReplicas(replicas), true)

	// When replica goes down check if checkpoint is removed from controller
	c.StopTestReplica("172.18.0.113")
	c.verifyCheckpoint(false)
	c.RestartTestReplica("172.18.0.113")
	c.verifyRWReplicaCount(3)

	// Set env for 1 replica to crash on receiving setCheckpoint
	// Take snapshot
	// Set checkpoint to this new snapshot
	// Verify that checkpoint is not set at controller, since 1 replica erred out
	inject.Envs["172.18.0.113:9502"]["PANIC_WHILE_SETTING_CHECKPOINT"] = true
	c.createSnapshot("snap-1")
	c.updateCheckpoint()
	c.verifyCheckpoint(false)
	inject.Envs["172.18.0.113:9502"]["PANIC_WHILE_SETTING_CHECKPOINT"] = false
	c.verifyCheckpoint(true)
	return nil
}
