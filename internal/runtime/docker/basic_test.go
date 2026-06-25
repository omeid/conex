package docker_test

import (
	"strings"
	"testing"

	"github.com/omeid/conex"
)

func TestMain(m *testing.M) {
	conex.Main(
		m,
		conex.OptRuntimeType(conex.RuntimeDocker),
		conex.OptRequireImage(basicImage),
		conex.OptGoImage("golang:latest"),
	)
}

func TestBasic(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	c.Drop()
}

func TestBasicMulti(t *testing.T) {
	t.Parallel()

	conf := &conex.Config{
		Image: basicImage,
		Cmd:   []string{"sh"},
	}

	c0 := conex.Box(t, conf)
	defer c0.Drop()

	c1 := conex.Box(t, conf)
	defer c1.Drop()

	for suffix, c := range map[string]conex.Container{"_0": c0, "_1": c1} {
		name := c.Name()

		if !strings.HasSuffix(name, suffix) {
			t.Fatalf("Expected suffix: %s, in %s", suffix, name)
		}
	}
}
