package runtime

import (
	"io"
	"time"
)

// Container is a simple interface to a docker
// container.
type Container interface {
	ID() string
	Name() string
	Image() string
	Address() string

	// Drop stops and removes the container. It is automatically called
	// via t.Cleanup, but can be called manually. Calling it multiple
	// times is a no-op.
	Drop()

	Wait(port string, timeout time.Duration) error // Wait for the port to respond to tcp/udp.

	// Exec creates a command to run inside the container.
	Exec(cmd ...string) *Cmd

	Logs(stdout io.Writer, stderr io.Writer) error
}

// Config contains the configuration data about a container.
type Config struct {
	Image      string   // Name of the image as it was passed by the operator (e.g. could be symbolic)
	Env        []string // List of environment variable to set in the container
	Entrypoint []string // Entrypoint to run in the container
	Cmd        []string // Command to run when starting the container
	Hostname   string   // Hostname
	Domainname string   // Domainname
	User       string   // User that will run the command(s) inside the container, also support user:group
	Expose     []string // Ports to expose, supports the docker command line style syntax proto/port or just port which defaults to tcp
	Privileged bool     // Run the container in privileged mode
	Binds      []string // Volume binds (e.g. "/host/path:/container/path")
}
