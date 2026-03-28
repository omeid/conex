//go:build tart

package conex_test

import (
	"os"
	"testing"
	"time"

	"github.com/omeid/conex"
)

var (
	tartMacImage   = "ghcr.io/cirruslabs/macos-sequoia-base:latest"
	tartLinuxImage = "ghcr.io/cirruslabs/ubuntu:latest"
)

func init() {
	conex.Require(func() string { return tartMacImage })
	conex.Require(func() string { return tartLinuxImage })
	os.Setenv("CONEX_RUNNER", "tart")
}

func TestMain(m *testing.M) {
	os.Exit(conex.Run(m))
}

// --- macOS VM tests ---

func TestTartMacBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartMacImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected VM to have an IP address")
	}
	t.Logf("VM address: %s", c.Address())
}

func TestTartMacExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartMacImage,
	})
	defer c.Drop()

	t.Logf("VM %s running at %s", c.Name(), c.Address())
}

func TestTartMacWait(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartMacImage,
	})
	defer c.Drop()

	err := c.Wait("22/tcp", 60*time.Second)
	if err != nil {
		t.Fatalf("SSH port not ready: %v", err)
	}
	t.Log("SSH port is accepting connections")
}

// --- Linux VM tests ---

func TestTartLinuxBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartLinuxImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected VM to have an IP address")
	}
	t.Logf("VM address: %s", c.Address())
}

func TestTartLinuxExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartLinuxImage,
	})
	defer c.Drop()

	t.Logf("VM %s running at %s", c.Name(), c.Address())
}

func TestTartLinuxWait(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartLinuxImage,
	})
	defer c.Drop()

	err := c.Wait("22/tcp", 60*time.Second)
	if err != nil {
		t.Fatalf("SSH port not ready: %v", err)
	}
	t.Log("SSH port is accepting connections")
}
