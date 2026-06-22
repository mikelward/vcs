package main

import (
	"fmt"
	"io"
	"os"
	"testing"
)

// TestMain chdirs to an isolated temp directory before running any tests.
func TestMain(m *testing.M) {
	p4Cmd = findP4()
	dir, err := os.MkdirTemp("", "vcs-p4-test-")
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

// captureIO runs fn with os.Stdout and os.Stderr redirected into a single
// pipe, returning everything written and fn's error.
func captureIO(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w

	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()

	runErr := fn()
	w.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	buf := <-done
	return string(buf), runErr
}

// inDir chdirs to dir, runs fn, then restores the original directory.
func inDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer os.Chdir(old)
	fn()
}

// runDispatch chdirs to dir, calls dispatch(subcmd, args) with stdout/stderr
// captured, and restores the original dir.
func runDispatch(t *testing.T, dir, subcmd string, args ...string) (string, error) {
	t.Helper()
	var out string
	var err error
	inDir(t, dir, func() {
		out, err = captureIO(t, func() error { return dispatch(subcmd, args) })
	})
	return out, err
}
