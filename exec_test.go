//go:build !tart

package conex_test

import (
	"bytes"
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
