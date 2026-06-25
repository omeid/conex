package vm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	iruntime "github.com/omeid/conex/internal/runtime"
	"github.com/omeid/conex/log"
	"github.com/omeid/conex/runtime"
)

var vmCommand = "tart"

func init() {
	var _ iruntime.Runtime = (*vmRuntime)(nil)
	var _ runtime.Container = (*vmContainer)(nil)
}

// tartRuntime runs tests on the host machine and manages Tart VMs
// as containers. VMs are cloned from base images and accessed via
// their direct IP addresses.
type vmRuntime struct {
	config  *iruntime.Config
	counter iruntime.Counter
}

type prefixWriter struct {
	w       io.Writer
	prefix  []byte
	newLine bool
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	var buf bytes.Buffer
	for _, b := range p {
		if pw.newLine {
			buf.Write(pw.prefix)
			pw.newLine = false
		}
		buf.WriteByte(b)
		if b == '\n' || b == '\r' {
			pw.newLine = true
		}
	}
	_, err = pw.w.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (r *vmRuntime) Pull(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, vmCommand, "pull", image)
	pw := &prefixWriter{
		w:       os.Stdout,
		prefix:  []byte("        "),
		newLine: true,
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull tart image %s: %w", image, err)
	}
	return nil
}

func (r *vmRuntime) Ensure(ctx context.Context, image string) (string, error) {
	return "", nil
}

func (r *vmRuntime) Build(ctx context.Context, image string, tag string) error {
	return fmt.Errorf("tart runtime does not support Dockerfile image refs: %s", image)
}

// NewTartRuntime creates a new runtime that runs tests in Tart VMs.
func NewVMRuntime(config *iruntime.Config) iruntime.Runtime {
	return &vmRuntime{
		config:  config,
		counter: iruntime.NewCounter(),
	}
}

// Run executes the tests directly on the host.
func (r *vmRuntime) Run(m *testing.M) int {
	return m.Run()
}

// Box clones a Tart VM from the given image and starts it.
// The Config.Image field specifies the Tart VM image to clone from.
// Cmd, Env, and Expose are supported through tart exec after boot.
func (r *vmRuntime) Box(t testing.TB, conf *runtime.Config, name string) runtime.Container {
	if len(conf.Binds) > 0 {
		log.Fatalf(t, "", "tart runtime does not support Binds (volume mounts)")
	}

	cname := conf.Image
	if len(conf.Cmd) != 0 {
		cname = cname + ": " + strings.Join(conf.Cmd, " ")
	}

	// Sanitize the name for tart (only alphanumeric, hyphens, underscores, dots)
	vmName := sanitizeVMName(name)
	vmName = fmt.Sprintf("%s_%d", vmName, r.counter.Count(vmName))

	log.Logf(t, "conex", "creating (%s) as %s", cname, vmName)

	// Clone from base image.
	if _, err := vmCmd("clone", conf.Image, vmName); err != nil {
		log.Fatalf(t, vmName, "Failed to clone Tart image %s: %s", conf.Image, err)
	}

	// Start VM in background, capturing stderr so we can report
	// failures that happen after the process is spawned (e.g.
	// locked keychain, permission errors).
	logs := new(safeBuffer)
	cmd := exec.Command(vmCommand, "run", "--no-graphics", vmName)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		if _, deleteErr := vmCmd("delete", vmName); deleteErr != nil {
			log.Fatalf(t, vmName, "Failed to delete VM after start failure: %s", deleteErr)
		}
		log.Fatalf(t, vmName, "Failed to start VM: %s", err)
	}

	// Monitor the process so we can detect early exits.
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	// Give the process a moment to fail on obvious errors (e.g.
	// locked keychain) before we start the longer IP-wait loop.
	select {
	case waitErr := <-exited:
		if waitErr != nil {
			log.Fatalf(t, vmName, "Tart VM process exited unexpectedly: %s: %s", waitErr, logs.String())
		} else {
			log.Fatalf(t, vmName, "Tart VM process exited cleanly but unexpectedly: %s", logs.String())
		}
	case <-time.After(500 * time.Millisecond):
		// Process still running, proceed.
	}

	log.Logf(t, "conex", "started (%s) as %s", cname, vmName)

	// Wait for VM to get an IP, aborting early if the process exits.
	ip, err := vmIPWait(vmName, 120*time.Second, exited)
	if err != nil {
		// If the error is from a timeout (process still running), kill it.
		// If the process already exited, Kill is harmless.
		_ = cmd.Process.Kill()
		// Wait for the process to finish. The channel may have already
		// been consumed by tartIPWait (process-exit case), so use a
		// timeout to avoid blocking forever.
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
		if _, deleteErr := vmCmd("delete", vmName); deleteErr != nil {
			log.Fatalf(t, vmName, "Failed to delete VM after IP wait failure: %s", deleteErr)
		}
		log.Fatalf(t, vmName, "VM failed to get IP: %s: %s", err, logs.String())
	}

	log.Logf(t, "conex", "VM %s has IP %s", vmName, ip)

	c := &vmContainer{
		vmName: vmName,
		image:  conf.Image,
		ip:     ip,
		cmd:    cmd,
		exited: exited,
		t:      t,
		logs:   logs,
	}

	// Run startup command if provided.
	if len(conf.Cmd) > 0 {
		cmdStr := strings.Join(conf.Cmd, " ")
		if _, err := vmExec(vmName, cmdStr); err != nil {
			c.Drop()
			log.Fatalf(t, c.vmName, "Failed to run startup command: %s", err)
		}
	}

	return c
}

