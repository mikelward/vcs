// Package autofetch implements the `vcs auto-fetch` subcommand: a
// fast, prompt-friendly entry point that decides whether to spawn a
// detached background fetch for the current repo.
//
// The shell prompt hook ("just cd'd into a repo, refresh the remote
// refs if they're stale") wants three things from one binary call:
//
//  1. VCS detection (skip silently if not in a tracked repo)
//  2. A per-VCS fetch-marker file whose mtime tracks the last fetch
//     (and whose location varies — colocated jj writes to
//     .git/FETCH_HEAD, not .jj/repo/store/git/FETCH_HEAD)
//  3. The right fetch command per VCS, spawned detached so the prompt
//     returns immediately
//
// Putting all of that here keeps the per-VCS knowledge in one place
// instead of duplicated across bash/zsh/fish/nu prompt hooks. Each
// shell's hook collapses to PWD-changed gate + auth check + a single
// `vcs auto-fetch &` call.
package autofetch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mikelward/vcs/internal/fetchlock"
	"github.com/mikelward/vcs/runner"
	"github.com/mikelward/vcs/vcsdetect"
)

// Action is what Run decided to do. Exported so tests and --verbose
// output can describe outcomes without parsing free-form strings.
type Action int

const (
	// ActionNotInRepo: PWD is not inside any tracked VCS repo. No-op.
	ActionNotInRepo Action = iota
	// ActionFresh: marker exists and mtime is within max-age. No-op.
	ActionFresh
	// ActionFetched: spawned a detached fetch (or, with --dry-run / a
	// test-injected Spawn, recorded the call).
	ActionFetched
	// ActionUnsupported: detected VCS has no fetch dispatch wired.
	ActionUnsupported
)

func (a Action) String() string {
	switch a {
	case ActionNotInRepo:
		return "not-in-repo"
	case ActionFresh:
		return "fresh"
	case ActionFetched:
		return "fetched"
	case ActionUnsupported:
		return "unsupported"
	}
	return "unknown"
}

// Options controls Run. All fields are optional; zero values pick
// sensible defaults (see Run).
type Options struct {
	// MaxAge is how old the fetch marker must be before Run will spawn
	// another fetch. Default 1h, matching shrc's BG_FETCH_INTERVAL_SECONDS.
	MaxAge time.Duration

	// ForceVCS overrides autodetection ("git", "hg", or "jj"). Empty
	// to autodetect from cwd.
	ForceVCS string

	// HgPath overrides the hg binary path (matches `vcs prompt-info
	// --hg-path=...`). Empty means use $PATH lookup.
	HgPath string

	// Cwd is the directory to detect from. Empty means os.Getwd.
	// Injectable so tests can point at a temp dir.
	Cwd string

	// Now returns the current time. Defaults to time.Now. Injectable
	// so tests can drive the mtime gate without sleeping.
	Now func() time.Time

	// Spawn is called to launch the fetch command. Defaults to
	// detachedSpawn (Setsid + exec.Command + Start, no Wait).
	// Injected by tests to record (name, args) without forking.
	// extraFiles are passed to the child via cmd.ExtraFiles so that
	// inherited file descriptors (e.g. a fetch lock) remain open until
	// the child exits.
	Spawn func(name string, args []string, extraFiles []*os.File) error
}

// Run executes one auto-fetch decision: detect the VCS in cwd, check
// the staleness gate, and (if stale) spawn the appropriate fetch.
// Returns the Action taken so the caller can emit verbose output if
// desired.
//
// Run is silent on the no-op paths and never blocks on network I/O —
// the spawned fetch is fully detached and inherits no stdio.
func Run(opts *Options) (Action, error) {
	if opts == nil {
		opts = &Options{}
	}
	if opts.MaxAge == 0 {
		opts.MaxAge = time.Hour
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Spawn == nil {
		opts.Spawn = detachedSpawn
	}


	dir := opts.Cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return ActionNotInRepo, fmt.Errorf("getwd: %w", err)
		}
	}

	info, err := vcsdetect.Detect(dir)
	if err != nil || info == nil || info.VCS == "" || info.RootDir == "" {
		return ActionNotInRepo, nil
	}
	if opts.ForceVCS != "" {
		info.VCS = opts.ForceVCS
	}

	marker, fetchName, fetchArgs, ok := dispatch(info, opts.HgPath)
	if !ok {
		return ActionUnsupported, nil
	}

	if !markerStale(marker, opts.MaxAge, opts.Now()) {
		return ActionFresh, nil
	}

	// For git, hold the fetch lock so a concurrent `vcs pull` (which
	// also runs an internal git fetch) can't race us on FETCH_HEAD.
	// We pass the lock fd to the child via ExtraFiles; the OS releases
	// it when git exits. Non-blocking: if pull already holds the lock,
	// skip — pull is fetching for us.
	var extraFiles []*os.File
	if info.VCS == "git" {
		lockFile, lockErr := fetchlock.TryLock(filepath.Dir(marker))
		if errors.Is(lockErr, fetchlock.ErrLocked) {
			return ActionFresh, nil
		}
		if lockErr == nil {
			extraFiles = []*os.File{lockFile}
		}
		// Other errors (permissions etc.): proceed without lock.
	}

	if err := opts.Spawn(fetchName, fetchArgs, extraFiles); err != nil {
		for _, f := range extraFiles {
			f.Close()
		}
		return ActionFetched, fmt.Errorf("spawn %s: %w", fetchName, err)
	}
	for _, f := range extraFiles {
		f.Close() // parent releases its copy; child holds the lock until it exits
	}
	return ActionFetched, nil
}

