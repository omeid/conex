# Conex [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/omeid/conex) [![Build Status](https://travis-ci.org/omeid/conex.svg?branch=master)](https://travis-ci.org/omeid/conex) [![Go Report Card](https://goreportcard.com/badge/github.com/omeid/conex)](https://goreportcard.com/report/github.com/omeid/conex)

Conex integrates Go `testing` with Docker (and Tart, experimentally) so integration tests can start real dependencies with less boilerplate.

## Why?

Integration tests are high-value when they run against real services. Conex handles common setup work so tests stay focused on behavior:

- Start and stop containers
- Create unique names to avoid collisions
- Pull images (or build from Dockerfiles) before tests run
- Wait for TCP/UDP ports to accept connections
- Expose ports

It also supports a driver convention so reusable test helpers can register their required images.

## Quick Start

Use `conex.Main(m)` in `TestMain`:

```go
func TestMain(m *testing.M) {
  conex.Main(m)
}
```

Or pass options for per run or per package behavior:

```go
func TestMain(m *testing.M) {
  conex.Main(
    m,
    conex.OptPullImages(true),
    conex.OptBuildImages(true),
    conex.OptRequireImage("gcr.io/distroless/cc-debian10"),
  )
}
```

## Example

```go
package example_test

import (
  "testing"

  "github.com/omeid/conex"
  "github.com/conex/postgresql"
)

func TestMain(m *testing.M) {
  conex.Main(m)
}

func TestPostgreSQL(t *testing.T) {
  db, container := postgresql.Box(t)
  defer container.Drop()

  _ = db
  // use db to interact with the postgresql database

  // you can also execute commands directly inside the container
  // using an API that closely matches os/exec:
  // cmd := container.Exec("psql", "-U", "postgres", "-c", "CREATE DATABASE testdb;")
  // out, err := cmd.CombinedOutput()
}
```

## Advanced Container Options

`Config` supports Docker-specific container options:

```go
c := conex.Box(t, &conex.Config{
  Image:      "docker:dind",
  Privileged: true,
  Binds:      []string{"/var/run/docker.sock:/var/run/docker.sock"},
})
```

`Privileged` and `Binds` are Docker runner options only.

## Driver Packages

Conex drivers are small packages that wrap a service container with a native client API. This lets tests focus on the service instead of raw container lifecycle details.

A driver usually:

1. Defines an `Image` variable
2. Registers it with `conex.Require(...)`
3. Exposes a helper that returns both a client and a `conex.Container`

See the [echo box source](https://github.com/conex/echo/blob/master/echo.go) for a concrete example.

Available boxes from `github.com/conex/*`:
- [Echo](https://github.com/conex/echo)
- [PostgreSQL](https://github.com/conex/postgresql)
- [MySQL](https://github.com/conex/mysql)
- [Redis](https://github.com/conex/redis)
- [Memcached](https://github.com/conex/memcached)
- [NATS](https://github.com/conex/nats)
- [NSQ](https://github.com/conex/nsq)
- [RethinkDB](https://github.com/conex/rethink)
- [Cassandra](https://github.com/conex/cassandra)
- [Mongo](https://github.com/conex/mongo)
- [Kafka](https://github.com/conex/kafka)
- [Consul](https://github.com/conex/consul)

## Image References

An image can be either:

- A registry reference: `name[:tag|@digest]`
- A Dockerfile path: `Dockerfile` or `Dockerfile.suffix`

Before tests run, Conex either pulls/builds these images or validates they already exist, based on configuration (`conex.OptPullImages` and `conex.OptBuildImages`).

## Runners

Conex auto-detects the runner:

- **Linux + local Docker socket**: native runner (direct container IP)
- **macOS/Windows/remote Docker**: docker runner (tests run in a container)

### Native Runner

Runs tests on the host and connects directly to container IPs.

### Docker Runner

Runs tests inside a container on a shared `conex` network. This avoids host-network limitations on Docker Desktop and remote Docker hosts.

When using the docker runner, Conex:

1. Creates a `conex` network
2. Runs the test binary in a Go container on that network
3. Starts service containers on the same network
4. Lets containers communicate via container names

Customize the Go image used by the docker runner:

```go
func TestMain(m *testing.M) {
  conex.Main(
    m,
    conex.OptGoImage("golang:1.21-alpine"),
  )
}
```

### Tart Runner (Experimental)

The Tart runner creates macOS/Linux VMs using [Tart](https://github.com/cirruslabs/tart) on Apple Silicon Macs.

```bash
CONEX_RUNNER=tart go test ./...
```

Tart image references should be Tart VM images (for example, `ghcr.io/cirruslabs/macos-sequoia-base:latest`). Dockerfile image refs are not supported with the Tart runner.

### Overriding Auto-Detection

While Conex auto-detects the runner by default, you can explicitly override it using an environment variable:

```bash
# Force native runner
CONEX_RUNNER=native go test ./...

# Force docker runner
CONEX_RUNNER=docker go test ./...
```

Alternatively, you can specify the runner programmatically in your tests using `conex.OptRunnerType`:

```go
func TestMain(m *testing.M) {
  conex.Main(
    m,
    conex.OptRunnerType(conex.RunnerDocker), // Explicitly force the Docker runner
  )
}
```

## Configuration

You can configure Conex per run:

```go
func TestMain(m *testing.M) {
  conex.Main(
    m,
    conex.OptPullImages(true),
    conex.OptBuildImages(true),
    conex.OptGoImage("golang:1.22"),
    conex.OptReturnCode(255),
  )
}
```

## License

[MIT](LICENSE)
