package conex

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/term"
)

func init() {
	var _ runner = (*nativeRunner)(nil)
}

// nativeRunner runs tests on the host machine and connects to containers
// via their IP addresses. This requires native Docker (not Docker for Mac).
type nativeRunner struct {
	client  client.APIClient
	config  *runnerConfig
	counter *counter
}

func (r *nativeRunner) Pull(ctx context.Context, image string) error {
	return dockerPull(ctx, r.client, image)
}

func (r *nativeRunner) Ensure(ctx context.Context, image string) (string, error) {
	return dockerEnsure(ctx, r.client, image)
}

func (r *nativeRunner) Build(ctx context.Context, image string, tag string) error {
	return dockerBuild(ctx, r.client, image, tag)
}

// NewNativeRunner creates a new native runner.
func newNativeRunner(client client.APIClient, config *runnerConfig) runner {
	return &nativeRunner{
		client:  client,
		config:  config,
		counter: &counter{seqs: make(map[string]int)},
	}
}

// Run executes the tests directly on the host.
func (r *nativeRunner) Run(m *testing.M) int {
	return m.Run()
}

// Box creates a container and returns a Container that uses the container's
// direct IP address for connections.
func (r *nativeRunner) Box(t testing.TB, conf *Config, name string) Container {
	return r.box(t, conf, name, nil)
}

func (r *nativeRunner) box(t testing.TB, conf *Config, name string, optCreateConfig func(*client.ContainerCreateOptions)) Container {
	name = fmt.Sprintf("%s_%d", name, r.counter.Count(name))
	cname := conf.Image
	if len(conf.Entrypoint) != 0 {
		cname = cname + " entrypoint: " + strings.Join(conf.Entrypoint, " ")
	}
	if len(conf.Cmd) != 0 {
		cname = cname + " cmd: " + strings.Join(conf.Cmd, " ")
	}

	Logf(t, "conex", "creating (%s) as %s", cname, name)

	exposedPorts := make(network.PortSet)
	for _, port := range conf.Expose {
		exposedPorts[network.MustParsePort(port)] = struct{}{}
	}

	cc := client.ContainerCreateOptions{
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
	}

	if optCreateConfig != nil {
		optCreateConfig(&cc)
	}

	cresp, err := r.client.ContainerCreate(t.Context(), cc)
	if err != nil {
		fatalf(t, name, "Failed to create container: %s", err)
	}

	_, err = r.client.ContainerStart(t.Context(), cresp.ID, client.ContainerStartOptions{})
	if err != nil {
		fatalf(t, name, "Failed to start container: %v", err)
	}

	Logf(t, "conex", "started (%s) as %s", cname, name)

	cjsonResult, err := r.client.ContainerInspect(t.Context(), cresp.ID, client.ContainerInspectOptions{})
	if err != nil {
		fatalf(t, name, "Failed to inspect: %v", err)
	}
	cjson := cjsonResult.Container

	// Determine address (usually the bridge network IP)
	var address string
	if cc.HostConfig.NetworkMode == ConexNetworkName {
		if os.Getenv(ConexRunnerEnv) == "1" {
			address = name
		} else if netSettings, ok := cjson.NetworkSettings.Networks[ConexNetworkName]; ok && netSettings.IPAddress.IsValid() {
			address = netSettings.IPAddress.String()
		}
	}

	if address == "" {
		for _, network := range cjson.NetworkSettings.Networks {
			if network.IPAddress.IsValid() {
				address = network.IPAddress.String()
				break
			}
		}
	}

	return &dockerContainer{
		json:    cjson,
		client:  r.client,
		t:       t,
		name:    name,
		address: address,
	}
}
