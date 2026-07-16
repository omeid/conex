package conex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"net/netip"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	// ConexNetworkName is the name of the Docker network used for conex containers.
	ConexNetworkName = "conex"
	// ConexRunnerEnv is the environment variable that indicates we're running inside a conex container.
	ConexRunnerEnv = "CONEX_INSIDE_DOCKER"
)

func init() {
	var _ runner = (*dockerRunner)(nil)
}

// dockerRunner runs tests inside a Docker container on the same network
// as other conex containers. This allows conex to work on systems where
// container IPs are not directly accessible (e.g., Docker for Mac).
type dockerRunner struct {
	*nativeRunner
	networkID string
}

func NewDockerRunner(client client.APIClient, config *runnerConfig) runner {
	return &dockerRunner{
		nativeRunner: newNativeRunner(client, config).(*nativeRunner),
	}
}

// Run executes the tests. If we're already inside a Docker container
// (detected by environment variable), it just runs the tests.
// Otherwise, it creates a container, mounts the current directory,
// and runs the tests inside it.
func (r *dockerRunner) Run(m *testing.M) int {
	// If we're already inside the container, just run the tests
	if os.Getenv(ConexRunnerEnv) == "1" {
		return m.Run()
	}

	// We need to run tests inside a Docker container
	return r.runInDocker()
}

// runInDocker creates a container and runs the test binary inside it.
func (r *dockerRunner) runInDocker() int {
	// Ensure the network exists
	if err := r.ensureNetwork(); err != nil {
		Logf(nil, "conex", "failed to create network: %v\n", err)
		return r.config.RetCode
	}

	workDir, err := os.Getwd()
	if err != nil {
		Logf(nil, "conex", "failed to get working directory: %v\n", err)
		return r.config.RetCode
	}

	if evaluated, err := filepath.EvalSymlinks(workDir); err == nil {
		workDir = evaluated
	}

	// Inspect triple OS, architecture, and glibc availability.
	triple, err := r.inspectTarget(context.Background(), r.config.GoImage)
	if err != nil {
		Logf(nil, "conex", "failed to inspect target image triple: %v\n", err)
		return r.config.RetCode
	}

	root := workDir
	projectRoot := findProjectRoot(workDir)
	if projectRoot != "" {
		root = projectRoot
	}

	cmd := make([]string, 0, len(os.Args))
	if !triple.matchOS() {
		var cleanup func()
		bin, cleanup, err := r.crossCompile(triple, root)
		if err != nil {
			Logf(nil, "conex", "failed to cross-compile test binary: %v\n", err)
			return r.config.RetCode
		}
		cmd = append(cmd, bin)
		defer cleanup()
	} else {
		bin, err := filepath.Abs(os.Args[0])
		if err != nil {
			Logf(nil, "conex", "failed to get test binary path: %v\n", err)
			return r.config.RetCode
		}
		cmd = append(cmd, bin)
	}

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.testlogfile") || strings.HasPrefix(arg, "--test.testlogfile") {
			continue
		}
		cmd = append(cmd, arg)
	}

	if evaluated, err := filepath.EvalSymlinks(cmd[0]); err == nil {
		cmd[0] = evaluated
	}

	// Create container name
	containerName := fmt.Sprintf("%s-runner", r.config.Name)

	Logf(nil, "conex", "Running tests inside container (%s)", r.config.GoImage)

	// Determine host Docker socket path
	dockerSocket := "/var/run/docker.sock"
	if runtime.GOOS == "linux" {
		if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
			dockerSocket = strings.TrimPrefix(host, "unix://")
		}
	}

	// Mount the compiled test binary and the project root directory.
	binds := []string{
		fmt.Sprintf("%s:%s:ro", cmd[0], cmd[0]),
		fmt.Sprintf("%s:%s", root, root),
		// Mount Docker socket so the test can create containers
		fmt.Sprintf("%s:/var/run/docker.sock", dockerSocket),
	}

	// Set environment variables
	env := []string{
		fmt.Sprintf("%s=1", ConexRunnerEnv),
		"CONEX_RUNNER=docker",
	}

	// Pass through relevant environment variables
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "DOCKER_HOST=") {
			continue
		}
		if strings.HasPrefix(e, "DOCKER_") ||
			strings.HasPrefix(e, "CONEX_") ||
			strings.HasPrefix(e, "GO") ||
			strings.HasPrefix(e, "PATH=") {
			env = append(env, e)
		}
	}

	// Create the container
	cresp, err := r.client.ContainerCreate(
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
		Logf(nil, "conex", "failed to create runner container: %v\n", err)
		return r.config.RetCode
	}

	// Ensure cleanup
	defer func() {
		// AutoRemove should handle this, but let's be safe
		_, _ = r.client.ContainerRemove(context.Background(), cresp.ID, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
	}()

	// Start the container
	_, err = r.client.ContainerStart(context.Background(), cresp.ID, client.ContainerStartOptions{})
	if err != nil {
		Logf(nil, "conex", "failed to start runner container: %v\n", err)
		return r.config.RetCode
	}

	// Attach to get stdout/stderr
	go func() {
		reader, err := r.client.ContainerLogs(context.Background(), cresp.ID, client.ContainerLogsOptions{
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
	waitRes := r.client.ContainerWait(context.Background(), cresp.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	var exitCode int
	select {
	case err := <-waitRes.Error:
		if err != nil {
			Logf(nil, "conex", "failed to wait for runner container: %v\n", err)
			return r.config.RetCode
		}
	case status := <-waitRes.Result:
		exitCode = int(status.StatusCode)
	}

	return exitCode
}

// ensureNetwork creates the conex network if it doesn't exist.
func (r *dockerRunner) ensureNetwork() error {
	networks, err := r.client.NetworkList(context.Background(), client.NetworkListOptions{})
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
	res, err := r.client.NetworkCreate(context.Background(), ConexNetworkName, client.NetworkCreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return err
	}

	r.networkID = res.ID
	return nil
}

func (r *dockerRunner) Box(t testing.TB, conf *Config, name string) Container {
	// Ensure network exists
	if r.networkID == "" {
		if err := r.ensureNetwork(); err != nil {
			fatalf(t, "", "Failed to ensure network: %v", err)
		}
	}

	optCreateConfig := func(cc *client.ContainerCreateOptions) {
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
		cc.Config.ExposedPorts = exposedPorts
		cc.HostConfig.NetworkMode = ConexNetworkName
		cc.HostConfig.PortBindings = portBindings
	}

	return r.box(t, conf, name, optCreateConfig)
}


