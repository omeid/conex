package docker

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

	iruntime "github.com/omeid/conex/internal/runtime"
	"github.com/omeid/conex/log"
	"github.com/omeid/conex/runtime"
)

func init() {
	var _ iruntime.Runtime = (*nativeRuntime)(nil)
}

// nativeRuntime runs tests on the host machine and connects to containers
// via their IP addresses. This requires native Docker (not Docker for Mac).
type nativeRuntime struct {
	client  client.APIClient
	config  *iruntime.Config
	counter iruntime.Counter
}

func (r *nativeRuntime) Pull(ctx context.Context, image string) error {
	return dockerPull(ctx, r.client, image)
}

func (r *nativeRuntime) Ensure(ctx context.Context, image string) (string, error) {
	return dockerEnsure(ctx, r.client, image)
}

func (r *nativeRuntime) Build(ctx context.Context, image string, tag string) error {
	return dockerBuild(ctx, r.client, image, tag)
}

// NewNativeRuntime creates a new native runtime.
func NewNativeRuntime(client client.APIClient, config *iruntime.Config) iruntime.Runtime {
	return &nativeRuntime{
		client:  client,
		config:  config,
		counter: iruntime.NewCounter(),
	}
}

// Run executes the tests directly on the host.
func (r *nativeRuntime) Run(m *testing.M) int {
	return m.Run()
}

// Box creates a container and returns a Container that uses the container's
// direct IP address for connections.
func (r *nativeRuntime) Box(t testing.TB, conf *runtime.Config, name string) runtime.Container {
	name = fmt.Sprintf("%s_%d", name, r.counter.Count(name))

	// cname is a simple canonical name that includes the
	// container image name and params.
	cname := conf.Image
	if len(conf.Entrypoint) != 0 {
		cname = cname + " entrypoint: " + strings.Join(conf.Entrypoint, " ")
	}
	if len(conf.Cmd) != 0 {
		cname = cname + " cmd: " + strings.Join(conf.Cmd, " ")
	}

	log.Logf(t, "conex", "creating (%s) as %s", cname, name)

	exposedPorts := make(network.PortSet)
	for _, port := range conf.Expose {
		exposedPorts[network.MustParsePort(port)] = struct{}{}
	}

	cresp, err := r.client.ContainerCreate(
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
		log.Fatalf(t, name, "Failed to create container: %s", err)
	}

	_, err = r.client.ContainerStart(t.Context(), cresp.ID, client.ContainerStartOptions{})
	if err != nil {
		log.Fatalf(t, name, "Failed to start container: %v", err)
	}

	log.Logf(t, "conex", "started (%s) as %s", cname, name)

	cjsonResult, err := r.client.ContainerInspect(t.Context(), cresp.ID, client.ContainerInspectOptions{})
	if err != nil {
		log.Fatalf(t, name, "Failed to inspect: %v", err)
	}

	// Determine address (usually the bridge network IP)
	var address string
	for _, network := range cjsonResult.Container.NetworkSettings.Networks {
		if network.IPAddress.IsValid() {
			address = network.IPAddress.String()
			break
		}
	}

	return &dockerContainer{
		json:    cjsonResult.Container,
		client:  r.client,
		t:       t,
		name:    name,
		address: address,
	}
}
