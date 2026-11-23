package main

import (
	"os"
	"testing"
)

// shortTempDir returns a temp dir whose path does not include the test name.
// t.TempDir() embeds it, which can push a unix socket path past the 108-byte
// sun_path limit on Windows (long %TEMP% plus long test names).
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nudge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
