package conex

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/term"
)

func init() {
	var _ Runner = (*NativeRunner)(nil)
	var _ Container = (*nativeContainer)(nil)
}

// NativeRunner runs tests on the host machine and connects to containers
// via their IP addresses. This requires native Docker (not Docker for Mac).
type NativeRunner struct {
	config *RunnerConfig
}

// NewNativeRunner creates a new native runner.
func NewNativeRunner(config *RunnerConfig) *NativeRunner {
	return &NativeRunner{config: config}
}

// Run executes the tests directly on the host.
func (r *NativeRunner) Run(m *testing.M) int {
	return m.Run()
}

// Box creates a container and returns a Container that uses the container's
// direct IP address for connections.
func (r *NativeRunner) Box(t testing.TB, conf *Config, name string) Container {
	// cname is a simple canonical name that includes the
	// container image name and params.
	cname := conf.Image
	if len(conf.Entrypoint) != 0 {
		cname = cname + " entrypoint: " + strings.Join(conf.Entrypoint, " ")
	}
	if len(conf.Cmd) != 0 {
		cname = cname + " cmd: " + strings.Join(conf.Cmd, " ")
	}

	Logf(t, "", "creating (%s) as %s", cname, name)

	exposedPorts := make(network.PortSet)
	for _, port := range conf.Expose {
		exposedPorts[network.MustParsePort(port)] = struct{}{}
	}

	cresp, err := r.config.Client.ContainerCreate(
		t.Context(),
		client.ContainerCreateOptions{
			Config: &container.Config{
				Image:        conf.Image,
				Entrypoint:   conf.Entrypoint,
				Cmd:          conf.Cmd,
				Env:          conf.Env,
				Hostname:     conf.Hostname,
				Domainname:   conf.Domainname,
				User:         conf.User,
				Tty:          term.IsTerminal(os.Stdout.Fd()),
				ExposedPorts: exposedPorts,
			},
			HostConfig: &container.HostConfig{
				Privileged: conf.Privileged,
				Binds:      conf.Binds,
			},
			Name: name,
		},
	)
	if err != nil {
		fatalf(t, "Failed to create container: %s", err)
	}

	_, err = r.config.Client.ContainerStart(t.Context(), cresp.ID, client.ContainerStartOptions{})
	if err != nil {
		fatalf(t, "Failed to start container: %v", err)
	}

	Logf(t, "", "started (%s) as %s", cname, name)

	cjsonResult, err := r.config.Client.ContainerInspect(t.Context(), cresp.ID, client.ContainerInspectOptions{})
	if err != nil {
		fatalf(t, "Failed to inspect: %v", err)
	}

	return &nativeContainer{
		json:   cjsonResult.Container,
		client: r.config.Client,
		t:      t,
	}
}

// nativeContainer implements Container for native Docker access via IP.
type nativeContainer struct {
	json     container.InspectResponse
	client   client.APIClient
	t        testing.TB
	dropOnce sync.Once
}

func (c *nativeContainer) ID() string {
	return c.json.ID
}

func (c *nativeContainer) Image() string {
	return c.json.Image
}

func (c *nativeContainer) Name() string {
	return c.json.Name
}

func (c *nativeContainer) Address() string {
	// For newer Docker versions, the IP is in the Networks map
	// Try to find an IP in any network (typically "bridge")
	for _, network := range c.json.NetworkSettings.Networks {
		if network.IPAddress.IsValid() {
			return network.IPAddress.String()
		}
	}

	return ""
}

func (c *nativeContainer) Drop() {
	c.dropOnce.Do(func() {
		// Try to stop the container, but don't fail if it's already stopped
		timeout := 10
		_, err := c.client.ContainerStop(context.Background(), c.json.ID, client.ContainerStopOptions{Timeout: &timeout})
		if err != nil {
			// Check if the error is because the container is not running
			// In that case, we can proceed to remove it
			if !strings.Contains(err.Error(), "is not running") &&
				!strings.Contains(err.Error(), "Container not running") {
				c.t.Log("failed to stop container: ", c.json.ID)
				c.t.Fatal(err)
			}
		}

		_, err = c.client.ContainerRemove(context.Background(), c.json.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			c.t.Fatal(err)
		}
	})
}

func (c *nativeContainer) Wait(port string, timeout time.Duration) error {
	err := wait(c.Address(), port, timeout)
	if err != nil && testing.Verbose() {
		c.t.Logf("=== Container %s Logs ===", c.Name())
		_ = c.Logs(os.Stdout, os.Stderr)
		c.t.Log("=========================")
	}
	return err
}

func (c *nativeContainer) Logs(stdout io.Writer, stderr io.Writer) error {
	reader, err := c.client.ContainerLogs(c.t.Context(), c.json.ID, client.ContainerLogsOptions{
		ShowStdout: stdout != nil,
		ShowStderr: stderr != nil,
		Follow:     false,
	})
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	if c.json.Config != nil && c.json.Config.Tty {
		w := stdout
		if w == nil {
			w = stderr
		}
		if w == nil {
			w = io.Discard
		}
		_, err := io.Copy(w, reader)
		return err
	}

	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	return err
}

func (c *nativeContainer) Exec(cmd ...string) *Cmd {
	return newDockerCmd(c.t, c.client, c.json.ID, cmd)
}
