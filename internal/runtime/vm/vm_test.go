//go:build vm

package vm_test

import (
	"testing"
	"time"

	"github.com/omeid/conex"
	"github.com/omeid/conex/runtime"
)

var (
	basicImage   = "ghcr.io/cirruslabs/macos-sequoia-base:latest"
	vmLinuxImage = "ghcr.io/cirruslabs/ubuntu:latest"
)

func init() {
	conex.Require(func() string { return basicImage })
	conex.Require(func() string { return vmLinuxImage })
}

func TestMain(m *testing.M) {
	conex.Main(m, conex.OptRuntimeType(conex.RuntimeVM))
}

// --- macOS VM tests ---

func TestVMMacBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected VM to have an IP address")
	}
	conex.Logf(t, "", "VM address: %s", c.Address())
}

func TestVMMacExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
	})
	defer c.Drop()

	conex.Logf(t, "", "VM %s running at %s", c.Name(), c.Address())
}

func TestVMMacWait(t *testing.T) {
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

func TestVMLinuxBox(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: vmLinuxImage,
	})
	defer c.Drop()

	if c.Address() == "" {
		t.Fatal("expected VM to have an IP address")
	}
	conex.Logf(t, "", "VM address: %s", c.Address())
}

func TestVMLinuxExec(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: vmLinuxImage,
	})
	defer c.Drop()

	conex.Logf(t, "", "VM %s running at %s", c.Name(), c.Address())
}

func TestVMLinuxWait(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: vmLinuxImage,
	})
	defer c.Drop()

	err := c.Wait("22/tcp", 60*time.Second)
	if err != nil {
		t.Fatalf("SSH port not ready: %v", err)
	}
	conex.Logf(t, "", "SSH port is accepting connections")
}
