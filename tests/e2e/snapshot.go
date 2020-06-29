package main

import (
	"strconv"
	"time"
)

func (config *testConfig) snapshotCreateDelete() {
	config.insertThread()
	defer config.releaseThread()
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
