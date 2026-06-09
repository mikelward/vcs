package autofetch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mikelward/vcs/runner"
)

// recordingSpawn returns a Spawn func that records every call into
// the given slice and a never-fails handler. Tests inspect the
// recorder to verify dispatch decisions without forking real VCS
// binaries.
type spawnCall struct {
	name string
	args []string
}

func recordingSpawn() (*[]spawnCall, func(string, []string, []*os.File) error) {
	var calls []spawnCall
	return &calls, func(name string, args []string, _ []*os.File) error {
		calls = append(calls, spawnCall{name, append([]string(nil), args...)})
		return nil
	}
}

// touchOld writes path with the given mtime. Helper for setting up
// stale/fresh markers in tests.
func touchOld(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// makeRepo creates a directory with the given marker subpath laid
// down and returns the repo root. layoutDirs lets a caller create
// extra empty dirs (used to force colocated jj layouts via .git/).
func makeRepo(t *testing.T, layoutDirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range layoutDirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunNotInRepo(t *testing.T) {
	dir := t.TempDir()
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{Cwd: dir, Spawn: spawn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionNotInRepo {
		t.Errorf("action = %v, want ActionNotInRepo", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

func TestRunGitStaleSpawnsFetch(t *testing.T) {
	root := makeRepo(t, ".git")
	// No FETCH_HEAD: counts as stale (initial fetch case).
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{Cwd: root, Spawn: spawn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.name != "git" {
		t.Errorf("spawn name = %q, want %q", got.name, "git")
	}
	wantArgs := []string{"-C", root, "fetch", "--quiet"}
	if !equalArgs(got.args, wantArgs) {
		t.Errorf("spawn args = %v, want %v", got.args, wantArgs)
	}
}

func TestRunGitFreshSkipsFetch(t *testing.T) {
	root := makeRepo(t, ".git")
	// FETCH_HEAD touched 1 minute ago — well within the default 1h gate.
	now := time.Now()
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-1*time.Minute))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

func TestRunGitStaleSpawnsAfterMaxAge(t *testing.T) {
	root := makeRepo(t, ".git")
	now := time.Now()
	// Marker is 2h old; default MaxAge is 1h.
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-2*time.Hour))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Errorf("got %d spawn calls, want 1", len(*calls))
	}
}

func TestRunCustomMaxAge(t *testing.T) {
	root := makeRepo(t, ".git")
	now := time.Now()
	// Marker is 2 minutes old; MaxAge is 1 minute, so it's stale.
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-2*time.Minute))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:    root,
		Spawn:  spawn,
		MaxAge: 1 * time.Minute,
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched (1min max-age, 2min-old marker)", action)
	}
	if len(*calls) != 1 {
		t.Errorf("got %d spawn calls, want 1", len(*calls))
	}
}

