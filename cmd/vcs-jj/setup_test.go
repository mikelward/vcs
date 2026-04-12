package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain chdirs to an isolated temp directory before running any tests,
// so dispatched commands (including destructive ones like undo/uncommit)
// can't touch the real repository the tests are running in.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vcs-jj-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup: MkdirTemp:", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, "setup: Chdir:", err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
