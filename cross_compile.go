package conex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type targetInfo struct {
	os       string
	arch     string
	hasGlibc bool
}

func (t targetInfo) matchOS() bool {
	if stdruntime.GOOS != t.os || stdruntime.GOARCH != t.arch {
		return false
	}
	if cgoEnabled && !t.hasGlibc {
		return false
	}
	return true
}

func (r *dockerRunner) inspectTarget(ctx context.Context, image string) (targetInfo, error) {
	inspect, err := r.client.ImageInspect(ctx, image)
	if err != nil {
		return targetInfo{}, err
	}

	info := targetInfo{
		os:   inspect.Os,
		arch: inspect.Architecture,
	}

	switch info.arch {
	case "x86_64":
		info.arch = "amd64"
	case "aarch64":
		info.arch = "arm64"
	}

	// Short circuit for common images to avoid container creation
	if strings.Contains(image, "alpine") {
		info.hasGlibc = false
		return info, nil
	}
	if strings.HasPrefix(image, "golang:") || image == "golang" {
		info.hasGlibc = true
		return info, nil
	}

	cresp, err := r.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:      image,
			Entrypoint: []string{"sh", "-c", "getconf GNU_LIBC_VERSION"},
			Tty:        false,
		},
		HostConfig: &container.HostConfig{
			AutoRemove: true,
		},
	})
	if err != nil {
		return info, err
	}

	resp, err := r.client.ContainerAttach(ctx, cresp.ID, client.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return info, err
	}

	if _, err := r.client.ContainerStart(ctx, cresp.ID, client.ContainerStartOptions{}); err != nil {
		return info, err
	}

	waitRes := r.client.ContainerWait(ctx, cresp.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	var statusCode int64
	select {
	case err := <-waitRes.Error:
		return info, err
	case result := <-waitRes.Result:
		statusCode = result.StatusCode
	}

	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	resp.Close()

	if statusCode == 0 {
		info.hasGlibc = true
		return info, nil
	}

	errStr := strings.TrimSpace(stderr.String())
	if errStr == "" {
		errStr = strings.TrimSpace(stdout.String())
	}

	if strings.Contains(errStr, "unknown variable") || strings.Contains(errStr, "not found") {
		info.hasGlibc = false
		return info, nil
	}

	return info, fmt.Errorf("getconf failed: exit code %d, output: %s", statusCode, errStr)
}

func (r *dockerRunner) crossCompile(target targetInfo, buildDir string) (string, func(), error) {
	testBinDir, err := os.MkdirTemp(buildDir, ".conex-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary build directory: %w", err)
	}

	testBinPath := filepath.Join(testBinDir, "conex_test.bin")
	buildCmd := exec.Command("go", "test", "-c", "-o", testBinPath)
	buildCmd.Env = append(os.Environ(), "GOOS="+target.os, "GOARCH="+target.arch)
	if !target.hasGlibc {
		buildCmd.Env = append(buildCmd.Env, "CGO_ENABLED=0")
	}

	if out, err := buildCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(testBinDir)
		return "", nil, fmt.Errorf("failed to compile test binary for %s/%s: %w\n%s", target.os, target.arch, err, string(out))
	}

	cleanup := func() {
		_ = os.RemoveAll(testBinDir)
	}

	return testBinPath, cleanup, nil
}

func findProjectRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
