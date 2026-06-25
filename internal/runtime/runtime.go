package runtime

import (
	"context"
	"testing"

	"github.com/omeid/conex/runtime"
)

// Runtime is an abstraction that allows running tests in different environments
// (e.g., natively on the host or inside a Docker container).
type Runtime interface {
	// Run executes the test suite. The runtime is responsible for setting up
	// any necessary environment and executing m.Run().
	Run(m *testing.M) int

	// Box creates a container and returns a Container interface.
	Box(t testing.TB, conf *runtime.Config, name string) runtime.Container

	Pull(ctx context.Context, image string) error
	Ensure(ctx context.Context, image string) (string, error)
	Build(ctx context.Context, image string, tag string) error
}

// Config holds configuration for creating a runtime.
type Config struct {
	Name       string // prefix for container names
	PullImages bool
	Images     []string
	RetCode    int
	GoImage    string // Go image for running tests in Docker runtime
}
