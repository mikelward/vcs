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

// TestMain chdirs to an isolated temp directory before running any tests,
// so dispatched commands (including destructive ones like undo/uncommit)
// can't touch the real repository the tests are running in.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vcs-git-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup: MkdirTemp:", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, "setup: Chdir:", err)
		os.Exit(1)
	}
	// Clear environment that leaks in when tests are run from a git hook.
	os.Unsetenv("GIT_DIR")
	os.Unsetenv("GIT_INDEX_FILE")
	os.Unsetenv("GIT_WORK_TREE")
	os.Unsetenv("GIT_PREFIX")
	// Override init.templatedir: the user's template may install hooks
	// (e.g. a pre-commit) that would run against test repos.
	emptyTemplate := filepath.Join(dir, "empty-template")
	os.MkdirAll(emptyTemplate, 0755)
	os.Setenv("GIT_TEMPLATE_DIR", emptyTemplate)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// captureIO runs fn with os.Stdout and os.Stderr redirected into a single
// pipe, returning everything written and fn's error. Subprocesses spawned
// by fn inherit the pipe through runner.Run.
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
// captured, and restores the original dir. Returns the captured output and
// dispatch's return error.
func runDispatch(t *testing.T, dir, subcmd string, args ...string) (string, error) {
	t.Helper()
	var out string
	var err error
	inDir(t, dir, func() {
		out, err = captureIO(t, func() error { return dispatch(subcmd, args) })
	})
	return out, err
}

// gitRun runs `git <args>` in dir and fails the test if it errors.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// gitOut runs `git <args>` in dir, trims trailing newline, returns stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimRight(string(out), "\n")
}

// writeFile writes content into dir/name, creating parent dirs as needed.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// newGitRepo creates a bare remote and a local clone with one initial
// commit pushed. Returns (remote, local). Commit signing and hooks are
// disabled so commits are reproducible.
func newGitRepo(t *testing.T) (remote, local string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	local = filepath.Join(root, "local")

	gitRun(t, ".", "init", "--bare", "-b", "main", remote)

	// git clone runs in parent dir.
	cmd := exec.Command("git", "clone", "-q", remote, local)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}

	gitRun(t, local, "config", "commit.gpgsign", "false")
	gitRun(t, local, "config", "tag.gpgsign", "false")
	gitRun(t, local, "config", "user.email", "test@example.com")
	gitRun(t, local, "config", "user.name", "Test User")
	// Disable hooks so core.hooksPath=/dev/null style doesn't matter.
	gitRun(t, local, "config", "core.hooksPath", filepath.Join(root, "nohooks"))

	gitRun(t, local, "commit", "--allow-empty", "-m", "initial commit")
	gitRun(t, local, "push", "-u", "origin", "HEAD")
	return remote, local
}

// editorStub returns a path to a short shell script that rewrites the
// commit message file to `edited by test`, usable as GIT_EDITOR.
func editorStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "editor.sh")
	body := "#!/bin/sh\necho 'edited by test' > \"$1\"\n"
	if err := os.WriteFile(p, []byte(body), 0755); err != nil {
		t.Fatalf("write editor: %v", err)
	}
	return p
}

// setenv sets an env var for the duration of the test.
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
