package main

import (
	"strconv"
	"time"
)

func (config *TestConfig) SnapshotCreateDelete() {
	config.InsertThread()
	defer config.ReleaseThread()
	ctrlClient := getControllerClient(config.ControllerIP)
	i := 0
	for {
		ctrlClient.Snapshot("snap-" + strconv.Itoa(i))
		time.Sleep(5 * time.Second)
		ctrlClient.DeleteSnapshot("snap-" + strconv.Itoa(i))
		i++
		if config.Stop {
			return
		}
	}
}
