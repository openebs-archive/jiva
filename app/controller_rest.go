package app

import (
	"github.com/openebs/jiva/frontend/rest"
)

func init() {
	frontends["rest"] = rest.New()
}
