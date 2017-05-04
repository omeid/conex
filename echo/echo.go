package echo

import (
	"fmt"
	"testing"

	"github.com/omeid/conex"
	"github.com/omeid/echo"
	echoHttp "github.com/omeid/echo/http"
)

// Image to use for the box.
var Image = "omeid/echo:http"

func init() {
	conex.Require(func() string { return Image })
}

// Box returns an echo client connect to an echo container based on
// your provided tags.
func Box(t testing.TB, reverse bool) (echo.Echo, conex.Container) {
	params := []string{}

	if reverse {
		params = append(params, "-reverse")
	}

	c := conex.Box(t, &conex.Config{Image: Image, Cmd: params})

	addr := fmt.Sprintf("http://%s:3000", c.Address())

	e, err := echoHttp.NewClient(addr)
	if err != nil {
		t.Fatal(err)
	}

	return e, c
}
