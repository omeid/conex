//go:build !tart

package conex_test

import "testing"

func TestTartSkipped(t *testing.T) {
	t.Skip("tart tests require macOS with Tart installed. Run with: go test -tags tart -run TestTart ./...")
}
