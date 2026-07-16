//go:build !tart

package build_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omeid/conex"
)

var (
	buildImage      = "Dockerfile"
	privilegedImage = "Dockerfile.privileged"
)

func TestMain(m *testing.M) {
	conex.Main(
		m,
		conex.OptRequireImage(buildImage),
		conex.OptRequireImage(privilegedImage),
	)
}

func TestBuildBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: buildImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected container to have an address")
	}
	conex.Logf(t, "", "container address: %s", c.Address())
}

func TestBuildContentsExist(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: buildImage,
	})
	defer c.Drop()

	out := dockerExec(t, c, "cat /conex-build-marker")
	expected := "hello from conex build"
	if !strings.Contains(out, expected) {
		t.Fatalf("expected marker to contain %q, got %q", expected, out)
	}
	conex.Logf(t, "", "marker file: %s", strings.TrimSpace(out))
}

func TestPrivileged(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image:      privilegedImage,
		Privileged: true,
	})
	defer c.Drop()

	// In privileged mode, we can access /dev/mem and similar.
	// A simpler check: ip link add works only in privileged mode.
	out := dockerExec(t, c, "ip link add dummy0 type dummy 2>&1 && echo OK || echo DENIED")
	if !strings.Contains(out, "OK") {
		t.Fatalf("expected privileged operation to succeed, got: %s", out)
	}
	conex.Logf(t, "", "privileged mode: OK")
}

func TestNotPrivileged(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image:      privilegedImage,
		Privileged: false,
	})
	defer c.Drop()

	// Without privileged, ip link add should fail.
	out := dockerExec(t, c, "ip link add dummy0 type dummy 2>&1 && echo OK || echo DENIED")
	if !strings.Contains(out, "DENIED") {
		t.Fatalf("expected unprivileged operation to fail, got: %s", out)
	}
	conex.Logf(t, "", "unprivileged mode: correctly denied")
}

func TestBindMount(t *testing.T) {
	tempDir, err := os.MkdirTemp("../.tmp", "conex-tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	hostDir, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(hostDir)
	}()

	if err := os.WriteFile(filepath.Join(hostDir, "bind-marker.txt"), []byte("from host"), 0644); err != nil {
		t.Fatal(err)
	}

	c := conex.Box(t, &conex.Config{
		Image: buildImage,
		Binds: []string{hostDir + ":/mnt/conex"},
	})
	defer c.Drop()

	// Verify the host file is visible inside the container.
	out := dockerExec(t, c, "cat /mnt/conex/bind-marker.txt")
	if !strings.Contains(out, "from host") {
		t.Fatalf("expected bind mount content, got: %s", out)
	}
	conex.Logf(t, "", "bind mount: OK")

	// Write a file from inside the container.
	dockerExec(t, c, "echo 'from container' > /mnt/conex/container-marker.txt")

	// Verify it's visible on the host.
	data, err := os.ReadFile(filepath.Join(hostDir, "container-marker.txt"))
	if err != nil {
		t.Fatalf("expected container-written file on host: %v", err)
	}
	if !strings.Contains(string(data), "from container") {
		t.Fatalf("expected 'from container', got %q", string(data))
	}
	conex.Logf(t, "", "bind mount write-back: OK")
}

func dockerExec(t *testing.T, c conex.Container, command string) string {
	t.Helper()
	cmd := c.Exec("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("container exec %q: %v: %s", command, err, stderr.String())
	}
	return stdout.String()
}
