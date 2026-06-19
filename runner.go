package conex

import (
	"context"
	"testing"
)

// runner is an abstraction that allows running tests either natively on the host
// or inside a Docker container. This enables conex to work on systems where
// container IPs are not directly accessible from the host (e.g., Docker for Mac).
type runner interface {
	// Run executes the test suite. The runner is responsible for setting up
	// any necessary environment and executing m.Run().
	Run(m *testing.M) int

	// Box creates a container and returns a Container interface.
	// The implementation determines how the container is accessed (direct IP vs network alias).
	Box(t testing.TB, conf *Config, name string) Container

	Pull(ctx context.Context, image string) error
	Ensure(ctx context.Context, image string) (string, error)
	Build(ctx context.Context, image string, tag string) error
}

// RunnerConfig holds configuration for creating a runner.
type runnerConfig struct {
	Name       string // prefix for container names
	PullImages bool
	Images     []string
	RetCode    int
	GoImage    string // Go image for running tests in Docker runner
}