// tartContainer implements Container for Tart VMs.
type vmContainer struct {
	vmName   string
	image    string
	ip       string
	cmd      *exec.Cmd
	exited   <-chan error
	t        testing.TB
	dropOnce sync.Once
	logs     *safeBuffer
}

func (c *vmContainer) ID() string {
	return c.vmName
}

func (c *vmContainer) Image() string {
	return c.image
}

func (c *vmContainer) Name() string {
	return c.vmName
}

func (c *vmContainer) Address() string {
	return c.ip
}

func (c *vmContainer) Drop() {
	c.dropOnce.Do(func() {
		// Stop the VM.
		if _, err := vmCmd("stop", c.vmName); err != nil {
			c.t.Fatalf("Failed to stop VM: %s", err)
		}
		if c.exited != nil {
			// Wait for the background goroutine monitoring cmd.Wait() to finish.
			<-c.exited
		}

		// Delete the VM.
		if _, err := vmCmd("delete", c.vmName); err != nil {
			c.t.Fatalf("Failed to delete VM: %s", err)
		}
	})
}

// Wait pings the given port until it responds or timeout occurs.
func (c *vmContainer) Wait(port string, timeout time.Duration) error {
	err := iruntime.Wait(c.ip, port, timeout)
	if err != nil && testing.Verbose() {
		log.Logf(nil, "conex", "=== Container %s Logs ===\n", c.Name())
		_ = c.Logs(os.Stdout, os.Stderr)
		log.Logf(nil, "conex", "=========================\n")
	}
	return err
}

func (c *vmContainer) Logs(stdout io.Writer, stderr io.Writer) error {
	b := c.logs.Bytes()
	if stdout != nil {
		if _, err := stdout.Write(b); err != nil {
			return err
		}
	} else if stderr != nil {
		if _, err := stderr.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func (c *vmContainer) Exec(cmd ...string) *runtime.Cmd {
	if len(cmd) == 0 {
		return nil
	}

	cmdObj := &runtime.Cmd{
		Path:   cmd[0],
		Args:   cmd,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	args := append([]string{"exec", c.vmName}, cmd...)
	osCmd := exec.Command("tart", args...)

	start := func() error {
		// Pass through current environment plus the configured environment.
		osCmd.Env = append(os.Environ(), cmdObj.Env...)
		osCmd.Dir = cmdObj.Dir
		osCmd.Stdin = cmdObj.Stdin
		osCmd.Stdout = cmdObj.Stdout
		osCmd.Stderr = cmdObj.Stderr
		return osCmd.Start()
	}

	wait := func() error {
		return osCmd.Wait()
	}

	return runtime.WireCommand(cmdObj, start, wait)
}

// vmCmd runs a vm command and returns its combined output.
func vmCmd(args ...string) (string, error) {
	cmd := exec.Command(vmCommand, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tart %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// tartExec runs a command inside a Tart VM.
func vmExec(vmName, cmd string) (string, error) {
	return vmCmd("exec", vmName, "sh", "-c", cmd)
}

// tartIPWait waits for a Tart VM to get an IP address.
// If exited is non-nil it is checked on every tick; a value means the VM
// process died and there is no point waiting further.
func vmIPWait(vmName string, timeout time.Duration, exited <-chan error) (string, error) {
	deadline := time.After(timeout)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case err := <-exited:
			return "", fmt.Errorf("VM process exited while waiting for IP: %v", err)
		case <-deadline:
			return "", fmt.Errorf("timeout waiting for VM %s IP", vmName)
		case <-tick.C:
			out, err := vmCmd("ip", vmName)
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
func sanitizeVMName(name string) string {
	r := strings.NewReplacer("/", "-", " ", "-", ":", "-")
	return r.Replace(name)
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *safeBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buf.Bytes()
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}
