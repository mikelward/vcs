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
