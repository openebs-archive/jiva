package main

import (
	"github.com/docker/docker/pkg/reexec"
	"github.com/openebs/jiva/frontend/gotgt"
	"github.com/openebs/sparse-tools/cli/sfold"
	"github.com/openebs/sparse-tools/cli/ssync"
)

func initializeBackendProcesses() {
	reexec.Register("ssync", ssync.Main)
	reexec.Register("sfold", sfold.Main)

}

func main() {
	frontends["gotgt"] = gotgt.New()
	initializeBackendProcesses()

	testFunctions()
	testCheckpoint()
	//testTwoChild()
}
