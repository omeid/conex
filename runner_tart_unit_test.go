package conex

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestTartIPWaitProcessExit verifies that tartIPWait returns immediately
// when the VM process exits instead of waiting for the full timeout.
// This is the core of the locked-keychain fix: without it, a process
// that dies on start (e.g. keychain locked) would cause a 120 s timeout
// with a misleading "timeout waiting for IP" error.
func TestTartIPWaitProcessExit(t *testing.T) {
	exited := make(chan error, 1)
	exited <- errors.New("exit status 1")

	start := time.Now()
	_, err := tartIPWait("test-vm", 30*time.Second, exited)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from tartIPWait when process exits")
	}

	if !strings.Contains(err.Error(), "VM process exited") {
		t.Fatalf("expected 'VM process exited' error, got: %s", err)
	}

	// Must return almost immediately, not anywhere near the 30 s timeout.
	if elapsed > 5*time.Second {
		t.Fatalf("tartIPWait took %s; expected immediate return on process exit", elapsed)
	}
}

// TestTartIPWaitTimeout verifies the timeout path still works when the
// process stays alive but no IP is ever assigned.
func TestTartIPWaitTimeout(t *testing.T) {
	// Channel that never receives — simulates a running process.
	exited := make(chan error, 1)

	start := time.Now()
	_, err := tartIPWait("nonexistent-vm", 3*time.Second, exited)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error from tartIPWait")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %s", err)
	}

	if elapsed < 2*time.Second || elapsed > 10*time.Second {
		t.Fatalf("tartIPWait took %s; expected ~3s", elapsed)
	}
}

// TestTartIPWaitProcessExitDuringPoll verifies that if the process dies
// mid-poll (not pre-filled), tartIPWait still detects it promptly.
func TestTartIPWaitProcessExitDuringPoll(t *testing.T) {
	exited := make(chan error, 1)

	// Simulate a process that dies after 1 second.
	go func() {
		time.Sleep(1 * time.Second)
		exited <- errors.New("signal: killed")
	}()

	start := time.Now()
	_, err := tartIPWait("test-vm", 30*time.Second, exited)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from tartIPWait when process exits mid-poll")
	}

	if !strings.Contains(err.Error(), "VM process exited") {
		t.Fatalf("expected 'VM process exited' error, got: %s", err)
	}

	// Should return shortly after the 1 s delay, well before the 30 s timeout.
	if elapsed > 10*time.Second {
		t.Fatalf("tartIPWait took %s; expected prompt return after process exit", elapsed)
	}
}

func TestSanitizeTartName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with/slash", "with-slash"},
		{"with space", "with-space"},
		{"with:colon", "with-colon"},
		{"a/b:c d", "a-b-c-d"},
	}

	for _, tt := range tests {
		got := sanitizeTartName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeTartName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
