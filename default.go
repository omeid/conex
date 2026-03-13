package conex

import (
	"os"
	"runtime"
	"testing"
)

var std Manager

var requiredImages = []func() string{}

var (
	// FailReturnCode is used as status code when conex fails to setup during Run.
	// This does not override the return value of testing.M.Run, only when conex
	// fails to even testing.M.Run.
	FailReturnCode = 255
	// PullImages dictates whatever the Manager should attempt to pull the images
	// on run or simply ensure they exist.
	// Note: Pulling images may result into updates.
	PullImages = true

	// GoImage is the Docker image used to run tests inside a container when
	// using the Docker runner. This should be a Go image that matches your
	// Go version. Set this before calling Run() if you need a specific version.
	// Example: "golang:1.21-alpine"
	GoImage = "golang:1.22"
)

// Require adds the image name returned by the provided functions
// to the list of images pull by the default Manager when Run is
// called. Used by driver packages, see conex/redis, conex/rethink.
func Require(images ...func() string) {
	requiredImages = append(requiredImages, images...)
}

// detectRunner determines the appropriate runner based on the environment.
// It returns RunnerNative if we're on Linux with a local Docker socket,
// otherwise RunnerDocker for environments like Docker for Mac where
// container IPs are not directly accessible.
func detectRunner() RunnerType {
	// Allow explicit override via environment variable
	if envRunner := os.Getenv("CONEX_RUNNER"); envRunner != "" {
		return RunnerType(envRunner)
	}

	// If we're already inside a Docker container, use the docker runner
	if os.Getenv(ConexRunnerEnv) == "1" {
		return RunnerDocker
	}

	// On Linux with a local Docker socket, container IPs are directly accessible
	if runtime.GOOS == "linux" {
		// Check if DOCKER_HOST is set to something non-local
		dockerHost := os.Getenv("DOCKER_HOST")
		if dockerHost == "" || dockerHost == "unix:///var/run/docker.sock" {
			return RunnerNative
		}
	}

	// For macOS, Windows, or remote Docker hosts, use the docker runner
	// since container IPs won't be directly accessible
	return RunnerDocker
}

// Run prepares a docker client, pulls the provided list of images
// and then runs your tests.
func Run(m *testing.M, images ...string) int {

	for _, i := range requiredImages {
		images = append(images, i())
	}

	runnerType := detectRunner()

	std = newManager(FailReturnCode, PullImages, runnerType, GoImage, images...)

	return std.Run(m, images...)
}

// Box creates a new container using the provided image and passes
// your parameters.
func Box(t testing.TB, conf *Config) Container {
	if std == nil {
		panic("You must call conex.Run first. Use TestMain.")
	}

	return std.Box(t, conf)
}
