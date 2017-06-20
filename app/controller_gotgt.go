package app

import (
	"github.com/openebs/jiva/frontend/gotgt"
)

func init() {
	frontends["gotgt"] = gotgt.New()
}
