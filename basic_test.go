package conex_test

import (
	"os"
	"strings"
	"testing"

	"github.com/omeid/conex"
)

const basicImage = "alpine"

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m, basicImage))
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
