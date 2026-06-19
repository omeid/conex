//go:build !tart

package containertest_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/omeid/conex"
)

const testImage = "alpine"

func TestMain(m *testing.M) {
	conex.Main(
		m,
		conex.OptRequireImage(testImage),
		conex.OptRunnerType(conex.RunnerDocker), // Force container runtime
		conex.OptGoImage("golang:latest"),
	)
}

func TestConexInsideContainer(t *testing.T) {
	// 1. Verify we are indeed running inside the container runner
	if os.Getenv("CONEX_INSIDE_DOCKER") != "1" {
		t.Fatal("Expected CONEX_INSIDE_DOCKER to be '1'")
	}

	// 2. Start a container on the shared network
	c := conex.Box(t, &conex.Config{
		Image: testImage,
		Cmd:   []string{"sleep", "100"},
	})

	if c.Address() == "" {
		t.Fatal("Expected container to have an address")
	}

	// 3. Verify exec functionality works inside container-in-container
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := c.Exec("echo", "hello", "conex")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run cmd: %v (stderr: %s)", err, stderr.String())
	}

	got := strings.TrimSpace(stdout.String())
	if got != "hello conex" {
		t.Errorf("Expected 'hello conex', got %q", got)
	}
}
