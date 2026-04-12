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
	os.Unsetenv("JJ_OP_ID")
	// Empty template dir stops init.templatedir global from installing hooks
	// into the backing git repo jj creates.
	emptyTemplate := filepath.Join(dir, "empty-template")
	os.MkdirAll(emptyTemplate, 0755)
	os.Setenv("GIT_TEMPLATE_DIR", emptyTemplate)

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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

func jjRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj %v in %s: %v\n%s", args, dir, err, out)
	}
}

func jjOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("jj %v in %s: %v", args, dir, err)
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

// newJJRepo creates a colocated jj+git repo and configures a test user.
func newJJRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	cmd := exec.Command("jj", "git", "init", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj git init: %v\n%s", err, out)
	}
	jjRun(t, repo, "config", "set", "--repo", "user.name", "Test User")
	jjRun(t, repo, "config", "set", "--repo", "user.email", "test@example.com")
	return repo
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
