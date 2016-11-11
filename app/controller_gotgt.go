package app

import (
	"github.com/openebs/longhorn/frontend/gotgt"
)

func init() {
	frontends["gotgt"] = gotgt.New()
}
