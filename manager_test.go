package conex_test

import (
	"testing"

	"github.com/omeid/conex"
)

func TestOptRunnerType(t *testing.T) {
	// Test setting runner to RunnerDocker
	m1 := conex.New(conex.OptRunnerType(conex.RunnerDocker))
	if m1 == nil {
		t.Fatalf("Expected manager to not be nil")
	}

	// Test setting runner to RunnerNative
	m2 := conex.New(conex.OptRunnerType(conex.RunnerNative))
	if m2 == nil {
		t.Fatalf("Expected manager to not be nil")
	}

	// Test setting runner to RunnerTart
	m3 := conex.New(conex.OptRunnerType(conex.RunnerTart))
	if m3 == nil {
		t.Fatalf("Expected manager to not be nil")
	}
}
