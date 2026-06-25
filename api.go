// Package conex provides easy to use Docker Integration with Testing.
package conex

import (
	"testing"

	"github.com/omeid/conex/log"
	"github.com/omeid/conex/runtime"
)

// Manager is the conex container manager.
type Manager interface {
	Run(m *testing.M, images ...string) int
	Box(t testing.TB, config *Config) Container
}

// Config contains the configuration data about a container.
type Config = runtime.Config

// Container is a simple interface to a docker container.
type Container = runtime.Container

// Logf logs directly to stdout to avoid the test file and line number prefix.
// If a testing.TB is provided, it buffers the logs ONLY if there are parallel
// tests running, to prevent interleaving while allowing real-time logs otherwise.
func Logf(t testing.TB, plugin string, fmt string, args ...any) {
	log.Logf(t, plugin, fmt, args...)
}
