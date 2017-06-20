// +build tcmu

package app

import (
	"github.com/openebs/jiva/frontend/tcmu"
)

func init() {
	frontends["tcmu"] = tcmu.New()
}
