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
	cmdWrite := c.Exec("sh", "-c", "cat > /tmp/exec_test.txt")
	cmdWrite.Stdin = strings.NewReader(inputData)

	err := cmdWrite.Run()
	if err != nil {
		t.Fatalf("Failed to write to container using exec: %v", err)
	}

	// 2. cat from container to a host buffer
	var outBuf bytes.Buffer
	cmdRead := c.Exec("cat", "/tmp/exec_test.txt")
	cmdRead.Stdout = &outBuf

	err = cmdRead.Run()
	if err != nil {
		t.Fatalf("Failed to read from container using exec: %v", err)
	}

	if outBuf.String() != inputData {
		t.Fatalf("Expected output %q, got %q", inputData, outBuf.String())
	}
}
