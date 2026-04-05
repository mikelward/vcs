// Package vcsdetect detects the VCS, backend, hosting, and root directory
// for a given working directory. It reads and writes .vcs_cache files
// compatible with the shell implementation.
package vcsdetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Info holds the detected VCS information for a directory.
type Info struct {
	VCS     string // "git", "hg", "jj", "g4"
	Backend string // "git", "piper", etc.
	Hosting string // "github", "gitlab", "bitbucket", "sourcehut", "gerrit"
	RootDir string // absolute path to the repo root
}

// CachePath returns the path to the .vcs_cache file for dir.
func CachePath(dir string) string {
	return filepath.Join(dir, ".vcs_cache")
}

// ReadCache reads a .vcs_cache file compatible with the bash format:
//
//	line 1: <vcs> <backend> <hosting>  (- for empty fields)
//	line 2: <rootdir>
func ReadCache(path string) (*Info, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitN(string(data), "\n", 3)
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid cache format")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid cache line 1: %q", lines[0])
	}
	info := &Info{
		VCS:     fields[0],
		Backend: fields[1],
		Hosting: fields[2],
		RootDir: strings.TrimRight(lines[1], "\n"),
	}
	if info.Backend == "-" {
		info.Backend = ""
	}
	if info.Hosting == "-" {
		info.Hosting = ""
	}
	return info, nil
}

// WriteCache writes a .vcs_cache file. Errors are silently ignored
// (the filesystem may be read-only).
func WriteCache(dir string, info *Info) {
	backend := info.Backend
	if backend == "" {
		backend = "-"
	}
	hosting := info.Hosting
	if hosting == "" {
		hosting = "-"
	}
	content := fmt.Sprintf("%s %s %s\n%s\n", info.VCS, backend, hosting, info.RootDir)
	_ = os.WriteFile(CachePath(dir), []byte(content), 0666)
}

// Detect finds the VCS for the given directory, checking the cache first.
// If no cache exists, it walks up the directory tree looking for VCS markers.
func Detect(dir string) (*Info, error) {
	// Try cache first.
	if info, err := ReadCache(CachePath(dir)); err == nil {
		return info, nil
	}

	// Walk up looking for VCS markers.
	// Order matters: jj > hg > git > g4 (same as bash).
	d := dir
	var vcsName, rootDir string
	for {
		for _, marker := range []struct {
			dir string
			vcs string
		}{
			{".jj", "jj"},
			{".hg", "hg"},
			{".git", "git"},
			{".citc", "g4"},
			{".p4config", "g4"},
		} {
			if _, err := os.Stat(filepath.Join(d, marker.dir)); err == nil {
				vcsName = marker.vcs
				rootDir = d
				break
			}
		}
		if vcsName != "" {
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}

	if vcsName == "" {
		return nil, fmt.Errorf("no VCS found in %s", dir)
	}

	info := &Info{VCS: vcsName, RootDir: rootDir}

	// Detect backend.
	switch vcsName {
	case "jj":
		storePath := filepath.Join(rootDir, ".jj", "repo", "store", "type")
		if data, err := os.ReadFile(storePath); err == nil {
			info.Backend = strings.TrimSpace(string(data))
		}
	case "git":
		info.Backend = "git"
	}

	// Detect hosting from git origin URL.
	if info.Backend == "git" {
		info.Hosting = detectHosting(vcsName, rootDir)
	}

	// Write cache (best-effort).
	WriteCache(dir, info)
	return info, nil
}

// detectHosting reads the git origin URL and returns the hosting platform.
func detectHosting(vcsName, rootDir string) string {
	var configPath string
	switch vcsName {
	case "jj":
		configPath = filepath.Join(rootDir, ".jj", "repo", "store", "git", "config")
	default:
		configPath = filepath.Join(rootDir, ".git", "config")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Simple git config parser: find [remote "origin"] section and extract url.
	lines := strings.Split(string(data), "\n")
	inOrigin := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = false
			continue
		}
		if inOrigin && strings.HasPrefix(trimmed, "url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				url := strings.TrimSpace(parts[1])
				return classifyURL(url)
			}
		}
	}
	return ""
}

func classifyURL(url string) string {
	switch {
	case strings.Contains(url, "github.com"):
		return "github"
	case strings.Contains(url, "gitlab.com") || strings.Contains(url, "gitlab."):
		return "gitlab"
	case strings.Contains(url, "bitbucket.org"):
		return "bitbucket"
	case strings.Contains(url, "sr.ht"):
		return "sourcehut"
	case strings.Contains(url, "googlesource.com"):
		return "gerrit"
	}
	return ""
}
