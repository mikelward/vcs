package promptinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikelward/vcs/vcsdetect"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   []string
	}{
		{"default format", DefaultFormat, []string{"branch", "fetch_stale", "project", "status", "subdir"}},
		{"single field", "{branch}", []string{"branch"}},
		{"no fields", "hello world", nil},
		{"duplicate fields", "{branch} {branch}", []string{"branch"}},
		{"custom", "{project}: {status}", []string{"project", "status"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFields(tt.format)
			var keys []string
			for k := range got {
				keys = append(keys, k)
			}
			if len(keys) == 0 {
				keys = nil
			}
			sortStrings(keys)
			sortStrings(tt.want)
			if !stringSliceEqual(keys, tt.want) {
				t.Errorf("ParseFields(%q) = %v, want %v", tt.format, keys, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		format string
		color  bool
		want   string
	}{
		{
			name:   "basic no color",
			result: &Result{Project: "myrepo", Branch: "main", Status: "*"},
			format: `{project} {branch} {status}`,
			color:  false,
			want:   "myrepo main *",
		},
		{
			name:   "empty fields collapse spaces",
			result: &Result{Project: "myrepo", Branch: "", Status: ""},
			format: `{project} {branch} {status}`,
			color:  false,
			want:   "myrepo",
		},
		{
			name:   "with color",
			result: &Result{Project: "myrepo", Branch: "main", Status: "*"},
			format: `{project} {branch} {status}`,
			color:  true,
			want:   "\033[32mmyrepo\033[0m main \033[33m*\033[0m",
		},
		{
			name:   "empty field no color codes",
			result: &Result{Project: "myrepo", Branch: "", Status: ""},
			format: `{project} {branch} {status}`,
			color:  true,
			want:   "\033[32mmyrepo\033[0m",
		},
		{
			name:   "fetch_stale true",
			result: &Result{Project: "myrepo", FetchStale: true},
			format: `{project} {fetch_stale}`,
			color:  false,
			want:   "myrepo fetch",
		},
		{
			name:   "fetch_stale false",
			result: &Result{Project: "myrepo", FetchStale: false},
			format: `{project} {fetch_stale}`,
			color:  false,
			want:   "myrepo",
		},
		{
			name:   "fetch_stale with color",
			result: &Result{Project: "myrepo", FetchStale: true},
			format: `{project} {fetch_stale}`,
			color:  true,
			want:   "\033[32mmyrepo\033[0m \033[33mfetch\033[0m",
		},
		{
			name:   "subdir with color",
			result: &Result{Project: "myrepo", Subdir: "src/pkg"},
			format: `{project} {subdir}`,
			color:  true,
			want:   "\033[32mmyrepo\033[0m \033[34msrc/pkg\033[0m",
		},
		{
			name:   "tab in format",
			result: &Result{Project: "myrepo", Branch: "main"},
			format: `{project}\t{branch}`,
			color:  false,
			want:   "myrepo\tmain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Format(tt.result, tt.format, tt.color)
			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorWrap(t *testing.T) {
	tests := []struct {
		text, code, want string
	}{
		{"hello", "\033[32m", "\033[32mhello\033[0m"},
		{"", "\033[32m", ""},
	}
	for _, tt := range tests {
		got := colorWrap(tt.text, tt.code)
		if got != tt.want {
			t.Errorf("colorWrap(%q, %q) = %q, want %q", tt.text, tt.code, got, tt.want)
		}
	}
}

func TestFetchAge(t *testing.T) {
	t.Run("nonexistent", func(t *testing.T) {
		if getFetchStale("/nonexistent/path/FETCH_HEAD") {
			t.Error("getFetchStale on nonexistent file should return false")
		}
	})

	t.Run("recent", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "FETCH_HEAD")
		os.WriteFile(path, []byte("test"), 0644)
		if getFetchStale(path) {
			t.Error("getFetchStale on recent file should return false")
		}
	})

	t.Run("stale", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "FETCH_HEAD")
		os.WriteFile(path, []byte("test"), 0644)
		old := time.Now().Add(-25 * time.Hour)
		os.Chtimes(path, old, old)
		if !getFetchStale(path) {
			t.Error("getFetchStale on stale file should return true")
		}
	})
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

func TestGatherGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)
	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := ParseFields(DefaultFormat)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Project != filepath.Base(tmp) {
		t.Errorf("Project = %q, want %q", result.Project, filepath.Base(tmp))
	}
	if result.Subdir != "" {
		t.Errorf("Subdir = %q, want empty", result.Subdir)
	}
	if result.Branch != "main" {
		t.Errorf("Branch = %q, want %q", result.Branch, "main")
	}
}

func TestGatherGitSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)
	subdir := filepath.Join(tmp, "src", "pkg")
	os.MkdirAll(subdir, 0755)

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"subdir": true}

	origDir, _ := os.Getwd()
	os.Chdir(subdir)
	defer os.Chdir(origDir)

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	want := filepath.Join("src", "pkg")
	if result.Subdir != want {
		t.Errorf("Subdir = %q, want %q", result.Subdir, want)
	}
}

func TestGatherGitUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)
	os.WriteFile(filepath.Join(tmp, "newfile.txt"), []byte("hello"), 0644)

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"status": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Status != "*" {
		t.Errorf("Status = %q, want %q", result.Status, "*")
	}
}

func TestGatherGitStaleFetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)

	fetchPath := filepath.Join(tmp, ".git", "FETCH_HEAD")
	os.WriteFile(fetchPath, []byte("test"), 0644)
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(fetchPath, old, old)

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"fetch_stale": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if !result.FetchStale {
		t.Error("FetchStale should be true for stale FETCH_HEAD")
	}
}

func TestGatherFieldSelection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)
	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"branch": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Branch != "main" {
		t.Errorf("Branch = %q, want %q", result.Branch, "main")
	}
	if result.Project != "" {
		t.Errorf("Project = %q, want empty (not requested)", result.Project)
	}
	if result.Status != "" {
		t.Errorf("Status = %q, want empty (not requested)", result.Status)
	}
}

func TestGatherJJRepo(t *testing.T) {
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not found")
	}

	tmp := t.TempDir()
	cmd := exec.Command("jj", "git", "init", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj git init failed: %v\n%s", err, out)
	}

	info := &vcsdetect.Info{VCS: "jj", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"branch": true}

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Branch != "" {
		t.Errorf("Branch = %q, want empty for jj", result.Branch)
	}
}

func TestGatherHGRepo(t *testing.T) {
	hgCmd := "hg"
	if p, err := exec.LookPath("chg"); err == nil {
		hgCmd = p
	} else if _, err := exec.LookPath("hg"); err != nil {
		t.Skip("hg not found")
	}

	tmp := t.TempDir()
	cmd := exec.Command(hgCmd, "init", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hg init failed: %v\n%s", err, out)
	}

	info := &vcsdetect.Info{VCS: "hg", RootDir: tmp}
	fields := map[string]bool{"branch": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Branch != "default" {
		t.Errorf("Branch = %q, want %q for hg", result.Branch, "default")
	}
}

// helpers

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
