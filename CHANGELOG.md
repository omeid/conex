# Changelog

## [v0.0.3] - 2026-03-29

### Added
- **Tart Runner (Experimental)**: New runner for creating macOS and Linux VMs using [Tart](https://github.com/cirruslabs/tart) on Apple Silicon Macs. VMs are cloned, started, and cleaned up automatically just like Docker containers. Activated via `CONEX_RUNNER=tart`.
- Support for both macOS (`ghcr.io/cirruslabs/macos-sequoia-base`) and Linux (`ghcr.io/cirruslabs/ubuntu`) Tart images.
- **Dockerfile Build Support**: Images can now be built from Dockerfiles instead of pulled from a registry. Use a path starting with `Dockerfile` (e.g. `Dockerfile.ssh`, `Dockerfile.testing`) as the image name. Conex detects Dockerfile paths, builds the image before tests run, and tags it automatically.
- **Privileged containers**: `Config.Privileged` runs a container in privileged mode (e.g. for Docker-in-Docker).
- **Volume bind mounts**: `Config.Binds` mounts host paths into containers (e.g. `"/host/path:/container/path"`).

### Fixed
- Docker API version negotiation with modern Docker daemons. The client no longer defaults to API version 1.25 which is rejected by Docker >= 1.40.

## [v0.0.2] - 2026-03-27

### Added
- **Docker Runner**: New runner that automatically runs tests inside a Docker container on a shared network. This enables conex to work on macOS, Windows, and other environments where container IPs are not directly accessible from the host.
- **Auto-detection**: Conex now automatically detects the appropriate runner based on the environment:
  - Linux with local Docker socket → Native runner
  - macOS, Windows, or remote Docker → Docker runner
- **`GoImage` configuration**: Set `conex.GoImage` to specify the Go Docker image used when running tests inside a container (default: `golang:1.22`).
- **`CONEX_RUNNER` environment variable**: Override auto-detection by setting to `native` or `docker`.

### Changed
- **Updated `go-dockerclient`**: Upgraded from v1.3.0 to v1.11.0 for compatibility with newer Docker server versions.
- **Updated `docker/docker`**: Upgraded from v0.7.3 to v25.0.4.
- **Go version**: Minimum Go version is now 1.21.
- **Container IP resolution**: Fixed `Address()` to work with newer Docker API where IP addresses are in the `Networks` map rather than the top-level `IPAddress` field.
- **Image pulling**: Added default "latest" tag when pulling images without an explicit tag (required by newer Docker API).

### Fixed
- Container `Drop()` no longer fails if the container has already stopped.
- Tests now work with modern PostgreSQL images that require `POSTGRES_HOST_AUTH_METHOD=trust` or a password.

### Removed
- Removed the caveat about Docker for Mac not being supported - it now works via the Docker runner.
