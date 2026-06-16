package conex

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"net/netip"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/term"
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
	cresp, err := r.config.Client.ContainerCreate(
		context.Background(),
		client.ContainerCreateOptions{
			Config: &container.Config{
				Image:      r.config.GoImage,
				Cmd:        cmd,
				Env:        env,
				WorkingDir: workDir,
				Tty:        false,
			},
			HostConfig: &container.HostConfig{
				NetworkMode: ConexNetworkName,
				Binds:       binds,
				AutoRemove:  true,
			},
			Name: containerName,
		},
	)
	if err != nil {
		fmt.Printf("conex: failed to create runner container: %v\n", err)
		return r.config.RetCode
	}

	// Ensure cleanup
	defer func() {
		// AutoRemove should handle this, but let's be safe
		_, _ = r.config.Client.ContainerRemove(context.Background(), cresp.ID, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
	}()

	// Start the container
	_, err = r.config.Client.ContainerStart(context.Background(), cresp.ID, client.ContainerStartOptions{})
	if err != nil {
		fmt.Printf("conex: failed to start runner container: %v\n", err)
		return r.config.RetCode
	}

	// Attach to get stdout/stderr
	go func() {
		reader, err := r.config.Client.ContainerLogs(context.Background(), cresp.ID, client.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		})
		if err != nil {
			return
		}
		defer func() { _ = reader.Close() }()
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
	}()

	// Wait for container to finish
	waitRes := r.config.Client.ContainerWait(context.Background(), cresp.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	var exitCode int
	select {
	case err := <-waitRes.Error:
		if err != nil {
			fmt.Printf("conex: failed to wait for runner container: %v\n", err)
			return r.config.RetCode
		}
	case status := <-waitRes.Result:
		exitCode = int(status.StatusCode)
	}

	return exitCode
}

// ensureNetwork creates the conex network if it doesn't exist.
func (r *DockerRunner) ensureNetwork() error {
	networks, err := r.config.Client.NetworkList(context.Background(), client.NetworkListOptions{})
	if err != nil {
		return err
	}

	for _, net := range networks.Items {
		if net.Name == ConexNetworkName {
			r.networkID = net.ID
			return nil
		}
	}

	// Create the network
	res, err := r.config.Client.NetworkCreate(context.Background(), ConexNetworkName, client.NetworkCreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return err
	}

	r.networkID = res.ID
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
	if len(conf.Entrypoint) != 0 {
		cname = cname + " entrypoint: " + strings.Join(conf.Entrypoint, " ")
	}
	if len(conf.Cmd) != 0 {
		cname = cname + " cmd: " + strings.Join(conf.Cmd, " ")
	}

	logf(t, "creating (%s) as %s on network %s", cname, name, ConexNetworkName)

	exposedPorts := make(network.PortSet)
	portBindings := make(network.PortMap)

	for _, port := range conf.Expose {
		dp := network.MustParsePort(port)
		exposedPorts[dp] = struct{}{}
		// Bind to random host port for potential debugging
		portBindings[dp] = []network.PortBinding{{
			HostIP:   netip.MustParseAddr("0.0.0.0"),
			HostPort: "",
		}}
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
				NetworkMode:  ConexNetworkName,
				PortBindings: portBindings,
				Privileged:   conf.Privileged,
				Binds:        conf.Binds,
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

	logf(t, "started (%s) as %s", cname, name)

	cjsonResult, err := r.config.Client.ContainerInspect(t.Context(), cresp.ID, client.ContainerInspectOptions{})
	if err != nil {
		fatalf(t, "Failed to inspect: %v", err)
	}
	cjson := cjsonResult.Container

	// Determine how to address this container
	var address string
	if os.Getenv(ConexRunnerEnv) == "1" {
		// We're inside a container, use the container name
		address = name
	} else {
		// We're on the host, try to use the container's IP on the conex network
		if netSettings, ok := cjson.NetworkSettings.Networks[ConexNetworkName]; ok && netSettings.IPAddress.IsValid() {
			address = netSettings.IPAddress.String()
		} else {
			// Try any available network
			for _, network := range cjson.NetworkSettings.Networks {
				if network.IPAddress.IsValid() {
					address = network.IPAddress.String()
					break
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
	json     container.InspectResponse
	client   client.APIClient
	t        testing.TB
	name     string
	address  string
	dropOnce sync.Once
}

func (c *dockerContainer) ID() string {
	return c.json.ID
}

func (c *dockerContainer) Image() string {
	return c.json.Config.Image
}

func (c *dockerContainer) Name() string {
	return c.json.Name
}

func (c *dockerContainer) Address() string {
	return c.address
}

func (c *dockerContainer) Drop() {
	c.dropOnce.Do(func() {
		// Try to stop the container, but don't fail if it's already stopped
		timeout := 10
		_, _ = c.client.ContainerStop(context.Background(), c.json.ID, client.ContainerStopOptions{Timeout: &timeout})

		_, err := c.client.ContainerRemove(context.Background(), c.json.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			c.t.Fatal(err)
		}
	})
}

func (c *dockerContainer) Wait(port string, timeout time.Duration) error {
	err := wait(c.Address(), port, timeout)
	if err != nil && testing.Verbose() {
		c.t.Logf("=== Container %s Logs ===", c.Name())
		_ = c.Logs(os.Stdout, os.Stderr)
		c.t.Log("=========================")
	}
	return err
}

func (c *dockerContainer) Exec(cmd ...string) *Cmd {
	return newDockerCmd(c.t, c.client, c.json.ID, cmd)
}

// Logs writes the container logs to the provided stdout and stderr writers.
func (c *dockerContainer) Logs(stdout io.Writer, stderr io.Writer) error {
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

func newDockerCmd(t testing.TB, cli client.APIClient, containerID string, cmd []string) *Cmd {
	if len(cmd) == 0 {
		return nil
	}

	var outStream io.Writer = os.Stdout
	if os.Stdout == nil {
		outStream = io.Discard
	}
	var errStream io.Writer = os.Stderr
	if os.Stderr == nil {
		errStream = io.Discard
	}

	c := &Cmd{
		Path:   cmd[0],
		Args:   cmd,
		Stdout: outStream,
		Stderr: errStream,
	}

	var execID string
	errCh := make(chan error, 1)

	c.start = func() error {
		opts := client.ExecCreateOptions{
			Cmd:          c.Args,
			Env:          c.Env,
			WorkingDir:   c.Dir,
			AttachStdin:  c.Stdin != nil,
			AttachStdout: true,
			AttachStderr: true,
		}

		execInfo, err := cli.ExecCreate(t.Context(), containerID, opts)
		if err != nil {
			return err
		}
		execID = execInfo.ID

		go func() {
			resp, err := cli.ExecAttach(t.Context(), execID, client.ExecAttachOptions{
				TTY: false,
			})
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Close()

			if c.Stdin != nil {
				go func() {
					_, _ = io.Copy(resp.Conn, c.Stdin)
					_ = resp.CloseWrite()
				}()
			}

			_, copyErr := stdcopy.StdCopy(c.Stdout, c.Stderr, resp.Reader)
			errCh <- copyErr
		}()
		return nil
	}

	c.wait = func() error {
		err := <-errCh
		if err != nil && err != io.EOF {
			return err
		}
		execInspect, err := cli.ExecInspect(t.Context(), execID, client.ExecInspectOptions{})
		if err != nil {
			return err
		}
		if execInspect.ExitCode != 0 {
			return fmt.Errorf("exit status %d", execInspect.ExitCode)
		}
		return nil
	}

	return c
}
