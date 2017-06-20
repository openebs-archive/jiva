package app

import (
	"github.com/openebs/jiva/frontend/tgt"
)

func init() {
	frontends["tgt"] = tgt.New()
}
