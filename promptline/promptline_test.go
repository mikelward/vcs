package promptline

import (
	"encoding/binary"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikelward/vcs/vcsdetect"
)

func TestHostPart(t *testing.T) {
	tests := []struct {
		name       string
		hostname   string
		shpool     string
		production bool
		color      bool
		want       string
	}{
		{
			name:     "plain host, in shpool, no color",
			hostname: "myhost",
			shpool:   "work",
			want:     "myhost [work]",
		},
		{
			name:     "plain host, no shpool, no color",
			hostname: "myhost",
			want:     "myhost shpool",
		},
		{
			name:       "production, in shpool, no color",
			hostname:   "prod1",
			shpool:     "sess",
			production: true,
			want:       "prod1 [sess]",
		},
		{
			name:     "in shpool, with color",
			hostname: "myhost",
			shpool:   "work",
			color:    true,
			want:     "myhost [\033[32mwork\033[0m]",
		},
		{
			name:     "not in shpool, with color",
			hostname: "myhost",
			color:    true,
			want:     "myhost \033[33mshpool\033[0m",
		},
		{
			name:       "production, in shpool, with color",
			hostname:   "prod1",
			shpool:     "sess",
			production: true,
			color:      true,
			want:       "\033[31mprod1\033[0m [\033[32msess\033[0m]",
		},
		{
			name:       "production, no shpool, with color",
			hostname:   "prod1",
			production: true,
			color:      true,
			want:       "\033[31mprod1\033[0m \033[33mshpool\033[0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HostPart(tt.hostname, tt.shpool, tt.production, tt.color)
			if got != tt.want {
				t.Errorf("HostPart(%q, %q, %v, %v) = %q, want %q",
					tt.hostname, tt.shpool, tt.production, tt.color, got, tt.want)
			}
		})
	}
}

func TestTildeDirectory(t *testing.T) {
	tests := []struct {
		name, cwd, home, want string
	}{
		{"home itself", "/home/user", "/home/user", "~"},
		{"under home", "/home/user/proj", "/home/user", "~/proj"},
		{"deep under home", "/home/user/a/b/c", "/home/user", "~/a/b/c"},
		{"outside home", "/etc", "/home/user", "/etc"},
		{"home prefix substring", "/home/user2", "/home/user", "/home/user2"},
		{"empty home", "/home/user", "", "/home/user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TildeDirectory(tt.cwd, tt.home)
			if got != tt.want {
				t.Errorf("TildeDirectory(%q, %q) = %q, want %q",
					tt.cwd, tt.home, got, tt.want)
			}
		})
	}
}

func TestAuthWarning(t *testing.T) {
	if got := AuthWarning("SSH", false); got != "SSH" {
		t.Errorf("AuthWarning(SSH, false) = %q, want %q", got, "SSH")
	}
	want := "\033[33mSSH\033[0m"
	if got := AuthWarning("SSH", true); got != want {
		t.Errorf("AuthWarning(SSH, true) = %q, want %q", got, want)
	}
}

func TestBuildNoVCSFallback(t *testing.T) {
	tmp := t.TempDir()

	t.Run("no color", func(t *testing.T) {
		got := Build(&Options{
			Hostname: "host",
			Shpool:   "sess",
			NoVCS:    true,
			Cwd:      tmp,
			HomeDir:  "/nonexistent",
			SkipAuth: true,
			AuthOK:   true,
		})
		want := "host [sess] " + tmp + " vcs"
		if got != want {
			t.Errorf("Build() = %q, want %q", got, want)
		}
	})

	t.Run("color", func(t *testing.T) {
		got := Build(&Options{
			Hostname: "host",
			Shpool:   "sess",
			NoVCS:    true,
			Cwd:      tmp,
			HomeDir:  "/nonexistent",
			Color:    true,
			SkipAuth: true,
			AuthOK:   true,
		})
		want := "host [\033[32msess\033[0m] \033[34m" + tmp + " \033[33mvcs\033[0m\033[0m"
		if got != want {
			t.Errorf("Build() = %q, want %q", got, want)
		}
	})
}

