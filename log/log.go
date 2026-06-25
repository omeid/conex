package log

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	testLogsMu sync.Mutex
	testLogs   = make(map[testing.TB][]string)
)

func isParallel(t testing.TB) bool {

	if t == nil {
		return false
	}

	parallel := runtime.GOMAXPROCS(0) > 1
	pf := flag.Lookup("test.parallel")

	if pf != nil {
		parallel = pf.Value.String() != "1"
	}

	return parallel
}

// Logf logs directly to stdout to avoid the test file and line number prefix.
// If a testing.TB is provided, it buffers the logs ONLY if there are parallel
// tests running, to prevent interleaving while allowing real-time logs otherwise.
func Logf(t testing.TB, plugin string, f string, args ...any) {
	if len(f) > 0 && f[0] >= 'a' && f[0] <= 'z' {
		f = string(f[0]-32) + f[1:]
	}

	prefix := ""
	if plugin != "" {
		prefix = plugin + ": "
	}

	msg := fmt.Sprintf("    "+prefix+f, args...)

	if isParallel(t) {
		testLogsMu.Lock()
		defer testLogsMu.Unlock()

		if _, exists := testLogs[t]; !exists {
			testLogs[t] = []string{}
			t.Cleanup(func() {
				testLogsMu.Lock()
				logs := testLogs[t]
				delete(testLogs, t)
				testLogsMu.Unlock()

				if len(logs) > 0 {
					var out strings.Builder
					for _, l := range logs {
						out.WriteString(l)
						out.WriteRune('\n')
					}
					fmt.Print(out.String())
				}
			})
		}
		fmt.Printf("--- cached: %s ---\n", msg)
		testLogs[t] = append(testLogs[t], msg)
	} else {
		fmt.Println(msg)
	}
}

// Same story as above.
func Fatalf(t testing.TB, name string, f string, args ...any) {
	plugin := "conex"
	if name != "" {
		plugin += " " + name
	}
	Logf(t, plugin, f, args...)
	t.FailNow()
}
