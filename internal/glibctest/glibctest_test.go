package glibctest_test

import (
	"os"
	"testing"

	"github.com/omeid/conex"
)

var expectedExitCode = map[string]int{
	"alpine:3.20":          255,
	"Dockerfile.libc_test": 255,
	"golang:1.20":          0,
	"ubuntu:latest":        0,
}

func TestMain(m *testing.M) {
	if os.Getenv("CONEX_INSIDE_DOCKER") == "1" {
		os.Exit(conex.Run(m))
	}

	for image, expectedRet := range expectedExitCode {
		if os.Getenv("CGO_ENABLED") == "0" {
			expectedRet = 0
		}

		os.Setenv("CONEX_TEST_GO_IMAGE", image)
		
		ret := conex.Run(m,
			conex.OptRunnerType(conex.RunnerDocker),
			conex.OptRequireImage(image),
			conex.OptGoImage(image),
		)
		if ret != expectedRet {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func TestGlibc(t *testing.T) {
	image := os.Getenv("CONEX_TEST_GO_IMAGE")
	if image == "" {
		t.Fatal("CONEX_TEST_GO_IMAGE is not set")
	}
	
	// If we reach here, it means we are inside the container running tests.
	// We only reach here for images that have glibc (expectedExitCode == 0).
	// Therefore, CGO_ENABLED doesn't strictly matter for execution, but we
	// can assert it's empty since we didn't set it.
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Errorf("expected CGO_ENABLED to not be 0 in image %s", image)
	}
}
