package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain isolates test runs so destructive commands can't touch the real
// repository the tests are invoked from, and points HGRCPATH at a test hgrc
// that declares the oneline templates vcs-hg relies on.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vcs-hg-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup: MkdirTemp:", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, "setup: Chdir:", err)
		os.Exit(1)
	}
	// Clear hg environment leaking in from hooks.
	for _, k := range []string{"HG_NODE", "HG_PARENT1", "HG_PARENT2", "HG_PENDING", "HG_HOOKTYPE", "HG_HOOKNAME"} {
		os.Unsetenv(k)
	}
	// Write a minimal hgrc defining the oneline templates vcs-hg uses.
	hgrc := filepath.Join(dir, "hgrc")
	body := "[templates]\n" +
		"shortnode = \"{label('log.changeset', shortest(node, 7))}\"\n" +
		"shortdesc = \"{desc|firstline}\"\n" +
		"onelinesummary = \"{shortnode} {shortdesc}\"\n" +
		"oneline = \"{shortnode} {shortdesc}\\n\"\n" +
		"[ui]\n" +
		"username = Test User <test@example.com>\n" +
		"[extensions]\n" +
		"rebase =\n"
	if err := os.WriteFile(hgrc, []byte(body), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "setup: WriteFile hgrc:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	os.Setenv("HGRCPATH", hgrc)

	// Resolve hgCmd for dispatch. Use chg if present (faster), else hg.
	if p, err := exec.LookPath("chg"); err == nil {
		hgCmd = p
	} else {
		hgCmd = "hg"
	}

	// Three different situations, and collapsing any two of them is how a
	// green run comes to mean nothing:
	//
	//   absent  — nothing on PATH is called hg. Legitimate; the hg-backed
	//             tests skip themselves via requireHg and say so, while the
	//             tests that never shell out to hg still run.
	//   broken  — hg is on PATH but will not execute, either because the file
	//             has no execute bits or because running it fails. That is a
	//             broken toolchain, not an absent one, and it must fail:
	//             exiting 0 here made `go test ./...` report ok having run
	//             nothing.
	if _, err := exec.LookPath(hgCmd); err != nil {
		if blocked, found := unrunnableOnPath(hgCmd); found {
			fmt.Fprintf(os.Stderr, "setup: %s is on PATH but will not run: %s is not executable\n", hgCmd, blocked)
			os.RemoveAll(dir)
			os.Exit(1)
		}
		hgUnavailable = fmt.Sprintf("hg not found in PATH: %v", err)
	} else if out, err := exec.Command(hgCmd, "--version").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "setup: %s is on PATH but will not run: %v\n%s", hgCmd, err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// unrunnableOnPath reports a file that a PATH lookup found by name but could
// not run. exec.LookPath returns the same ErrNotFound whether nothing on PATH
// carries the name at all or a file of that name has lost its execute bits, so
// on its own it files a broken install under "absent" — the dependent tests
// then skip and the package reports ok, which is the false green this setup
// exists to prevent. Call it only after LookPath has already failed: any match
// it finds is by construction one LookPath declined to run.
func unrunnableOnPath(name string) (path string, found bool) {
	if strings.ContainsRune(name, filepath.Separator) {
		// A name with a separator is used as-is; PATH is not consulted.
		if fi, err := os.Stat(name); err == nil && fi.Mode().IsRegular() {
			return name, true
		}
		return "", false
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "." // an empty PATH entry means the current directory
		}
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && fi.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

// hgUnavailable is empty when hg is usable; otherwise it says why, and
// requireHg skips on it.
var hgUnavailable string

// requireHg skips the calling test when hg is not installed. A skip is
// reported per test, so `go test` names what did not run instead of a bare
// ok that hides an empty suite.
func requireHg(t *testing.T) {
	t.Helper()
	if hgUnavailable != "" {
		t.Skip(hgUnavailable)
	}
}

// captureIO redirects os.Stdout/err into a pipe during fn, returns output and err.
func captureIO(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
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
	return string(<-done), runErr
}

func inDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	fn()
}

func runDispatch(t *testing.T, dir, subcmd string, args ...string) (string, error) {
	t.Helper()
	var out string
	var err error
	inDir(t, dir, func() {
		out, err = captureIO(t, func() error { return dispatch(subcmd, args) })
	})
	return out, err
}

func hgRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	requireHg(t)
	cmd := exec.Command(hgCmd, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hg %v in %s: %v\n%s", args, dir, err, out)
	}
}

func hgOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	requireHg(t)
	cmd := exec.Command(hgCmd, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hg %v in %s: %v", args, dir, err)
	}
	return strings.TrimRight(string(out), "\n")
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0755)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// newHgRepo creates a remote repo with an initial commit, and clones it to
// local. Returns (remote, local).
func newHgRepo(t *testing.T) (remote, local string) {
	t.Helper()
	requireHg(t)
	root := t.TempDir()
	remote = filepath.Join(root, "remote")
	local = filepath.Join(root, "local")

	hgRun(t, ".", "init", remote)
	writeFile(t, remote, "file.txt", "initial\n")
	hgRun(t, remote, "add", "file.txt")
	hgRun(t, remote, "commit", "-m", "initial commit", "-u", "test <test@example.com>")

	hgRun(t, ".", "clone", "-q", remote, local)
	return remote, local
}

func editorStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "editor.sh")
	os.WriteFile(p, []byte("#!/bin/sh\necho 'edited by test' > \"$1\"\n"), 0755)
	return p
}

func setenv(t *testing.T, k, v string) {
	t.Helper()
	old, had := os.LookupEnv(k)
	os.Setenv(k, v)
	t.Cleanup(func() {
		if had {
			os.Setenv(k, old)
		} else {
			os.Unsetenv(k)
		}
	})
}
