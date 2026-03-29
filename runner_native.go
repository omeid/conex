package conex

import (
	"context"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
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
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	logf(t, "creating (%s) as %s", cname, name)

	exposedPorts := make(map[docker.Port]struct{})
	for _, port := range conf.Expose {
		exposedPorts[docker.Port(port)] = struct{}{}
	}

	c, err := r.config.Client.CreateContainer(
		docker.CreateContainerOptions{
			Name: name,
			Config: &docker.Config{
				Image:        conf.Image,
				Cmd:          conf.Cmd,
				Env:          conf.Env,
				Hostname:     conf.Hostname,
				Domainname:   conf.Domainname,
				User:         conf.User,
				Tty:          true,
				ExposedPorts: exposedPorts,
			},
			HostConfig: &docker.HostConfig{
				Privileged: conf.Privileged,
				Binds:      conf.Binds,
			},
		},
	)
	if err != nil {
		fatalf(t, "Failed to create container: %s", err)
	}

	err = r.config.Client.StartContainer(c.ID, nil)
	if err != nil {
		fatalf(t, "Failed to start container: %v", err)
	}

	logf(t, "started (%s) as %s", cname, name)

	cjson, err := r.config.Client.InspectContainer(c.ID)

	if err != nil {
		fatalf(t, "Failed to inspect: %v", err)
	}

	return &nativeContainer{
		json:   cjson,
		client: r.config.Client,
		t:      t,
	}
}

// nativeContainer implements Container for native Docker access via IP.
type nativeContainer struct {
	json   *docker.Container
	client *docker.Client
	t      testing.TB
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
	// First try the top-level IPAddress (older Docker versions)
	if c.json.NetworkSettings.IPAddress != "" {
		return c.json.NetworkSettings.IPAddress
	}

	// For newer Docker versions, the IP is in the Networks map
	// Try to find an IP in any network (typically "bridge")
	for _, network := range c.json.NetworkSettings.Networks {
		if network.IPAddress != "" {
			return network.IPAddress
		}
	}

	return ""
}

func (c *nativeContainer) Drop() {
	// Try to stop the container, but don't fail if it's already stopped
	err := c.client.StopContainer(c.json.ID, 10)
	if err != nil {
		// Check if the error is because the container is not running
		// In that case, we can proceed to remove it
		if !strings.Contains(err.Error(), "is not running") &&
			!strings.Contains(err.Error(), "Container not running") {
			c.t.Log("failed to stop container: ", c.json.ID)
			c.t.Fatal(err)
		}
	}

	err = c.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            c.json.ID,
		RemoveVolumes: true,
		Force:         true,
		Context:       context.Background(),
	})
	if err != nil {
		c.t.Fatal(err)
	}
}

func (c *nativeContainer) Wait(port string, timeout time.Duration) error {
	return wait(c.Address(), port, timeout)
}
