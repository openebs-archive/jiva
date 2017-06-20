package app

import (
	"github.com/rancher/longhorn/frontend/gotgt"
)

func init() {
	frontends["gotgt"] = gotgt.New()
}
