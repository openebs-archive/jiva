package main

import inject "github.com/openebs/jiva/error-inject"

func (c *testConfig) checkpointTest(replicas []string) error {
	c.verifyRWReplicaCount(3)
	c.verifyCheckpoint(true)
	// Check if checkpoint is set on all the replicas
	verify("VerifyCheckpointSameAtReplicas", c.verifyCheckpointSameAtReplicas(replicas), true)

	// When replica goes down check if checkpoint is removed from controller
	c.StopTestReplica("172.17.0.113")
	c.verifyCheckpoint(false)
	c.RestartTestReplica("172.17.0.113")
	c.verifyRWReplicaCount(3)

	// Set env for 1 replica to crash on receiving setCheckpoint
	// Take snapshot
	// Set checkpoint to this new snapshot
	// Verify that checkpoint is not set at controller, since 1 replica erred out
	inject.Envs["172.17.0.113:9502"]["PANIC_WHILE_SETTING_CHECKPOINT"] = true
	c.createSnapshot("snap-1")
	c.updateCheckpoint()
	c.verifyCheckpoint(false)
	inject.Envs["172.17.0.113:9502"]["PANIC_WHILE_SETTING_CHECKPOINT"] = false
	c.verifyCheckpoint(true)
	return nil
}
