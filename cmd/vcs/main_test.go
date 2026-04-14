package main

import (
	"os"
	"path/filepath"
	"testing"
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
