package docker_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/omeid/conex"
)

func TestWaitAndLogs(t *testing.T) {
	c := conex.Box(t, &conex.Config{
		Image: basicImage,
		Cmd:   []string{"sh", "-c", "echo 'hello conex logs' && echo 'hello conex error' >&2 && sleep 10"},
	})

	// 1. Test Wait timeout on a non-existent port
	err := c.Wait("12345", 2*time.Second)
	if err == nil {
		t.Fatal("Expected Wait to fail on port 12345, but it succeeded")
	}

	// 2. Test Logs capture
	var stdout, stderr bytes.Buffer
	err = c.Logs(&stdout, &stderr)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	logStr := stdout.String() + stderr.String()
	if !strings.Contains(logStr, "hello conex logs") {
		t.Fatalf("Expected logs to contain 'hello conex logs', got: %q", logStr)
	}
	if !strings.Contains(logStr, "hello conex error") {
		t.Fatalf("Expected logs to contain 'hello conex error', got: %q", logStr)
	}
}
