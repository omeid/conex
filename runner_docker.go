package conex

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

const (
	// ConexNetworkName is the name of the Docker network used for conex containers.
	ConexNetworkName = "conex"
	// ConexRunnerEnv is the environment variable that indicates we're running inside a conex container.
	ConexRunnerEnv = "CONEX_INSIDE_DOCKER"
)

func init() {
	var _ Runner = (*DockerRunner)(nil)
	var _ Container = (*dockerContainer)(nil)
}

// DockerRunner runs tests inside a Docker container on the same network
// as other conex containers. This allows conex to work on systems where
// container IPs are not directly accessible (e.g., Docker for Mac).
type DockerRunner struct {
	config    *RunnerConfig
	networkID string
}

// NewDockerRunner creates a new Docker runner.
func NewDockerRunner(config *RunnerConfig) *DockerRunner {
	return &DockerRunner{config: config}
}

// Run executes the tests. If we're already inside a Docker container
// (detected by environment variable), it just runs the tests.
// Otherwise, it creates a container, mounts the current directory,
// and runs the tests inside it.
func (r *DockerRunner) Run(m *testing.M) int {
	// If we're already inside the container, just run the tests
	if os.Getenv(ConexRunnerEnv) == "1" {
		return m.Run()
	}

	// We need to run tests inside a Docker container
	return r.runInDocker()
}

// runInDocker creates a container and runs the test binary inside it.
func (r *DockerRunner) runInDocker() int {
	// Ensure the network exists
	if err := r.ensureNetwork(); err != nil {
		fmt.Printf("conex: failed to create network: %v\n", err)
		return r.config.RetCode
	}

	// Get the test binary path and working directory
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		fmt.Printf("conex: failed to get test binary path: %v\n", err)
		return r.config.RetCode
	}

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("conex: failed to get working directory: %v\n", err)
		return r.config.RetCode
	}

	// Build the command - re-run the test binary with same args
	// The test binary is already compiled, we just need to run it
	cmd := os.Args

	// Create container name
	containerName := fmt.Sprintf("%s-runner", r.config.Name)

	fmt.Printf("=== conex: Running tests inside container (%s)\n", r.config.GoImage)

	// Mount the test binary and working directory
	binds := []string{
		fmt.Sprintf("%s:%s:ro", testBinary, testBinary),
		fmt.Sprintf("%s:%s", workDir, workDir),
		// Mount Docker socket so the test can create containers
		"/var/run/docker.sock:/var/run/docker.sock",
	}

	// Set environment variables
	env := []string{
		fmt.Sprintf("%s=1", ConexRunnerEnv),
		"CONEX_RUNNER=docker",
	}

	// Pass through relevant environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "DOCKER_") ||
			strings.HasPrefix(e, "CONEX_") ||
			strings.HasPrefix(e, "GO") ||
			strings.HasPrefix(e, "PATH=") {
			env = append(env, e)
		}
	}

	// Create the container
	container, err := r.config.Client.CreateContainer(docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image:      r.config.GoImage,
			Cmd:        cmd,
			Env:        env,
			WorkingDir: workDir,
			Tty:        false,
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: ConexNetworkName,
			Binds:       binds,
			AutoRemove:  true,
		},
	})
	if err != nil {
		fmt.Printf("conex: failed to create runner container: %v\n", err)
		return r.config.RetCode
	}

	// Ensure cleanup
	defer func() {
		// AutoRemove should handle this, but let's be safe
		_ = r.config.Client.RemoveContainer(docker.RemoveContainerOptions{
			ID:      container.ID,
			Force:   true,
			Context: context.Background(),
		})
	}()

	// Start the container
	err = r.config.Client.StartContainer(container.ID, nil)
	if err != nil {
		fmt.Printf("conex: failed to start runner container: %v\n", err)
		return r.config.RetCode
	}

	// Attach to get stdout/stderr
	go func() {
		_ = r.config.Client.Logs(docker.LogsOptions{
			Container:    container.ID,
			OutputStream: os.Stdout,
			ErrorStream:  os.Stderr,
			Stdout:       true,
			Stderr:       true,
			Follow:       true,
		})
	}()

	// Wait for container to finish
	exitCode, err := r.config.Client.WaitContainer(container.ID)
	if err != nil {
		fmt.Printf("conex: failed to wait for runner container: %v\n", err)
		return r.config.RetCode
	}

	return exitCode
}

