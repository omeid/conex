//go:build !tart

package conex_test

import (
	"github.com/omeid/conex"
)

var (
	basicImage = "alpine:3.20"
)

func init() {
	conex.Require(func() string { return basicImage })
}
