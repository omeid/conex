package conex

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	// RunnerTart runs VMs using Tart virtualization.
	// Container IPs are directly accessible from the host.
	RunnerTart RunnerType = "tart"
)

func init() {
	var _ Runner = (*TartRunner)(nil)
	var _ Container = (*tartContainer)(nil)
}

// TartRunner runs tests on the host machine and manages Tart VMs
// as containers. VMs are cloned from base images and accessed via
// their direct IP addresses.
type TartRunner struct {
	config *RunnerConfig
}

// NewTartRunner creates a new tart runner.
func NewTartRunner(config *RunnerConfig) *TartRunner {
	return &TartRunner{config: config}
}

// Run executes the tests directly on the host.
func (r *TartRunner) Run(m *testing.M) int {
	return m.Run()
}

// Box clones a Tart VM from the given image and starts it.
// The Config.Image field specifies the Tart VM image to clone from.
// Cmd, Env, and Expose are supported through tart exec after boot.
func (r *TartRunner) Box(t testing.TB, conf *Config, name string) Container {
	cname := conf.Image
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	// Sanitize the name for tart (only alphanumeric, hyphens, underscores, dots)
	vmName := sanitizeTartName(name)

	logf(t, "creating (%s) as %s", cname, vmName)

	// Clone from base image.
	if _, err := tartCmd("clone", conf.Image, vmName); err != nil {
		fatalf(t, "Failed to clone VM: %s", err)
	}

	// Start VM in background.
	cmd := exec.Command("tart", "run", "--no-graphics", vmName)
	if err := cmd.Start(); err != nil {
		tartCmd("delete", vmName)
		fatalf(t, "Failed to start VM: %s", err)
	}

	logf(t, "started (%s) as %s", cname, vmName)

	// Wait for VM to get an IP.
	ip, err := tartIPWait(vmName, 120*time.Second)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		tartCmd("delete", vmName)
		fatalf(t, "VM failed to get IP: %s", err)
	}

	logf(t, "VM %s has IP %s", vmName, ip)

	c := &tartContainer{
		vmName: vmName,
		image:  conf.Image,
		ip:     ip,
		cmd:    cmd,
		t:      t,
	}

	// Run startup command if provided.
	if len(conf.Cmd) > 0 {
		cmdStr := strings.Join(conf.Cmd, " ")
		if _, err := tartExec(vmName, cmdStr); err != nil {
			c.Drop()
			fatalf(t, "Failed to run startup command: %s", err)
		}
	}

	return c
}

// tartContainer implements Container for Tart VMs.
type tartContainer struct {
	vmName string
	image  string
	ip     string
	cmd    *exec.Cmd
	t      testing.TB
}

func (c *tartContainer) ID() string {
	return c.vmName
}

func (c *tartContainer) Image() string {
	return c.image
}

func (c *tartContainer) Name() string {
	return c.vmName
}

func (c *tartContainer) Address() string {
	return c.ip
}

func (c *tartContainer) Drop() {
	// Stop the VM.
	tartCmd("stop", c.vmName)
	if c.cmd != nil {
		c.cmd.Wait()
	}

	// Delete the VM.
	if _, err := tartCmd("delete", c.vmName); err != nil {
		c.t.Log("failed to delete VM:", c.vmName, err)
	}
}

func (c *tartContainer) Wait(port string, timeout time.Duration) error {
	return wait(c.ip, port, timeout)
}

// tartCmd runs a tart command and returns its combined output.
func tartCmd(args ...string) (string, error) {
	cmd := exec.Command("tart", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tart %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// tartExec runs a command inside a Tart VM.
func tartExec(vmName, cmd string) (string, error) {
	return tartCmd("exec", vmName, "sh", "-c", cmd)
}

// tartIPWait waits for a Tart VM to get an IP address.
func tartIPWait(vmName string, timeout time.Duration) (string, error) {
	deadline := time.After(timeout)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			return "", fmt.Errorf("timeout waiting for VM %s IP", vmName)
		case <-tick.C:
			out, err := tartCmd("ip", vmName)
			if err == nil {
				ip := strings.TrimSpace(out)
				if ip != "" {
					return ip, nil
				}
			}
		}
	}
}

// sanitizeTartName ensures the VM name is valid for Tart.
func sanitizeTartName(name string) string {
	r := strings.NewReplacer("/", "-", " ", "-", ":", "-")
	return r.Replace(name)
}
