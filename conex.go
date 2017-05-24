// Package conex provides easy to use Docker Integration with Testing.
package conex

import "testing"

// We keep logger here because the filename is shown along with the logs,
// this means that conex.go is put right before each log in tests which
// makes the source of the log more clear to the user.
func logf(t testing.TB, f string, args ...interface{}) {
	t.Logf(f, args...)
}

//Same story as above.
func fatalf(t testing.TB, f string, args ...interface{}) {
	t.Fatalf(f, args...)
}

// Manager is the conex container manager.
type Manager interface {
	Run(m *testing.M, images ...string) int
	Box(t testing.TB, config *Config) Container
}

// Container is a simple interface to a docker
// container.
type Container interface {
	ID() string
	Name() string
	Image() string
	Address() string

	Drop()

	//TODO: Yo.
	// Ports() []string
}

// Config contains the configuration data about a container.
type Config struct {
	Image      string   // Name of the image as it was passed by the operator (e.g. could be symbolic)
	Env        []string // List of environment variable to set in the container
	Cmd        []string // Command to run when starting the container
	Hostname   string   // Hostname
	Domainname string   // Domainname
	User       string   // User that will run the command(s) inside the container, also support user:group
	Expose     []string // Ports to expose, supports the docker command line style syntax proto/port or just port which defaults to tcp
}
