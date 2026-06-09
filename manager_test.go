package conex

import (
	"testing"
)

func TestOptRunnerType(t *testing.T) {
	// Test setting runner to RunnerDocker
	m1 := New(OptRunnerType(RunnerDocker))
	if mgr, ok := m1.(*manager); ok {
		if mgr.conf.runner != RunnerDocker {
			t.Errorf("Expected runner to be %q, got %q", RunnerDocker, mgr.conf.runner)
		}
	} else {
		t.Fatalf("Expected manager to be of type *manager")
	}

	// Test setting runner to RunnerNative
	m2 := New(OptRunnerType(RunnerNative))
	if mgr, ok := m2.(*manager); ok {
		if mgr.conf.runner != RunnerNative {
			t.Errorf("Expected runner to be %q, got %q", RunnerNative, mgr.conf.runner)
		}
	} else {
		t.Fatalf("Expected manager to be of type *manager")
	}
	
	// Test setting runner to RunnerTart
	m3 := New(OptRunnerType(RunnerTart))
	if mgr, ok := m3.(*manager); ok {
		if mgr.conf.runner != RunnerTart {
			t.Errorf("Expected runner to be %q, got %q", RunnerTart, mgr.conf.runner)
		}
	} else {
		t.Fatalf("Expected manager to be of type *manager")
	}
}