// ensureNetwork creates the conex network if it doesn't exist.
func (r *DockerRunner) ensureNetwork() error {
	networks, err := r.config.Client.ListNetworks()
	if err != nil {
		return err
	}

	for _, net := range networks {
		if net.Name == ConexNetworkName {
			r.networkID = net.ID
			return nil
		}
	}

	// Create the network
	net, err := r.config.Client.CreateNetwork(docker.CreateNetworkOptions{
		Name:   ConexNetworkName,
		Driver: "bridge",
	})
	if err != nil {
		return err
	}

	r.networkID = net.ID
	return nil
}

// Box creates a container on the conex network and returns a Container
// that uses the container name for connections.
func (r *DockerRunner) Box(t testing.TB, conf *Config, name string) Container {
	// Ensure network exists
	if r.networkID == "" {
		if err := r.ensureNetwork(); err != nil {
			fatalf(t, "Failed to ensure network: %v", err)
		}
	}

	cname := conf.Image
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	logf(t, "creating (%s) as %s on network %s", cname, name, ConexNetworkName)

	exposedPorts := make(map[docker.Port]struct{})
	portBindings := make(map[docker.Port][]docker.PortBinding)

	for _, port := range conf.Expose {
		dp := docker.Port(port)
		exposedPorts[dp] = struct{}{}
		// Bind to random host port for potential debugging
		portBindings[dp] = []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}}
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
				NetworkMode:  ConexNetworkName,
				PortBindings: portBindings,
				Privileged:   conf.Privileged,
				Binds:        conf.Binds,
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

	// Determine how to address this container
	var address string
	if os.Getenv(ConexRunnerEnv) == "1" {
		// We're inside a container, use the container name
		address = name
	} else {
		// We're on the host, try to use the container's IP on the conex network
		if netSettings, ok := cjson.NetworkSettings.Networks[ConexNetworkName]; ok && netSettings.IPAddress != "" {
			address = netSettings.IPAddress
		} else {
			// Fallback: try top-level IPAddress first
			if cjson.NetworkSettings.IPAddress != "" {
				address = cjson.NetworkSettings.IPAddress
			} else {
				// Try any available network
				for _, network := range cjson.NetworkSettings.Networks {
					if network.IPAddress != "" {
						address = network.IPAddress
						break
					}
				}
			}
		}
	}

	return &dockerContainer{
		json:    cjson,
		client:  r.config.Client,
		t:       t,
		name:    name,
		address: address,
	}
}

// dockerContainer implements Container for Docker network-based access.
type dockerContainer struct {
	json    *docker.Container
	client  *docker.Client
	t       testing.TB
	name    string
	address string
}

func (c *dockerContainer) ID() string {
	return c.json.ID
}

func (c *dockerContainer) Image() string {
	return c.json.Image
}

func (c *dockerContainer) Name() string {
	return c.json.Name
}

func (c *dockerContainer) Address() string {
	return c.address
}

func (c *dockerContainer) Drop() {
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

func (c *dockerContainer) Wait(port string, timeout time.Duration) error {
	return wait(c.Address(), port, timeout)
}

// Logs returns the container logs as a ReadCloser.
func (c *dockerContainer) Logs() (io.ReadCloser, error) {
	var buf bytes.Buffer

	err := c.client.Logs(docker.LogsOptions{
		Container:    c.json.ID,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Stdout:       true,
		Stderr:       true,
	})
	if err != nil {
		return nil, err
	}

	return io.NopCloser(&buf), nil
}