// dispatch returns (markerPath, fetchCmd, fetchArgs, supported) for
// the given VCS. The fetch command is the foreground form; the
// caller spawns it detached.
func dispatch(info *vcsdetect.Info, hgPath string) (marker, name string, args []string, ok bool) {
	switch info.VCS {
	case "git":
		// `<root>/.git` is a directory in the common case but a file
		// in worktrees and submodules, where it holds a `gitdir: ...`
		// pointer to the real gitdir; FETCH_HEAD also lives in the
		// shared common dir, not the per-worktree gitdir. Ask git to
		// resolve all of that — `--git-path FETCH_HEAD` follows the
		// same logic git itself uses for fetch, so the mtime gate
		// reads the same file `git fetch` writes.
		marker = gitMarkerPath(info.RootDir)
		// `git -C ROOT fetch --quiet` resolves the gitdir for us
		// (handles worktrees and submodules without us having to
		// invoke `git rev-parse --git-dir` separately).
		return marker, "git", []string{"-C", info.RootDir, "fetch", "--quiet"}, true
	case "hg":
		// 00changelog.i is rewritten on every `hg pull` regardless of
		// whether new changesets arrived, so its mtime tracks the
		// last pull attempt.
		marker = filepath.Join(info.RootDir, ".hg", "store", "00changelog.i")
		hg := hgPath
		if hg == "" {
			hg = "hg"
		}
		return marker, hg, []string{"-R", info.RootDir, "pull", "--quiet"}, true
	case "jj":
		// Colocated workspaces (the default `jj git init` layout)
		// have a top-level `.git` directory and `jj git fetch` writes
		// to .git/FETCH_HEAD. Non-colocated workspaces keep the git
		// store under .jj/repo/store/git/. Prefer the colocated path
		// when present so the mtime gate doesn't always treat
		// colocated repos as stale.
		colocated := filepath.Join(info.RootDir, ".git")
		if st, err := os.Stat(colocated); err == nil && st.IsDir() {
			marker = filepath.Join(colocated, "FETCH_HEAD")
		} else {
			marker = filepath.Join(info.RootDir, ".jj", "repo", "store", "git", "FETCH_HEAD")
		}
		return marker, "jj", []string{"--repository", info.RootDir, "git", "fetch", "--quiet"}, true
	}
	return "", "", nil, false
}

// markerStale returns true when the marker is missing or its mtime is
// older than maxAge relative to now. A missing marker counts as stale
// because the repo has never been fetched (or was just initialized),
// and an initial fetch is exactly what the caller wants.
func markerStale(path string, maxAge time.Duration, now time.Time) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}
	return now.Sub(fi.ModTime()) > maxAge
}

// gitMarkerPath returns the absolute path to FETCH_HEAD for the git
// repo rooted at rootDir, handling worktrees and submodules where
// `.git` is a file (or the gitdir lives elsewhere) and FETCH_HEAD
// resides in the shared common dir.
//
// The fast path — `<root>/.git/FETCH_HEAD` when `.git` is a real
// directory — needs no fork. Worktrees/submodules fall through to
// `git rev-parse --git-path FETCH_HEAD`, which is one fork but only
// runs on the cd-into-worktree case.
func gitMarkerPath(rootDir string) string {
	dotGit := filepath.Join(rootDir, ".git")
	if fi, err := os.Stat(dotGit); err == nil && fi.IsDir() {
		return filepath.Join(dotGit, "FETCH_HEAD")
	}
	cmd := exec.Command("git", "-C", rootDir, "rev-parse", "--git-path", "FETCH_HEAD")
	cmd.Env = runner.CleanGitEnv()
	out, err := cmd.Output()
	if err != nil {
		// Fall back to the naive path; markerStale will treat it as
		// stale (file won't exist) and we'll fetch — which is the
		// safe direction for an unresolvable repo.
		return filepath.Join(dotGit, "FETCH_HEAD")
	}
	resolved := strings.TrimSpace(string(out))
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(rootDir, resolved)
	}
	return resolved
}

// detachedSpawn launches name+args in a new session with no stdio,
// then returns immediately without waiting. The child becomes its
// own session leader (Setsid) so it isn't killed when the calling
// shell's prompt finishes drawing.
//
// In dry-run mode (VCS_DRY_RUN=1, set by `vcs -n`/`--dry-run`), the
// command is printed via runner.PrintCommand and not actually
// spawned, so users can preview prompt-hook behavior without
// triggering a real network fetch.
//
// GIT_TERMINAL_PROMPT=0 is set on every spawn so an HTTPS-creds
// prompt can't hang the orphaned process indefinitely. jj uses git's
// network layer underneath, so the same env var helps it too; the
// var is harmless to hg.
//
// TODO: also set GIT_SSH_COMMAND='ssh -o BatchMode=yes' (or equivalent)
// so SSH key passphrases can't prompt and hang the orphaned process.
// GIT_TERMINAL_PROMPT=0 only covers HTTPS credential prompts.
func detachedSpawn(name string, args []string, extraFiles []*os.File) error {
	if runner.DryRun {
		runner.PrintCommand(name, args)
		return nil
	}
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.ExtraFiles = extraFiles
	return cmd.Start()
}
