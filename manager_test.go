package conex_test

import (
	"testing"

	"github.com/omeid/conex"
)

func TestOptRuntimeType(t *testing.T) {
	// Test setting runtime to RuntimeDocker
	m1 := conex.New(conex.OptRuntimeType(conex.RuntimeDocker))
	if m1 == nil {
		t.Fatalf("Expected manager to not be nil")
	}

	// Test setting runtime to RuntimeNative
	m2 := conex.New(conex.OptRuntimeType(conex.RuntimeNative))
	if m2 == nil {
		t.Fatalf("Expected manager to not be nil")
	}

	// Test setting runtime to RuntimeTart
	m3 := conex.New(conex.OptRuntimeType(conex.RuntimeVM))
	if m3 == nil {
		t.Fatalf("Expected manager to not be nil")
	}
}
