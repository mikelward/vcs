package vcsdetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCache(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".vcs_cache")

	// Valid cache.
	os.WriteFile(cache, []byte("git git github\n/home/user/repo\n"), 0666)
	info, err := ReadCache(cache)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "git" || info.Backend != "git" || info.Hosting != "github" || info.RootDir != "/home/user/repo" {
		t.Errorf("unexpected info: %+v", info)
	}

	// Sentinel dashes.
	os.WriteFile(cache, []byte("hg - -\n/home/user/hgrepo\n"), 0666)
	info, err = ReadCache(cache)
	if err != nil {
		t.Fatal(err)
	}
	if info.Backend != "" || info.Hosting != "" {
		t.Errorf("expected empty backend/hosting, got %q/%q", info.Backend, info.Hosting)
	}
}

func TestWriteAndReadCache(t *testing.T) {
	dir := t.TempDir()
	info := &Info{VCS: "jj", Backend: "git", Hosting: "github", RootDir: "/tmp/repo"}
	WriteCache(dir, info)

	got, err := ReadCache(CachePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if *got != *info {
		t.Errorf("got %+v, want %+v", got, info)
	}
}

func TestWriteCacheEmptyFields(t *testing.T) {
	dir := t.TempDir()
	info := &Info{VCS: "hg", RootDir: "/tmp/hgrepo"}
	WriteCache(dir, info)

	data, _ := os.ReadFile(CachePath(dir))
	if string(data) != "hg - -\n/tmp/hgrepo\n" {
		t.Errorf("unexpected cache content: %q", string(data))
	}
}

func TestDetectGit(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0755)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "git" {
		t.Errorf("expected git, got %s", info.VCS)
	}
	if info.Backend != "git" {
		t.Errorf("expected backend git, got %s", info.Backend)
	}
	if info.RootDir != dir {
		t.Errorf("expected rootdir %s, got %s", dir, info.RootDir)
	}
}

func TestDetectHg(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".hg"), 0755)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "hg" {
		t.Errorf("expected hg, got %s", info.VCS)
	}
}

func TestDetectJJ(t *testing.T) {
	dir := t.TempDir()
	jjPath := filepath.Join(dir, ".jj", "repo", "store")
	os.MkdirAll(jjPath, 0755)
	os.WriteFile(filepath.Join(jjPath, "type"), []byte("git\n"), 0666)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "jj" {
		t.Errorf("expected jj, got %s", info.VCS)
	}
	if info.Backend != "git" {
		t.Errorf("expected backend git, got %s", info.Backend)
	}
}

func TestDetectSubdir(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	sub := filepath.Join(dir, "src", "pkg")
	os.MkdirAll(sub, 0755)

	info, err := Detect(sub)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "git" {
		t.Errorf("expected git, got %s", info.VCS)
	}
	if info.RootDir != dir {
		t.Errorf("expected rootdir %s, got %s", dir, info.RootDir)
	}
}

func TestDetectPriority(t *testing.T) {
	// jj takes priority over git.
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	os.MkdirAll(filepath.Join(dir, ".jj", "repo", "store"), 0755)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "jj" {
		t.Errorf("expected jj (higher priority), got %s", info.VCS)
	}
}

func TestDetectUsesCache(t *testing.T) {
	dir := t.TempDir()
	// Write cache pointing to a fake VCS.
	os.WriteFile(filepath.Join(dir, ".vcs_cache"), []byte("hg - -\n/fake/root\n"), 0666)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.VCS != "hg" || info.RootDir != "/fake/root" {
		t.Errorf("expected cached hg at /fake/root, got %+v", info)
	}
}

func TestDetectNoVCS(t *testing.T) {
	dir := t.TempDir()
	_, err := Detect(dir)
	if err == nil {
		t.Error("expected error for dir with no VCS")
	}
}

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:user/repo.git", "github"},
		{"https://github.com/user/repo", "github"},
		{"https://gitlab.com/user/repo", "gitlab"},
		{"https://gitlab.example.com/repo", "gitlab"},
		{"https://bitbucket.org/user/repo", "bitbucket"},
		{"git@git.sr.ht:~user/repo", "sourcehut"},
		{"https://chromium.googlesource.com/foo", "gerrit"},
		{"https://example.com/repo", ""},
	}
	for _, tt := range tests {
		got := classifyURL(tt.url)
		if got != tt.want {
			t.Errorf("classifyURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDetectHostingFromConfig(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.Mkdir(gitDir, 0755)
	config := `[core]
	bare = false
[remote "origin"]
	url = git@github.com:user/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0666)

	info, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Hosting != "github" {
		t.Errorf("expected github hosting, got %q", info.Hosting)
	}
}
