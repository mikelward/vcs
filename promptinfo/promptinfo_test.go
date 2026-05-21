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
		{"default format", DefaultFormat, []string{"behind", "branch", "project", "status", "subdir"}},
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
			name:   "behind true",
			result: &Result{Project: "myrepo", Behind: true},
			format: `{project} {behind}`,
			color:  false,
			want:   "myrepo pull",
		},
		{
			name:   "behind false",
			result: &Result{Project: "myrepo", Behind: false},
			format: `{project} {behind}`,
			color:  false,
			want:   "myrepo",
		},
		{
			name:   "behind with color",
			result: &Result{Project: "myrepo", Behind: true},
			format: `{project} {behind}`,
			color:  true,
			want:   "\033[32mmyrepo\033[0m \033[33mpull\033[0m",
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

func testHgPath(t *testing.T) string {
	t.Helper()
	var p string
	if q, err := exec.LookPath("chg"); err == nil {
		p = q
	} else if q, err := exec.LookPath("hg"); err == nil {
		p = q
	} else {
		t.Skip("hg not found")
		return ""
	}
	if out, err := exec.Command(p, "--version").CombinedOutput(); err != nil {
		t.Skipf("hg not functional: %v\n%s", err, out)
	}
	return p
}

func runHg(t *testing.T, hgPath, dir string, args ...string) {
	t.Helper()
	if dir != "" {
		args = append([]string{"--cwd", dir}, args...)
	}
	cmd := exec.Command(hgPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hg %v failed: %v\n%s", args, err, out)
	}
}

func initHgRepoWithRemote(t *testing.T, hgPath string) (remote, local string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote")
	local = filepath.Join(root, "local")

	runHg(t, hgPath, "", "init", remote)
	os.WriteFile(filepath.Join(remote, "file.txt"), []byte("initial\n"), 0644)
	runHg(t, hgPath, remote, "add", "file.txt")
	runHg(t, hgPath, remote, "commit", "-m", "initial commit", "-u", "Test <test@example.com>")
	runHg(t, hgPath, "", "clone", "-q", remote, local)
	return remote, local
}

func testJJ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not found")
	}
}

func runJJ(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj %v failed: %v\n%s", args, err, out)
	}
}

func initJJClone(t *testing.T) (upstream, local string) {
	t.Helper()
	testJJ(t)

	root := t.TempDir()
	upstream = filepath.Join(root, "upstream")
	os.MkdirAll(upstream, 0755)
	runGit(t, upstream, "init", "-b", "main")
	runGit(t, upstream, "config", "commit.gpgsign", "false")
	runGit(t, upstream, "commit", "--allow-empty", "--no-verify", "-m", "first")

	local = filepath.Join(root, "local")
	cmd := exec.Command("jj", "git", "clone", upstream, local)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj git clone failed: %v\n%s", err, out)
	}
	return upstream, local
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

// TestGatherGitBehindMtimeFallback covers the no-upstream case: a git repo
// with no configured tracking branch falls back to FETCH_HEAD mtime, so a
// stale marker still flips Behind.
func TestGatherGitBehindMtimeFallback(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmp := initGitRepo(t)

	fetchPath := filepath.Join(tmp, ".git", "FETCH_HEAD")
	os.WriteFile(fetchPath, []byte("test"), 0644)
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(fetchPath, old, old)

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: tmp}
	fields := map[string]bool{"behind": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if !result.Behind {
		t.Error("Behind should be true for stale FETCH_HEAD with no upstream")
	}
}

// TestGatherGitBehindUpstream covers the in-upstream-and-behind case: set up
// a remote/upstream where the tracking branch is ahead of HEAD, and confirm
// status --branch --porcelain detects it as behind.
func TestGatherGitBehindUpstream(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	// Build an "upstream" repo with two commits, then a "local" clone that
	// rewinds to the first commit so it's behind by one.
	upstream := t.TempDir()
	runGit(t, upstream, "init", "-b", "main")
	runGit(t, upstream, "config", "commit.gpgsign", "false")
	runGit(t, upstream, "commit", "--allow-empty", "--no-verify", "-m", "first")
	runGit(t, upstream, "commit", "--allow-empty", "--no-verify", "-m", "second")

	local := t.TempDir()
	// Re-init in `local` rather than `git clone <upstream> <local>` so
	// `clone` doesn't reject a non-empty TempDir on some platforms.
	runGit(t, local, "init", "-b", "main")
	runGit(t, local, "config", "commit.gpgsign", "false")
	runGit(t, local, "remote", "add", "origin", upstream)
	runGit(t, local, "fetch", "origin")
	runGit(t, local, "reset", "--hard", "origin/main~")
	runGit(t, local, "branch", "--set-upstream-to=origin/main", "main")

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: local}
	fields := map[string]bool{"behind": true, "status": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if !result.Behind {
		t.Error("Behind should be true when upstream is ahead of HEAD")
	}
	if result.Status != "" {
		t.Errorf("Status = %q, want empty for clean tree", result.Status)
	}
}

