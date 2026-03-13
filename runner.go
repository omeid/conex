package conex

import (
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

// Runner is an abstraction that allows running tests either natively on the host
// or inside a Docker container. This enables conex to work on systems where
// container IPs are not directly accessible from the host (e.g., Docker for Mac).
type Runner interface {
	// Run executes the test suite. The runner is responsible for setting up
	// any necessary environment and executing m.Run().
	Run(m *testing.M) int

	// Box creates a container and returns a Container interface.
	// The implementation determines how the container is accessed (direct IP vs network alias).
	Box(t testing.TB, conf *Config, name string) Container
}

// RunnerConfig holds configuration for creating a runner.
type RunnerConfig struct {
	Client     *docker.Client
	Name       string // prefix for container names
	PullImages bool
	Images     []string
	RetCode    int
	Counter    *counter
	GoImage    string // Go image for running tests in Docker runner
}
