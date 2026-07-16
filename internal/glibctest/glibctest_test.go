package glibctest_test

import (
	"os"
	"testing"

	"github.com/omeid/conex"
)

var expectedExitCode = map[string]int{
	"alpine:3.20":          0,
	"Dockerfile.libc_test": 0,
	"golang:1.20":          0,
	"ubuntu:latest":        0,
}

func TestMain(m *testing.M) {
	if os.Getenv("CONEX_INSIDE_DOCKER") == "1" {
		os.Exit(conex.Run(m))
	}

	for image, expectedRet := range expectedExitCode {
		_ = os.Setenv("CONEX_TEST_GO_IMAGE", image)

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

	// We reach here for all images since they either have glibc or were
	// successfully cross-compiled with CGO_ENABLED=0.
}
