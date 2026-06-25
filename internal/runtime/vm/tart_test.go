//go:build tart

package vm_test

import (
	"testing"
	"time"

	"github.com/omeid/conex"
	"github.com/omeid/conex/runtime"
)

var (
	basicImage     = "ghcr.io/cirruslabs/macos-sequoia-base:latest"
	tartLinuxImage = "ghcr.io/cirruslabs/ubuntu:latest"
)

func init() {
	conex.Require(func() string { return basicImage })
	conex.Require(func() string { return tartLinuxImage })
}

func TestMain(m *testing.M) {
	conex.Main(m, conex.OptRuntimeType(conex.RuntimeTart))
}

// --- macOS VM tests ---

func TestTartMacBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected VM to have an IP address")
	}
	conex.Logf(t, "", "VM address: %s", c.Address())
}

func TestTartMacExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	defer c.Drop()

	conex.Logf(t, "", "VM %s running at %s", c.Name(), c.Address())
}

func TestTartMacWait(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	defer c.Drop()

	err := c.Wait("22/tcp", 60*time.Second)
	if err != nil {
		t.Fatalf("SSH port not ready: %v", err)
	}
	conex.Logf(t, "", "SSH port is accepting connections")
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
	conex.Logf(t, "", "VM address: %s", c.Address())
}

func TestTartLinuxExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: tartLinuxImage,
	})
	defer c.Drop()

	conex.Logf(t, "", "VM %s running at %s", c.Name(), c.Address())
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
	conex.Logf(t, "", "SSH port is accepting connections")
}