func TestBuildOutsideRepo(t *testing.T) {
	tmp := t.TempDir()
	// Guard against test env where tmp is somehow inside a repo.
	if info, err := vcsdetect.Detect(tmp); err == nil && info != nil {
		t.Skipf("tmp %q is inside a %s repo rooted at %s", tmp, info.VCS, info.RootDir)
	}

	got := Build(&Options{
		Hostname: "host",
		Shpool:   "sess",
		Cwd:      tmp,
		HomeDir:  "/nonexistent",
		SkipAuth: true,
		AuthOK:   true,
	})
	want := "host [sess] " + tmp
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuildInsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)

	// Chdir so promptinfo.Gather's os.Getwd() matches the repo root,
	// yielding an empty Subdir.
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	t.Run("no color", func(t *testing.T) {
		got := Build(&Options{
			Hostname: "host",
			Shpool:   "sess",
			HomeDir:  "/nonexistent",
			SkipAuth: true,
			AuthOK:   true,
		})
		if !strings.HasPrefix(got, "host [sess] ") {
			t.Errorf("Build() = %q, want prefix %q", got, "host [sess] ")
		}
		project := filepath.Base(tmp)
		if !strings.Contains(got, project) {
			t.Errorf("Build() = %q, want to contain project %q", got, project)
		}
		if !strings.Contains(got, "main") {
			t.Errorf("Build() = %q, want to contain branch %q", got, "main")
		}
	})

	t.Run("color", func(t *testing.T) {
		got := Build(&Options{
			Hostname: "host",
			Shpool:   "sess",
			HomeDir:  "/nonexistent",
			Color:    true,
			SkipAuth: true,
			AuthOK:   true,
		})
		if !strings.Contains(got, "\033[34m") {
			t.Errorf("Build() = %q, want blue wrap \\033[34m", got)
		}
		if !strings.Contains(got, "\033[32m") {
			t.Errorf("Build() = %q, want green project \\033[32m", got)
		}
	})
}

func TestBuildAuthWarning(t *testing.T) {
	tmp := t.TempDir()
	got := Build(&Options{
		Hostname: "host",
		Shpool:   "sess",
		NoVCS:    true,
		Cwd:      tmp,
		HomeDir:  "/nonexistent",
		SkipAuth: true,
		AuthOK:   false,
	})
	if !strings.HasSuffix(got, " SSH") {
		t.Errorf("Build() = %q, want suffix %q", got, " SSH")
	}
}

func TestSSHAgentHasKeys(t *testing.T) {
	t.Run("empty sock", func(t *testing.T) {
		if sshAgentHasKeys("") {
			t.Error("empty SSH_AUTH_SOCK should report no keys")
		}
	})

	t.Run("nonexistent sock", func(t *testing.T) {
		if sshAgentHasKeys(filepath.Join(t.TempDir(), "missing.sock")) {
			t.Error("nonexistent socket should report no keys")
		}
	})

	t.Run("agent with one key", func(t *testing.T) {
		path := startFakeAgent(t, 1)
		if !sshAgentHasKeys(path) {
			t.Error("agent with 1 key should report keys present")
		}
	})

	t.Run("agent with no keys", func(t *testing.T) {
		path := startFakeAgent(t, 0)
		if sshAgentHasKeys(path) {
			t.Error("agent with 0 keys should report no keys")
		}
	})

	t.Run("agent sends wrong response type", func(t *testing.T) {
		path := startFakeAgentRaw(t, func(c net.Conn) {
			// Read (and discard) the 5-byte request.
			buf := make([]byte, 5)
			_, _ = c.Read(buf)
			// Respond with length=5, type=30 (garbage), nkeys=7.
			resp := []byte{0, 0, 0, 5, 30, 0, 0, 0, 7}
			_, _ = c.Write(resp)
		})
		if sshAgentHasKeys(path) {
			t.Error("wrong response type should report no keys")
		}
	})
}

// startFakeAgent stands up a unix socket that replies to one
// REQUEST_IDENTITIES with an IDENTITIES_ANSWER advertising nkeys keys
// (no actual key blobs — sshAgentHasKeys only reads the count).
func startFakeAgent(t *testing.T, nkeys uint32) string {
	t.Helper()
	return startFakeAgentRaw(t, func(c net.Conn) {
		buf := make([]byte, 5)
		if _, err := c.Read(buf); err != nil {
			return
		}
		// length = type(1) + nkeys(4) = 5
		resp := make([]byte, 9)
		binary.BigEndian.PutUint32(resp[0:4], 5)
		resp[4] = 12 // SSH2_AGENT_IDENTITIES_ANSWER
		binary.BigEndian.PutUint32(resp[5:9], nkeys)
		_, _ = c.Write(resp)
	})
}

func startFakeAgentRaw(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				handle(c)
			}(c)
		}
	}()
	return path
}

func TestAuthCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth_cache")

	// Missing cache: no hit.
	if w, hit := readAuthCache(path, time.Minute); hit || w != "" {
		t.Errorf("missing cache should miss; got warning=%q hit=%v", w, hit)
	}

	// Write empty (auth OK), read back.
	writeAuthCache(path, "")
	if w, hit := readAuthCache(path, time.Minute); !hit || w != "" {
		t.Errorf(`after writeAuthCache(""): got warning=%q hit=%v, want "" true`, w, hit)
	}
	// Empty file on disk matches shell convention.
	if data, err := os.ReadFile(path); err != nil {
		t.Fatalf("read: %v", err)
	} else if len(data) != 0 {
		t.Errorf("ok cache file should be empty; got %q", data)
	}

	// Overwrite with a warning.
	writeAuthCache(path, "SSH")
	if w, hit := readAuthCache(path, time.Minute); !hit || w != "SSH" {
		t.Errorf(`after writeAuthCache("SSH"): got warning=%q hit=%v, want "SSH" true`, w, hit)
	}
}

func TestAuthCacheStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth_cache")
	writeAuthCache(path, "")

	// Backdate mtime by an hour.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if _, hit := readAuthCache(path, time.Minute); hit {
		t.Error("stale cache should miss")
	}
}

// TestAuthCacheShellFormat verifies the cache file can be authored by the
// shell (empty == ok, any content == warning text) and the Go reader will
// honour it. Also covers the warning-without-newline case the shell may emit.
func TestAuthCacheShellFormat(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"shell empty file (ok)", "", ""},
		{"shell SSH with newline", "SSH\n", "SSH"},
		{"shell SSH no newline", "SSH", "SSH"},
		{"arbitrary warning text", "agent\n", "agent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "auth_cache")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, hit := readAuthCache(path, time.Minute)
			if !hit {
				t.Fatalf("expected cache hit for %q", tt.content)
			}
			if got != tt.want {
				t.Errorf("readAuthCache(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestAuthCacheConfigDefaults(t *testing.T) {
	home := t.TempDir()
	path, ttl := authCacheConfig(&Options{HomeDir: home})
	if want := filepath.Join(home, DefaultAuthCacheName); path != want {
		t.Errorf("default path = %q, want %q", path, want)
	}
	if ttl != DefaultAuthCacheTTL {
		t.Errorf("default ttl = %v, want %v", ttl, DefaultAuthCacheTTL)
	}
}

func TestAuthCacheConfigDisabled(t *testing.T) {
	if p, _ := authCacheConfig(&Options{AuthCachePath: "-", HomeDir: "/x"}); p != "" {
		t.Errorf(`AuthCachePath="-" should disable, got %q`, p)
	}
	if p, _ := authCacheConfig(&Options{AuthCacheTTL: -1, HomeDir: "/x"}); p != "" {
		t.Errorf("negative TTL should disable, got %q", p)
	}
}

// TestBuildAuthCacheHit verifies Build consults the cache without dialing
// the agent. We point SSH_AUTH_SOCK at a nonexistent path and seed the cache
// with "ok"; a cache hit means no warning even though no real agent exists.
func TestBuildAuthCacheHit(t *testing.T) {
	home := t.TempDir()
	cachePath := filepath.Join(home, DefaultAuthCacheName)
	writeAuthCache(cachePath, "")

	// Ensure any real agent dial would fail.
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "nope.sock"))

	tmp := t.TempDir()
	got := Build(&Options{
		Hostname: "host",
		Shpool:   "sess",
		NoVCS:    true,
		Cwd:      tmp,
		HomeDir:  home,
	})
	if strings.Contains(got, "SSH") {
		t.Errorf("cached ok should suppress warning; got %q", got)
	}
}

func TestBuildAuthCacheMissAgentDown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "nope.sock"))

	tmp := t.TempDir()
	got := Build(&Options{
		Hostname: "host",
		Shpool:   "sess",
		NoVCS:    true,
		Cwd:      tmp,
		HomeDir:  home,
	})
	if !strings.HasSuffix(got, " SSH") {
		t.Errorf("no cache + no agent should emit SSH warning; got %q", got)
	}
	// The miss should have populated the cache with the warning text.
	if w, hit := readAuthCache(filepath.Join(home, DefaultAuthCacheName), time.Minute); !hit || w != "SSH" {
		t.Errorf(`expected cache populated with "SSH"; got warning=%q hit=%v`, w, hit)
	}
}

// initGitRepo creates a temp git repo with signing disabled and one commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	runGit(t, tmp, "init", "-b", "main")
	runGit(t, tmp, "config", "commit.gpgsign", "false")
	runGit(t, tmp, "commit", "--allow-empty", "--no-verify", "-m", "initial commit")
	return tmp
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
