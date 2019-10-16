package util

import (
	"fmt"
	"testing"
)

func TestFilter(t *testing.T) {

	chain := []string{"snap1", "snap7", "snap10"}
	snapshots := []string{"snap1", "snap2", "snap3", "snap4", "snap5"}
	snapshots = Filter(snapshots, func(i string) bool {
		return Contains(chain, i)
	})

	fmt.Println(snapshots)
}