func TestRunHgStaleSpawnsPull(t *testing.T) {
	root := makeRepo(t, ".hg/store")
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{Cwd: root, Spawn: spawn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.name != "hg" {
		t.Errorf("spawn name = %q, want %q", got.name, "hg")
	}
	wantArgs := []string{"-R", root, "pull", "--quiet"}
	if !equalArgs(got.args, wantArgs) {
		t.Errorf("spawn args = %v, want %v", got.args, wantArgs)
	}
}

func TestRunHgFreshSkipsPull(t *testing.T) {
	root := makeRepo(t, ".hg/store")
	now := time.Now()
	touchOld(t, filepath.Join(root, ".hg", "store", "00changelog.i"), now.Add(-1*time.Minute))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

func TestRunHgUsesCustomPath(t *testing.T) {
	root := makeRepo(t, ".hg/store")
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:    root,
		Spawn:  spawn,
		HgPath: "/usr/local/bin/chg",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	if (*calls)[0].name != "/usr/local/bin/chg" {
		t.Errorf("spawn name = %q, want %q", (*calls)[0].name, "/usr/local/bin/chg")
	}
}

// jj non-colocated: only .jj/ exists; marker lives under
// .jj/repo/store/git/FETCH_HEAD.
func TestRunJJNonColocatedStale(t *testing.T) {
	root := makeRepo(t, ".jj/repo/store/git")
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{Cwd: root, Spawn: spawn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.name != "jj" {
		t.Errorf("spawn name = %q, want %q", got.name, "jj")
	}
	wantArgs := []string{"--repository", root, "git", "fetch", "--quiet"}
	if !equalArgs(got.args, wantArgs) {
		t.Errorf("spawn args = %v, want %v", got.args, wantArgs)
	}
}

func TestRunJJNonColocatedFresh(t *testing.T) {
	root := makeRepo(t, ".jj/repo/store/git")
	now := time.Now()
	touchOld(t, filepath.Join(root, ".jj", "repo", "store", "git", "FETCH_HEAD"), now.Add(-1*time.Minute))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

// jj colocated: both .jj/ and .git/ exist at root. `jj git fetch`
// writes to .git/FETCH_HEAD, so the gate must read from there. Verify
// by simultaneously backdating the non-colocated marker and freshening
// the colocated one — Run must skip (proving it consulted the
// colocated path, not the stale non-colocated one).
func TestRunJJColocatedFresh(t *testing.T) {
	root := makeRepo(t, ".jj/repo/store/git", ".git")
	now := time.Now()
	// Colocated marker: fresh.
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-1*time.Minute))
	// Non-colocated marker: very stale, shouldn't matter.
	touchOld(t, filepath.Join(root, ".jj", "repo", "store", "git", "FETCH_HEAD"), now.Add(-48*time.Hour))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh (colocated path should win)", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

func TestRunJJColocatedStale(t *testing.T) {
	root := makeRepo(t, ".jj/repo/store/git", ".git")
	now := time.Now()
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-2*time.Hour))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	if (*calls)[0].name != "jj" {
		t.Errorf("spawn name = %q, want %q", (*calls)[0].name, "jj")
	}
}

// jj records the backing git dir in .jj/repo/store/git_target; when present
// it is authoritative over the .git-dir-exists heuristic. Point it at a
// custom location with a fresh FETCH_HEAD while the top-level .git holds a
// stale one — Run must skip, proving git_target was consulted.
func TestRunJJGitTargetWins(t *testing.T) {
	root := makeRepo(t, ".jj/repo/store", ".git", "custom-git")
	if err := os.WriteFile(filepath.Join(root, ".jj", "repo", "store", "git_target"), []byte("../../../custom-git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	touchOld(t, filepath.Join(root, "custom-git", "FETCH_HEAD"), now.Add(-1*time.Minute))
	touchOld(t, filepath.Join(root, ".git", "FETCH_HEAD"), now.Add(-48*time.Hour))
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:   root,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh (git_target path should win)", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

// makePiperRepo creates a jj workspace whose store type marks a non-git
// backend, so vcsdetect reports Backend="piper".
func makePiperRepo(t *testing.T) string {
	t.Helper()
	root := makeRepo(t, ".jj/repo/store")
	if err := os.WriteFile(filepath.Join(root, ".jj", "repo", "store", "type"), []byte("piper"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

// jj piper backend: no git FETCH_HEAD marker. Staleness is gated on the
// op-log sync time, and the refresh command is `jj piper pull`.
func TestRunJJPiperStaleSpawnsSync(t *testing.T) {
	root := makePiperRepo(t)
	now := time.Now()
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:        root,
		Spawn:      spawn,
		Now:        func() time.Time { return now },
		JJSyncTime: func(string) (time.Time, bool) { return now.Add(-2 * time.Hour), true },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d spawn calls, want 1", len(*calls))
	}
	got := (*calls)[0]
	if got.name != "jj" {
		t.Errorf("spawn name = %q, want %q", got.name, "jj")
	}
	wantArgs := []string{"--repository", root, "piper", "pull"}
	if !equalArgs(got.args, wantArgs) {
		t.Errorf("spawn args = %v, want %v", got.args, wantArgs)
	}
}

func TestRunJJPiperFreshSkipsSync(t *testing.T) {
	root := makePiperRepo(t)
	now := time.Now()
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:        root,
		Spawn:      spawn,
		Now:        func() time.Time { return now },
		JJSyncTime: func(string) (time.Time, bool) { return now.Add(-1 * time.Minute), true },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

// No recorded sync (e.g. a freshly created workspace) counts as stale so the
// first sync still runs.
func TestRunJJPiperNoSyncSpawns(t *testing.T) {
	root := makePiperRepo(t)
	calls, spawn := recordingSpawn()

	action, err := Run(&Options{
		Cwd:        root,
		Spawn:      spawn,
		JJSyncTime: func(string) (time.Time, bool) { return time.Time{}, false },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
	if len(*calls) != 1 {
		t.Errorf("got %d spawn calls, want 1", len(*calls))
	}
}

// Spawn errors propagate as Run errors but the Action is still
// reported as ActionFetched (the decision was to fetch; only the
// kernel-level Start failed). This keeps the error path observable
// while not duplicating the "we tried" outcome.
func TestRunSpawnErrorPropagates(t *testing.T) {
	root := makeRepo(t, ".git")
	failing := func(string, []string, []*os.File) error { return errors.New("boom") }

	action, err := Run(&Options{Cwd: root, Spawn: failing})
	if err == nil {
		t.Fatal("Run: want error, got nil")
	}
	if action != ActionFetched {
		t.Errorf("action = %v, want ActionFetched", action)
	}
}

// TestRunGitWorktreeResolvesMarker covers the worktree case: `.git`
// at the worktree root is a *file* pointing at the per-worktree
// gitdir (under <main>/.git/worktrees/<name>), and FETCH_HEAD lives
// in the shared common dir under the main `.git`. The naive
// `<root>/.git/FETCH_HEAD` join would never resolve, so without the
// gitMarkerPath fallback the gate would always treat worktrees as
// stale and re-fetch on every cd.
func TestRunGitWorktreeResolvesMarker(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	main := t.TempDir()
	gitInit(t, main)
	gitCommit(t, main, "initial")

	wtParent := t.TempDir()
	wt := filepath.Join(wtParent, "wt")
	gitRun(t, main, "worktree", "add", "-b", "wt-branch", wt)

	// `git fetch` in a worktree writes to whichever path
	// `git rev-parse --git-path FETCH_HEAD` resolves to, which varies
	// by git version: newer git (~2.21+) uses a per-worktree path
	// under <main-gitdir>/worktrees/<name>/FETCH_HEAD, older git
	// shares the common dir's <main-gitdir>/FETCH_HEAD. Ask git which
	// one it'd use and touch *that*, so the test stays aligned with
	// what the production gitMarkerPath() probes regardless of
	// version.
	gpCmd := exec.Command("git", "-C", wt, "rev-parse", "--git-path", "FETCH_HEAD")
	gpCmd.Env = runner.CleanGitEnv()
	gitPathOut, err := gpCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse --git-path FETCH_HEAD: %v", err)
	}
	wtFetchHead := strings.TrimSpace(string(gitPathOut))
	if !filepath.IsAbs(wtFetchHead) {
		wtFetchHead = filepath.Join(wt, wtFetchHead)
	}
	now := time.Now()
	touchOld(t, wtFetchHead, now.Add(-1*time.Minute))

	calls, spawn := recordingSpawn()
	action, err := Run(&Options{
		Cwd:   wt,
		Spawn: spawn,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh (worktree marker should resolve to common dir)", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

// TestRunGitLockHeldSkipsFetch verifies that when vcs pull holds the
// fetch lock, auto-fetch skips rather than racing on FETCH_HEAD.
func TestRunGitLockHeldSkipsFetch(t *testing.T) {
	root := makeRepo(t, ".git")
	// Simulate a concurrent pull by holding the fetch lock ourselves.
	lockPath := filepath.Join(root, ".git", "vcs-fetch.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}

	calls, spawn := recordingSpawn()
	action, err := Run(&Options{Cwd: root, Spawn: spawn})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action != ActionFresh {
		t.Errorf("action = %v, want ActionFresh (lock held, skip fetch)", action)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d spawn calls, want 0", len(*calls))
	}
}

// TestDetachedSpawnDryRunSkipsFork covers the global -n/--dry-run
// flag: detachedSpawn must consult runner.DryRun and skip the fork,
// otherwise `vcs -n auto-fetch` triggers a real network fetch
// despite being documented as a preview-only mode.
func TestDetachedSpawnDryRunSkipsFork(t *testing.T) {
	prev := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = prev })

	// /no/such/binary would fail loudly if Spawn ran for real (Start
	// returns ENOENT). Under DryRun we expect a clean nil with no
	// process started.
	if err := detachedSpawn("/no/such/binary", []string{"arg"}, nil); err != nil {
		t.Errorf("detachedSpawn(DryRun=true) error = %v, want nil", err)
	}
}

// TestDetachedSpawnInjectsSSHBatchMode verifies that GIT_SSH_COMMAND is
// set to 'ssh -o BatchMode=yes' when the caller has not set it, so an
// SSH passphrase prompt can't hang the orphaned process.
func TestDetachedSpawnInjectsSSHBatchMode(t *testing.T) {
	old, hadOld := os.LookupEnv("GIT_SSH_COMMAND")
	os.Unsetenv("GIT_SSH_COMMAND")
	if hadOld {
		t.Cleanup(func() { os.Setenv("GIT_SSH_COMMAND", old) })
	} else {
		t.Cleanup(func() { os.Unsetenv("GIT_SSH_COMMAND") })
	}

	// Use a named pipe so ReadFile blocks until the child writes,
	// making the wait deterministic with no sleep.
	fifo := filepath.Join(t.TempDir(), "sshcmd")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if err := detachedSpawn("sh", []string{"-c", `printf '%s' "$GIT_SSH_COMMAND" > ` + fifo}, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fifo) // blocks until child writes and closes its end
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BatchMode=yes") {
		t.Errorf("GIT_SSH_COMMAND = %q, want ssh -o BatchMode=yes", string(data))
	}
}

// TestDetachedSpawnPreservesCustomSSHCommand verifies that a
// user-defined GIT_SSH_COMMAND (e.g. a custom SSH wrapper) is left
// intact and not overridden by autofetch.
func TestDetachedSpawnPreservesCustomSSHCommand(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "my-custom-ssh")

	fifo := filepath.Join(t.TempDir(), "sshcmd")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if err := detachedSpawn("sh", []string{"-c", `printf '%s' "$GIT_SSH_COMMAND" > ` + fifo}, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fifo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "my-custom-ssh") {
		t.Errorf("GIT_SSH_COMMAND = %q, want my-custom-ssh", string(data))
	}
}

// TestDetachedSpawnPreservesGITSSH verifies that a user-defined GIT_SSH
// wrapper is left intact: injecting GIT_SSH_COMMAND would take precedence
// and silently bypass the configured wrapper.
func TestDetachedSpawnPreservesGITSSH(t *testing.T) {
	old, hadOld := os.LookupEnv("GIT_SSH_COMMAND")
	os.Unsetenv("GIT_SSH_COMMAND")
	if hadOld {
		t.Cleanup(func() { os.Setenv("GIT_SSH_COMMAND", old) })
	} else {
		t.Cleanup(func() { os.Unsetenv("GIT_SSH_COMMAND") })
	}
	t.Setenv("GIT_SSH", "my-ssh-wrapper")

	fifo := filepath.Join(t.TempDir(), "sshcmd")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if err := detachedSpawn("sh", []string{"-c", `printf '%s' "$GIT_SSH_COMMAND" > ` + fifo}, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fifo)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "" {
		t.Errorf("GIT_SSH_COMMAND = %q, want empty (GIT_SSH is set)", string(data))
	}
}

// TestCoreSSHCommandConfigured verifies that coreSSHCommandConfigured
// detects a repo-level core.sshCommand setting and returns false for
// non-git VCS binaries.
func TestCoreSSHCommandConfigured(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	if coreSSHCommandConfigured("git") {
		t.Error("expected false before core.sshCommand is set")
	}

	gitRun(t, repo, "config", "core.sshCommand", "ssh -i /custom/key")

	if !coreSSHCommandConfigured("git") {
		t.Error("expected true after core.sshCommand is set")
	}
	if !coreSSHCommandConfigured("jj") {
		t.Error("expected true for jj (also uses git config)")
	}
	if coreSSHCommandConfigured("hg") {
		t.Error("expected false for hg (not a git-like VCS)")
	}
}

// gitInit / gitCommit / gitRun: tiny helpers for the worktree test
// above. Inline rather than imported to keep autofetch's test
// dependencies tight (just stdlib + runner).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "init", "--quiet", "-b", "main")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	gitRun(t, dir, "commit", "--allow-empty", "--no-verify", "-m", msg)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(runner.CleanGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func equalArgs(a, b []string) bool {
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
