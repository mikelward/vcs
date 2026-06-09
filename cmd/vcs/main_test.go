package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikelward/vcs/runner"
)

func TestClearCache(t *testing.T) {
	dir := t.TempDir()
	// Create nested .vcs_cache files.
	os.WriteFile(filepath.Join(dir, ".vcs_cache"), []byte("test"), 0666)
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, ".vcs_cache"), []byte("test"), 0666)

	// clearCache operates on cwd.
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	clearCache()

	if _, err := os.Stat(filepath.Join(dir, ".vcs_cache")); err == nil {
		t.Error("top-level .vcs_cache should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(sub, ".vcs_cache")); err == nil {
		t.Error("sub/.vcs_cache should have been deleted")
	}
}

func TestDetectForced(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	info := detect("hg")
	if info.VCS != "hg" {
		t.Errorf("expected forced VCS hg, got %s", info.VCS)
	}
}

func TestDetectAtPath(t *testing.T) {
	// A git repo at dir, and we detect from an unrelated cwd.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	other := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(other)
	defer os.Chdir(orig)

	info := detectAt("", dir)
	if info.VCS != "git" {
		t.Errorf("expected git, got %s", info.VCS)
	}
}

func TestDetectDirFile(t *testing.T) {
	// Passing a file path should resolve to its containing directory.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	file := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(file, []byte("hi"), 0666); err != nil {
		t.Fatal(err)
	}

	got, err := detectDir([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestDetectDirDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := detectDir([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestDetectDirNoArgs(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(orig)

	got, err := detectDir(nil)
	if err != nil {
		t.Fatal(err)
	}
	// On macOS t.TempDir() paths go through /private/var symlinks; compare
	// via EvalSymlinks to avoid platform-specific flakiness.
	gotResolved, _ := filepath.EvalSymlinks(got)
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if gotResolved != dirResolved {
		t.Errorf("expected %s, got %s", dirResolved, gotResolved)
	}
}

func TestDetectDirMissing(t *testing.T) {
	if _, err := detectDir([]string{"/nonexistent/path/xyzzy"}); err == nil {
		t.Error("expected error for missing path")
	}
}

// TestPromptInfoStripsHgPathPrefix covers the same regression as
// TestAutoFetchCmdStripsHgPathPrefix (below) for the prompt-info
// subcommand: main()'s flag parser stores --hg-path as the literal flag
// string (`--hg-path=PATH`) for passthrough to vcs-hg, and promptInfo
// must strip the prefix before handing it to promptinfo.Options.HgPath,
// which is used directly as the hg binary path. A fake hg script reports
// a dirty file; if the prefix leaks through, the exec fails and the
// status field comes back empty.
func TestPromptInfoStripsHgPathPrefix(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".hg", "store"), 0755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fakehg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'M dirty.txt'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	// Capture stdout (promptInfo prints the formatted result there).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevStdout := os.Stdout
	os.Stdout = w

	promptInfo("", "--hg-path="+fake, []string{"--format={status}", "--color=never"})

	w.Close()
	os.Stdout = prevStdout
	out, _ := io.ReadAll(r)

	if got := strings.TrimSpace(string(out)); got != "*" {
		t.Errorf("prompt-info status = %q, want %q (hg path prefix not stripped?)", got, "*")
	}
}

// TestAutoFetchCmdStripsHgPathPrefix covers the regression where
// main()'s flag parser stores --hg-path as the literal flag string
// (`--hg-path=PATH`) for passthrough to vcs-hg, but autoFetchCmd
// previously forwarded that string into autofetch.Options.HgPath,
// which expects just the path. dispatch() would then try to spawn
// a binary literally named `--hg-path=PATH`. We exercise the strip
// via dry-run so detachedSpawn calls runner.PrintCommand instead of
// forking, and verify the printed command names the bare path.
func TestAutoFetchCmdStripsHgPathPrefix(t *testing.T) {
	prevDry := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = prevDry })

	// Fake hg repo: vcs detect keys off the .hg directory and an
	// empty .hg/store/ is enough for the dispatch path to fire.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".hg", "store"), 0755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	// Capture stderr (runner.PrintCommand writes there).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevStderr := os.Stderr
	os.Stderr = w

	autoFetchCmd("", "--hg-path=/usr/local/bin/chg", nil)

	w.Close()
	os.Stderr = prevStderr
	out, _ := io.ReadAll(r)
	got := string(out)

	if !strings.Contains(got, "/usr/local/bin/chg") {
		t.Errorf("dry-run output missing bare hg path; got %q", got)
	}
	if strings.Contains(got, "--hg-path=") {
		t.Errorf("dry-run output still contains --hg-path= prefix; got %q", got)
	}
}
