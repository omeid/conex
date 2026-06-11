//go:build !tart

package conex_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/omeid/conex"
)

func TestExecCat(t *testing.T) {
	t.Parallel()

	conf := &conex.Config{
		Image: "alpine",
		Cmd:   []string{"sleep", "1000"}, // Keep the container running
	}

	c := conex.Box(t, conf)
	defer c.Drop()

	inputData := "Hello from host to container buffer and back!"

	// 1. cat from host to container
	// We run `sh -c "cat > /tmp/exec_test.txt"` and provide Stdin.
	var writeErrBuf bytes.Buffer
	cmdWrite := c.Exec("sh", "-c", "cat > /tmp/exec_test.txt")
	cmdWrite.Stdin = strings.NewReader(inputData)
	cmdWrite.Stderr = &writeErrBuf

	err := cmdWrite.Run()
	if err != nil {
		t.Fatalf("Failed to write to container using exec: %v, stderr: %q", err, writeErrBuf.String())
	}

	// 2. cat from container to a host buffer
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmdRead := c.Exec("cat", "/tmp/exec_test.txt")
	cmdRead.Stdout = &outBuf
	cmdRead.Stderr = &errBuf

	err = cmdRead.Run()
	if err != nil {
		t.Fatalf("Failed to read from container using exec: %v, stderr: %q", err, errBuf.String())
	}

	if outBuf.String() != inputData {
		t.Fatalf("Expected output %q, got %q", inputData, outBuf.String())
	}
}

func TestExecNullIO(t *testing.T) {
	t.Parallel()

	conf := &conex.Config{
		Image: "alpine",
		Cmd:   []string{"sleep", "1000"}, // Keep the container running
	}

	c := conex.Box(t, conf)
	defer c.Drop()

	// nil stdout
	cmd1 := c.Exec("echo", "hello")
	cmd1.Stdout = nil
	cmd1.Stderr = io.Discard
	if err := cmd1.Run(); err == nil || !strings.Contains(err.Error(), "Stdout must be set to an io.Writer") {
		t.Fatalf("Expected Stdout must be set error, got: %v", err)
	}

	// nil stderr
	cmd2 := c.Exec("echo", "hello")
	cmd2.Stdout = io.Discard
	cmd2.Stderr = nil
	if err := cmd2.Run(); err == nil || !strings.Contains(err.Error(), "Stderr must be set to an io.Writer") {
		t.Fatalf("Expected Stderr must be set error, got: %v", err)
	}

	// Both set to io.Discard should pass
	cmd3 := c.Exec("echo", "hello")
	cmd3.Stdout = io.Discard
	cmd3.Stderr = io.Discard
	if err := cmd3.Run(); err != nil {
		t.Fatalf("Expected successful run with io.Discard, got: %v", err)
	}
}

func TestExecDefaultsToDiscard(t *testing.T) {
	conf := &conex.Config{
		Image: "alpine",
		Cmd:   []string{"sleep", "1000"},
	}

	c := conex.Box(t, conf)
	defer c.Drop()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	os.Stdout = nil
	os.Stderr = nil

	cmd := c.Exec("echo", "hello")

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if cmd.Stdout != io.Discard {
		t.Fatalf("Expected cmd.Stdout to be io.Discard when os.Stdout is nil")
	}
	if cmd.Stderr != io.Discard {
		t.Fatalf("Expected cmd.Stderr to be io.Discard when os.Stderr is nil")
	}
}
