package build_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/omeid/conex"
)

// Use a Dockerfile path instead of a registry image.
var buildImage = "Dockerfile"

func init() {
	conex.Require(func() string { return buildImage })
}

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m))
}

func TestBuildBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: buildImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected container to have an address")
	}
	t.Logf("container address: %s", c.Address())
}

func TestBuildContentsExist(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: buildImage,
	})
	defer c.Drop()

	// The Dockerfile creates /conex-build-marker.
	cmd := exec.Command("docker", "exec", c.ID(), "cat", "/conex-build-marker")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to read marker file: %v: %s", err, stderr.String())
	}

	expected := "hello from conex build"
	if !strings.Contains(stdout.String(), expected) {
		t.Fatalf("expected marker to contain %q, got %q", expected, stdout.String())
	}
	t.Logf("marker file: %s", strings.TrimSpace(stdout.String()))
}
