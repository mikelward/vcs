package promptline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