// TestGatherGitNotBehind covers the in-upstream-and-up-to-date case: same
// setup as above without the rewind. Should not be flagged behind, and the
// stale-mtime fallback should NOT kick in even when FETCH_HEAD is old (we
// have an upstream and it agrees with HEAD).
func TestGatherGitNotBehind(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	upstream := t.TempDir()
	runGit(t, upstream, "init", "-b", "main")
	runGit(t, upstream, "config", "commit.gpgsign", "false")
	runGit(t, upstream, "commit", "--allow-empty", "--no-verify", "-m", "only")

	local := t.TempDir()
	runGit(t, local, "init", "-b", "main")
	runGit(t, local, "config", "commit.gpgsign", "false")
	runGit(t, local, "remote", "add", "origin", upstream)
	runGit(t, local, "fetch", "origin")
	runGit(t, local, "reset", "--hard", "origin/main")
	runGit(t, local, "branch", "--set-upstream-to=origin/main", "main")

	// Age FETCH_HEAD past the stale threshold to prove the upstream
	// signal takes priority over the mtime fallback.
	fetchPath := filepath.Join(local, ".git", "FETCH_HEAD")
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(fetchPath, old, old)

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: local}
	fields := map[string]bool{"behind": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Behind {
		t.Error("Behind should be false when local is up to date with upstream, even with stale FETCH_HEAD")
	}
}

func TestGetBranchGitWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	main := initGitRepo(t) // on branch "main"
	runGit(t, main, "branch", "feature")

	wtDir := filepath.Join(t.TempDir(), "wt")
	runGit(t, main, "worktree", "add", wtDir, "feature")

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: wtDir}
	got := getBranch(info)
	if got != "feature" {
		t.Errorf("getBranch() in worktree = %q, want %q", got, "feature")
	}
}

func TestFetchHeadPathGitWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	main := initGitRepo(t)
	runGit(t, main, "branch", "feature")
	wtDir := filepath.Join(t.TempDir(), "wt")
	runGit(t, main, "worktree", "add", wtDir, "feature")

	info := &vcsdetect.Info{VCS: "git", Backend: "git", RootDir: wtDir}
	got := fetchHeadPath(info)

	// In a worktree, .git is a file, so the naive joined path is invalid.
	naive := filepath.Join(wtDir, ".git", "FETCH_HEAD")
	if got == naive {
		t.Errorf("fetchHeadPath() in worktree = naive path %q; should be resolved via git", got)
	}
	if filepath.Base(got) != "FETCH_HEAD" {
		t.Errorf("fetchHeadPath() = %q, base should be FETCH_HEAD", got)
	}
	// The containing directory should exist (a real git infrastructure dir).
	if _, err := os.Stat(filepath.Dir(got)); err != nil {
		t.Errorf("fetchHeadPath() = %q, directory does not exist: %v", got, err)
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

func TestGatherJJBehindUpstream(t *testing.T) {
	upstream, local := initJJClone(t)
	runGit(t, upstream, "commit", "--allow-empty", "--no-verify", "-m", "second")
	runJJ(t, local, "git", "fetch")

	info := &vcsdetect.Info{VCS: "jj", Backend: "git", RootDir: local}
	fields := map[string]bool{"behind": true, "status": true}

	result, err := Gather(info, fields, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if !result.Behind {
		t.Error("Behind should be true when a fetched remote bookmark is ahead of @")
	}
	if result.Status != "" {
		t.Errorf("Status = %q, want empty for clean tree", result.Status)
	}
}

func TestGatherJJNotBehind(t *testing.T) {
	_, local := initJJClone(t)

	info := &vcsdetect.Info{VCS: "jj", Backend: "git", RootDir: local}
	fetchPath := fetchHeadPath(info)
	if err := os.MkdirAll(filepath.Dir(fetchPath), 0755); err != nil {
		t.Fatalf("create jj fetch marker dir: %v", err)
	}
	if err := os.WriteFile(fetchPath, []byte("test"), 0644); err != nil {
		t.Fatalf("write jj FETCH_HEAD: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(fetchPath, old, old); err != nil {
		t.Fatalf("age jj FETCH_HEAD: %v", err)
	}

	result, err := Gather(info, map[string]bool{"behind": true}, nil)
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Behind {
		t.Error("Behind should be false when @ contains all remote bookmarks, even with stale FETCH_HEAD")
	}
}

func TestGatherHGRepo(t *testing.T) {
	hgCmd := testHgPath(t)

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

func TestGatherHGBehindUpstream(t *testing.T) {
	hgPath := testHgPath(t)
	remote, local := initHgRepoWithRemote(t, hgPath)
	os.WriteFile(filepath.Join(remote, "remote.txt"), []byte("remote\n"), 0644)
	runHg(t, hgPath, remote, "add", "remote.txt")
	runHg(t, hgPath, remote, "commit", "-m", "remote commit", "-u", "Test <test@example.com>")
	runHg(t, hgPath, local, "pull", "-q")

	info := &vcsdetect.Info{VCS: "hg", RootDir: local}
	fields := map[string]bool{"behind": true, "status": true}

	result, err := Gather(info, fields, &Options{HgPath: hgPath})
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if !result.Behind {
		t.Error("Behind should be true when hg summary reports an available update")
	}
	if result.Status != "" {
		t.Errorf("Status = %q, want empty for clean tree", result.Status)
	}
}

func TestParseHgSummaryCleanSecretPhase(t *testing.T) {
	out := "parent: 0:0123456789ab tip\ncommit: (clean) (secret)\nupdate: (current)\n"

	status, behind, hasUpstream := parseHgSummary(out, true)

	if status != "" {
		t.Errorf("Status = %q, want empty for clean secret-phase summary", status)
	}
	if behind {
		t.Error("Behind should be false when hg summary reports current")
	}
	if !hasUpstream {
		t.Error("HasUpstream should be true when hg summary reports update state")
	}
}

func TestGatherHGNotBehind(t *testing.T) {
	hgPath := testHgPath(t)
	_, local := initHgRepoWithRemote(t, hgPath)

	info := &vcsdetect.Info{VCS: "hg", RootDir: local}
	fetchPath := fetchHeadPath(info)
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(fetchPath, old, old); err != nil {
		t.Fatalf("age hg fetch marker: %v", err)
	}

	result, err := Gather(info, map[string]bool{"behind": true}, &Options{HgPath: hgPath})
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Behind {
		t.Error("Behind should be false when hg summary reports current, even with stale fetch marker")
	}
}

func TestGatherHGUnknownStatusWithBehind(t *testing.T) {
	hgPath := testHgPath(t)
	_, local := initHgRepoWithRemote(t, hgPath)
	if err := os.WriteFile(filepath.Join(local, "unknown.txt"), []byte("unknown\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info := &vcsdetect.Info{VCS: "hg", RootDir: local}
	fields := map[string]bool{"behind": true, "status": true}

	result, err := Gather(info, fields, &Options{HgPath: hgPath})
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	if result.Status != "*" {
		t.Errorf("Status = %q, want %q for unknown hg file", result.Status, "*")
	}
	if result.Behind {
		t.Error("Behind should be false when hg summary reports current")
	}
}

func TestBehindBySync(t *testing.T) {
	now := time.Unix(1700100000, 0)
	tests := []struct {
		name     string
		syncTime time.Time
		found    bool
		want     bool
	}{
		{"not found does not nag", time.Time{}, false, false},
		{"fresh sync does not nag", now.Add(-1 * time.Hour), true, false},
		{"stale sync nags", now.Add(-25 * time.Hour), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := behindBySync(tt.syncTime, tt.found, now, fetchStaleThreshold); got != tt.want {
				t.Errorf("behindBySync = %v, want %v", got, tt.want)
			}
		})
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
