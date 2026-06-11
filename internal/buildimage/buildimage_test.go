package buildimage_test

import (
	"testing"

	"github.com/omeid/conex"
)

var buildImage = "../Dockerfile.test-relative"

func TestMain(m *testing.M) {
	conex.Main(
		m,
		conex.OptRequireImage(buildImage),
		conex.OptRunnerType(conex.RunnerDocker), // Explicit runner
	)
}

func TestBuildImageInContainer(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: buildImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected container to have an address")
	}
	t.Logf("container address: %s", c.Address())
}
