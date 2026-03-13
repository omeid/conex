# Changelog

## [Unreleased]

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
