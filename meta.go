package conex

import (
	"os"
	"path/filepath"
	"strings"
)

// packageName tries to find the package name.
// func callerPackageName() string {
// 	pc, _, _, _ := runtime.Caller(2)
// 	parts := strings.Split(runtime.FuncForPC(pc).Name(), ".")
// 	pl := len(parts)
//
// 	packageName := ""
//
// 	if parts[pl-2][0] == '(' {
// 		packageName = strings.Join(parts[0:pl-2], ".")
// 	} else {
// 		packageName = strings.Join(parts[0:pl-1], ".")
// 	}
//
// 	return packageName
// }

//TODO: Find a way to use runtime et al, if possible; because
//      this is pretty hacky and may break if go build changes.
func testContainersPrefix() (string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return dir, err
	}

	dir = strings.TrimPrefix(dir, "/tmp/go-build")
	dir = strings.TrimSuffix(dir, "/_test")
	dir = "conex_" + dir

	return strings.Replace(dir, "/", "_", -1), nil
}
