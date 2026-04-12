package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	vcs "github.com/mikelward/vcs"
)

// TestAllCommandsHandled verifies every canonical VCS command is recognized
// by dispatch. Commands may fail because the test env is an empty dir, but
// none may return "unknown ... subcommand".
func TestAllCommandsHandled(t *testing.T) {
	for _, cmd := range vcs.Commands {
		_, err := captureIO(t, func() error { return dispatch(cmd, nil) })
		if err != nil && strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "subcommand") {
			t.Errorf("command %q not handled by vcs-git dispatch", cmd)
		}
	}
}

//
// base / graph / map / at_tip / outgoing / incoming / pending
//

func TestBase(t *testing.T) {
	_, local := newGitRepo(t)

	// Fresh clone: shows initial commit, no "(detached)".
	out, err := runDispatch(t, local, "base")
	if err != nil {
		t.Fatalf("base: %v\n%s", err, out)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("base missing 'initial commit': %q", out)
	}
	if strings.Contains(out, "(detached)") {
		t.Errorf("base unexpectedly marked detached: %q", out)
	}
	head := gitOut(t, local, "log", "-1", "--format=%h")
	if !strings.Contains(out, head) {
		t.Errorf("base missing short hash %s: %q", head, out)
	}

	// New commit updates the base line.
	writeFile(t, local, "basefile.txt", "x")
	gitRun(t, local, "add", "basefile.txt")
	gitRun(t, local, "commit", "-m", "base test commit")
	out, _ = runDispatch(t, local, "base")
	if !strings.Contains(out, "base test commit") {
		t.Errorf("base after new commit: %q", out)
	}

	// Detached HEAD: base prints "(detached) " prefix.
	sha := gitOut(t, local, "rev-parse", "HEAD")
	gitRun(t, local, "checkout", "-q", "--detach", sha)
	out, _ = runDispatch(t, local, "base")
	if !strings.Contains(out, "(detached)") {
		t.Errorf("detached base missing marker: %q", out)
	}
	if !strings.Contains(out, "base test commit") {
		t.Errorf("detached base missing commit: %q", out)
	}
}

func TestAtTip(t *testing.T) {
	_, local := newGitRepo(t)

	// On a branch: nil error.
	_, err := runDispatch(t, local, "at_tip")
	if err != nil {
		t.Errorf("at_tip on branch: %v", err)
	}

	// Detached HEAD: error.
	sha := gitOut(t, local, "rev-parse", "HEAD")
	gitRun(t, local, "checkout", "-q", "--detach", sha)
	_, err = runDispatch(t, local, "at_tip")
	if err == nil {
		t.Errorf("at_tip detached: want error, got nil")
	}
}

func TestBranch(t *testing.T) {
	_, local := newGitRepo(t)

	out, err := runDispatch(t, local, "branch")
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	branch := gitOut(t, local, "rev-parse", "--abbrev-ref", "HEAD")
	if strings.TrimSpace(out) != branch {
		t.Errorf("branch = %q, want %q", strings.TrimSpace(out), branch)
	}

	// Detached: prints nothing.
	sha := gitOut(t, local, "rev-parse", "HEAD")
	gitRun(t, local, "checkout", "-q", "--detach", sha)
	out, err = runDispatch(t, local, "branch")
	if err != nil {
		t.Fatalf("branch detached: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("branch detached should be empty, got %q", out)
	}
}

func TestGraph(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "g.txt", "x")
	gitRun(t, local, "add", "g.txt")
	gitRun(t, local, "commit", "-m", "graph test commit")

	// Explicit args show requested commits.
	out, _ := runDispatch(t, local, "graph", "-2")
	if !strings.Contains(out, "graph test commit") {
		t.Errorf("graph -2 missing head: %q", out)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("graph -2 missing parent: %q", out)
	}

	// No args shows outgoing only (unpushed head, not pushed initial).
	out, _ = runDispatch(t, local, "graph")
	if !strings.Contains(out, "graph test commit") {
		t.Errorf("graph missing outgoing commit: %q", out)
	}
	if strings.Contains(out, "initial commit") {
		t.Errorf("graph should omit pushed initial: %q", out)
	}
}

func TestMap(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "m.txt", "x")
	gitRun(t, local, "add", "m.txt")
	gitRun(t, local, "commit", "-m", "map outgoing commit")

	// On branch: shows outgoing, no graph markers, excludes pushed commits.
	out, _ := runDispatch(t, local, "map")
	if !strings.Contains(out, "map outgoing commit") {
		t.Errorf("map missing outgoing: %q", out)
	}
	if strings.Contains(out, "initial commit") {
		t.Errorf("map should exclude pushed initial: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "*") {
			t.Errorf("map on branch has graph marker: %q", line)
		}
	}

	// Detached HEAD: falls back to graph.
	sha := gitOut(t, local, "rev-parse", "HEAD")
	gitRun(t, local, "checkout", "-q", "--detach", sha)
	out, _ = runDispatch(t, local, "map")
	if !strings.Contains(out, "initial commit") {
		t.Errorf("map detached missing initial commit: %q", out)
	}
	if !strings.Contains(out, "*") {
		t.Errorf("map detached missing graph markers: %q", out)
	}
}

func TestOutgoingIncoming(t *testing.T) {
	remote, local := newGitRepo(t)

	// No unpushed commits.
	out, _ := runDispatch(t, local, "outgoing")
	if strings.TrimSpace(out) != "" {
		t.Errorf("outgoing clean: %q", out)
	}

	// Create and capture an unpushed commit.
	writeFile(t, local, "new.txt", "x")
	gitRun(t, local, "add", "new.txt")
	gitRun(t, local, "commit", "-m", "local commit")
	out, _ = runDispatch(t, local, "outgoing")
	if !strings.Contains(out, "local commit") {
		t.Errorf("outgoing missing commit: %q", out)
	}

	// incoming: push then add a remote commit via a second clone.
	gitRun(t, local, "push")
	gitRun(t, local, "fetch")
	out, _ = runDispatch(t, local, "incoming")
	if strings.TrimSpace(out) != "" {
		t.Errorf("incoming clean: %q", out)
	}

	// Second clone pushes a new commit.
	local2 := filepath.Join(filepath.Dir(local), "local2")
	cmd := exec.Command("git", "clone", "-q", remote, local2)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone2: %v\n%s", err, out)
	}
	gitRun(t, local2, "config", "commit.gpgsign", "false")
	gitRun(t, local2, "config", "core.hooksPath", filepath.Join(filepath.Dir(local), "nohooks"))
	gitRun(t, local2, "config", "user.email", "t@t")
	gitRun(t, local2, "config", "user.name", "T")
	writeFile(t, local2, "r.txt", "x")
	gitRun(t, local2, "add", "r.txt")
	gitRun(t, local2, "commit", "-m", "remote commit")
	gitRun(t, local2, "push")

	gitRun(t, local, "fetch")
	out, _ = runDispatch(t, local, "incoming")
	if !strings.Contains(out, "remote commit") {
		t.Errorf("incoming missing new commit: %q", out)
	}
}

// exec.Command is used above via stub wrapper; import directly.
var _ = filepath.Join

func TestPending(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "p.txt", "x")
	gitRun(t, local, "add", "p.txt")
	gitRun(t, local, "commit", "-m", "pending commit")

	out, _ := runDispatch(t, local, "pending")
	if !strings.Contains(out, "pending commit") {
		t.Errorf("pending missing unpushed commit: %q", out)
	}
}

//
// commit / amend / reword / describe / recommit
//

func TestCommit(t *testing.T) {
	_, local := newGitRepo(t)

	// With -m and no files: --all is injected, so staged and modified-tracked both commit.
	writeFile(t, local, "c.txt", "first")
	gitRun(t, local, "add", "c.txt")
	// Dispatch commit with just -m.
	_, err := runDispatch(t, local, "commit", "-m", "git commit test")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := gitOut(t, local, "log", "-1", "--format=%s"); got != "git commit test" {
		t.Errorf("commit subject = %q", got)
	}

	// With explicit file args: --all NOT added, unstaged modifications remain.
	writeFile(t, local, "staged.txt", "s")
	writeFile(t, local, "unstaged.txt", "u")
	gitRun(t, local, "add", "staged.txt", "unstaged.txt")
	// Modify unstaged after staging.
	writeFile(t, local, "unstaged.txt", "u-modified")
	_, err = runDispatch(t, local, "commit", "-m", "partial commit", "--", "staged.txt")
	if err != nil {
		t.Fatalf("partial commit: %v", err)
	}
	diff := gitOut(t, local, "diff", "--name-only")
	if !strings.Contains(diff, "unstaged.txt") {
		t.Errorf("partial commit should leave unstaged.txt modified; diff=%q", diff)
	}
}

func TestAmend(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "a.txt", "one")
	gitRun(t, local, "add", "a.txt")
	gitRun(t, local, "commit", "-m", "original")

	// Stage another file; amend with no file args should --all (pick it up).
	writeFile(t, local, "b.txt", "two")
	gitRun(t, local, "add", "b.txt")
	_, err := runDispatch(t, local, "amend")
	if err != nil {
		t.Fatalf("amend: %v", err)
	}
	if got := gitOut(t, local, "log", "-1", "--format=%s"); got != "original" {
		t.Errorf("amend changed subject to %q", got)
	}
	show := gitOut(t, local, "show", "--stat", "HEAD")
	if !strings.Contains(show, "b.txt") {
		t.Errorf("amend should include b.txt: %q", show)
	}
}

func TestRewordAndDescribe(t *testing.T) {
	_, local := newGitRepo(t)
	editor := editorStub(t)
	setenv(t, "GIT_EDITOR", editor)
	setenv(t, "EDITOR", editor)

	writeFile(t, local, "r.txt", "x")
	gitRun(t, local, "add", "r.txt")
	gitRun(t, local, "commit", "-m", "pre-reword")

	_, err := runDispatch(t, local, "reword")
	if err != nil {
		t.Fatalf("reword: %v", err)
	}
	if got := gitOut(t, local, "log", "-1", "--format=%s"); got != "edited by test" {
		t.Errorf("reword subject = %q", got)
	}

	_, err = runDispatch(t, local, "describe")
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if got := gitOut(t, local, "log", "-1", "--format=%s"); got != "edited by test" {
		t.Errorf("describe subject = %q", got)
	}
}

//
// revert / undo / uncommit
//

func TestRevertNoArgs(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "dirty.txt", "x")
	gitRun(t, local, "add", "dirty.txt")

	_, err := runDispatch(t, local, "revert")
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if s := gitOut(t, local, "status", "--short"); s != "" {
		t.Errorf("revert should clean tree; status=%q", s)
	}
}

func TestRevertWithArgs(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "keep.txt", "keep")
	writeFile(t, local, "revertme.txt", "original")
	gitRun(t, local, "add", "keep.txt", "revertme.txt")
	gitRun(t, local, "commit", "-m", "add two")
	writeFile(t, local, "keep.txt", "changed-keep")
	writeFile(t, local, "revertme.txt", "changed-revert")

	_, err := runDispatch(t, local, "revert", "revertme.txt")
	if err != nil {
		t.Fatalf("revert file: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(local, "revertme.txt"))
	if string(got) != "original" {
		t.Errorf("revertme.txt = %q", got)
	}
	got, _ = os.ReadFile(filepath.Join(local, "keep.txt"))
	if string(got) != "changed-keep" {
		t.Errorf("keep.txt = %q (should be preserved)", got)
	}
}

func TestUndo(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "u.txt", "x")
	gitRun(t, local, "add", "u.txt")
	gitRun(t, local, "commit", "-m", "to undo")
	parent := gitOut(t, local, "rev-parse", "HEAD~")

	_, err := runDispatch(t, local, "undo")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := gitOut(t, local, "rev-parse", "HEAD"); got != parent {
		t.Errorf("undo HEAD = %s, want %s", got, parent)
	}
	if _, err := os.Stat(filepath.Join(local, "u.txt")); err != nil {
		t.Errorf("undo should leave file in wd: %v", err)
	}
}

func TestUncommit(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "uc.txt", "x")
	gitRun(t, local, "add", "uc.txt")
	gitRun(t, local, "commit", "-m", "to uncommit")
	parent := gitOut(t, local, "rev-parse", "HEAD~")

	_, err := runDispatch(t, local, "uncommit")
	if err != nil {
		t.Fatalf("uncommit: %v", err)
	}
	if got := gitOut(t, local, "rev-parse", "HEAD"); got != parent {
		t.Errorf("uncommit HEAD = %s, want %s", got, parent)
	}
	// soft reset: uc.txt should be staged as A.
	status := gitOut(t, local, "status", "--short")
	if !strings.Contains(status, "A  uc.txt") {
		t.Errorf("uncommit should keep file staged; status=%q", status)
	}
}

//
// status / show / diffstat / addremove / drop / fetchtime
//

func TestStatus(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "s.txt", "x")
	gitRun(t, local, "add", "s.txt")
	out, _ := runDispatch(t, local, "status")
	if !strings.Contains(out, "s.txt") {
		t.Errorf("status missing file: %q", out)
	}
	if !strings.Contains(out, "A") {
		t.Errorf("status missing A prefix: %q", out)
	}

	writeFile(t, local, "untracked.txt", "u")
	out, _ = runDispatch(t, local, "status")
	if !strings.Contains(out, "untracked.txt") {
		t.Errorf("status missing untracked: %q", out)
	}
	if !strings.Contains(out, "??") {
		t.Errorf("status missing ?? prefix: %q", out)
	}

	gitRun(t, local, "add", "-A")
	gitRun(t, local, "commit", "-m", "cleanup")
	out, _ = runDispatch(t, local, "status")
	if strings.TrimSpace(out) != "" {
		t.Errorf("status clean: %q", out)
	}
}

func TestShow(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "h.txt", "x")
	gitRun(t, local, "add", "h.txt")
	gitRun(t, local, "commit", "-m", "show target")
	out, _ := runDispatch(t, local, "show", "HEAD")
	if !strings.Contains(out, "show target") {
		t.Errorf("show: %q", out)
	}
}

func TestDiffstat(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "d.txt", "hello\n")
	gitRun(t, local, "add", "d.txt")
	out, _ := runDispatch(t, local, "diffstat", "--cached")
	if !strings.Contains(out, "d.txt") {
		t.Errorf("diffstat missing file: %q", out)
	}
	if !strings.Contains(out, "insertion") {
		t.Errorf("diffstat missing insertion count: %q", out)
	}
}

func TestAddremove(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "tracked.txt", "x")
	gitRun(t, local, "add", "tracked.txt")
	gitRun(t, local, "commit", "-m", "add tracked")

	// Create new file, delete tracked.
	writeFile(t, local, "new.txt", "y")
	os.Remove(filepath.Join(local, "tracked.txt"))

	_, err := runDispatch(t, local, "addremove")
	if err != nil {
		t.Fatalf("addremove: %v", err)
	}
	status := gitOut(t, local, "status", "--short")
	if !strings.Contains(status, "new.txt") {
		t.Errorf("addremove missing new.txt: %q", status)
	}
	if !strings.Contains(status, "tracked.txt") {
		t.Errorf("addremove missing removal: %q", status)
	}
}

func TestDrop(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "base.txt", "x")
	gitRun(t, local, "add", "base.txt")
	gitRun(t, local, "commit", "-m", "drop base commit")

	writeFile(t, local, "target.txt", "x")
	gitRun(t, local, "add", "target.txt")
	gitRun(t, local, "commit", "-m", "drop this commit")
	target := gitOut(t, local, "rev-parse", "HEAD")

	writeFile(t, local, "child.txt", "x")
	gitRun(t, local, "add", "child.txt")
	gitRun(t, local, "commit", "-m", "drop child commit")

	// drop expects a commit-ish argument.
	_, err := runDispatch(t, local, "drop", target)
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	log := gitOut(t, local, "log", "--oneline")
	if strings.Contains(log, "drop this commit") {
		t.Errorf("drop did not remove target: %q", log)
	}
	if !strings.Contains(log, "drop child commit") {
		t.Errorf("drop removed child: %q", log)
	}
	if _, err := os.Stat(filepath.Join(local, "target.txt")); err == nil {
		t.Errorf("drop should remove target.txt")
	}
	if _, err := os.Stat(filepath.Join(local, "child.txt")); err != nil {
		t.Errorf("drop should keep child.txt: %v", err)
	}
}

func TestDropNoArg(t *testing.T) {
	_, local := newGitRepo(t)
	_, err := runDispatch(t, local, "drop")
	if err == nil {
		t.Errorf("drop with no args: want error")
	}
}

func TestFetchtime(t *testing.T) {
	_, local := newGitRepo(t)
	// Remove FETCH_HEAD written by newGitRepo's clone.
	gitDir := gitOut(t, local, "rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(local, gitDir)
	}
	os.Remove(filepath.Join(gitDir, "FETCH_HEAD"))

	_, err := runDispatch(t, local, "fetchtime")
	if err == nil {
		t.Errorf("fetchtime with no FETCH_HEAD: want error")
	}

	gitRun(t, local, "fetch")
	out, err := runDispatch(t, local, "fetchtime")
	if err != nil {
		t.Fatalf("fetchtime: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("fetchtime empty")
	}
}

//
// rename / remove / copy / mv / rm
//

func TestRename(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "r1.txt", "x")
	gitRun(t, local, "add", "r1.txt")
	gitRun(t, local, "commit", "-m", "add r1")

	_, err := runDispatch(t, local, "rename", "r1.txt", "r2.txt")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "r2.txt")); err != nil {
		t.Errorf("rename target missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "r1.txt")); err == nil {
		t.Errorf("rename source still present")
	}
	status := gitOut(t, local, "status", "--short")
	if !strings.Contains(status, "r2.txt") {
		t.Errorf("rename status: %q", status)
	}
}

func TestRemove(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "rm1.txt", "x")
	gitRun(t, local, "add", "rm1.txt")
	gitRun(t, local, "commit", "-m", "add rm1")

	_, err := runDispatch(t, local, "remove", "rm1.txt")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "rm1.txt")); err == nil {
		t.Errorf("remove did not delete file")
	}
	status := gitOut(t, local, "status", "--short")
	if !strings.Contains(status, "D") || !strings.Contains(status, "rm1.txt") {
		t.Errorf("remove status: %q", status)
	}
}

func TestCopy(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "c1.txt", "x")
	gitRun(t, local, "add", "c1.txt")
	gitRun(t, local, "commit", "-m", "add c1")

	_, err := runDispatch(t, local, "copy", "c1.txt", "c2.txt")
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "c2.txt")); err != nil {
		t.Errorf("copy target missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "c1.txt")); err != nil {
		t.Errorf("copy source gone: %v", err)
	}
	status := gitOut(t, local, "status", "--short")
	if !strings.Contains(status, "A") || !strings.Contains(status, "c2.txt") {
		t.Errorf("copy status: %q", status)
	}
}

func TestCopyFewArgs(t *testing.T) {
	_, local := newGitRepo(t)
	_, err := runDispatch(t, local, "copy", "only-one")
	if err == nil {
		t.Errorf("copy 1 arg: want error")
	}
}

func TestMoveAndRm(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "m1.txt", "x")
	gitRun(t, local, "add", "m1.txt")
	gitRun(t, local, "commit", "-m", "add m1")
	if _, err := runDispatch(t, local, "move", "m1.txt", "m2.txt"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "m2.txt")); err != nil {
		t.Errorf("move target missing")
	}
	gitRun(t, local, "commit", "-m", "mv test")

	writeFile(t, local, "x1.txt", "x")
	gitRun(t, local, "add", "x1.txt")
	gitRun(t, local, "commit", "-m", "add x1")
	if _, err := runDispatch(t, local, "rm", "x1.txt"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "x1.txt")); err == nil {
		t.Errorf("rm did not delete")
	}
}

//
// prev / next / goto
//

func TestPrevNext(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "n.txt", "x")
	gitRun(t, local, "add", "n.txt")
	gitRun(t, local, "commit", "-m", "for-next")
	head := gitOut(t, local, "rev-parse", "HEAD")
	parent := gitOut(t, local, "rev-parse", "HEAD~")

	if _, err := runDispatch(t, local, "prev"); err != nil {
		t.Fatalf("prev: %v", err)
	}
	if got := gitOut(t, local, "rev-parse", "HEAD"); got != parent {
		t.Errorf("prev HEAD=%s want=%s", got, parent)
	}

	if _, err := runDispatch(t, local, "next"); err != nil {
		t.Fatalf("next: %v", err)
	}
	if got := gitOut(t, local, "rev-parse", "HEAD"); got != head {
		t.Errorf("next HEAD=%s want=%s", got, head)
	}

	// next at tip has no child to walk to.
	branch := gitOut(t, local, "rev-parse", "--abbrev-ref", "HEAD")
	// next moved to head (detached); switch back to branch so we're at tip.
	gitRun(t, local, "checkout", branch)
	_, err := runDispatch(t, local, "next")
	if err == nil {
		t.Errorf("next at tip: want error")
	}
}

func TestGoto(t *testing.T) {
	_, local := newGitRepo(t)
	writeFile(t, local, "g1.txt", "x")
	gitRun(t, local, "add", "g1.txt")
	gitRun(t, local, "commit", "-m", "c1")
	writeFile(t, local, "g2.txt", "x")
	gitRun(t, local, "add", "g2.txt")
	gitRun(t, local, "commit", "-m", "c2")

	target := gitOut(t, local, "rev-parse", "HEAD~")
	if _, err := runDispatch(t, local, "goto", target); err != nil {
		t.Fatalf("goto: %v", err)
	}
	if got := gitOut(t, local, "rev-parse", "HEAD"); got != target {
		t.Errorf("goto HEAD=%s want=%s", got, target)
	}
}

// (splitGitArgs edge cases are in main_test.go)

//
// ignore / precommit (hook dispatch)
//

func TestIgnore(t *testing.T) {
	_, local := newGitRepo(t)
	if _, err := runDispatch(t, local, "ignore", "*.log", "build/"); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(local, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "*.log") {
		t.Errorf(".gitignore missing *.log: %q", got)
	}
	if !strings.Contains(string(got), "build/") {
		t.Errorf(".gitignore missing build/: %q", got)
	}
}

func TestPrecommitMissingHook(t *testing.T) {
	_, local := newGitRepo(t)
	// No hook file; precommit should return error and print "No ... pre-commit hook".
	out, err := runDispatch(t, local, "precommit")
	if err == nil {
		t.Errorf("precommit with no hook: want error")
	}
	if !strings.Contains(out, "pre-commit") {
		t.Errorf("precommit output should mention hook: %q", out)
	}
}

func TestPrecommitRunsHook(t *testing.T) {
	_, local := newGitRepo(t)
	gitDir := gitOut(t, local, "rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(local, gitDir)
	}
	hook := filepath.Join(gitDir, "hooks", "pre-commit")
	os.MkdirAll(filepath.Dir(hook), 0755)
	os.WriteFile(hook, []byte("#!/bin/sh\necho HOOK-RAN\n"), 0755)

	out, err := runDispatch(t, local, "precommit")
	if err != nil {
		t.Fatalf("precommit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HOOK-RAN") {
		t.Errorf("precommit didn't run hook: %q", out)
	}
}

//
// review / upload / uploadchain
//
// gitReview always calls `git push` then (if `gh` exists) `gh pr view` and
// on failure `gh pr create --fill [--reviewer X|--draft]`. The shell tests
// exercised a gerrit path, but vcs-git in Go has no gerrit branch; we test
// the github-only behavior.

// stubPath installs stub scripts for git and gh on PATH. Both scripts log
// their invocation to per-command log files. The gh stub exits 1 on
// `pr view` (simulating no existing PR) so `pr create` gets called.
// Returns (gitLog, ghLog).
func stubPath(t *testing.T) (gitLog, ghLog string) {
	t.Helper()
	dir := t.TempDir()
	gitLog = filepath.Join(dir, "git.log")
	ghLog = filepath.Join(dir, "gh.log")

	gitScript := "#!/bin/sh\necho \"$@\" >> " + gitLog + "\nexit 0\n"
	ghScript := "#!/bin/sh\necho \"$@\" >> " + ghLog + "\n" +
		"case \"$1 $2\" in\n" +
		"  'pr view') exit 1 ;;\n" +
		"  'repo view') echo main ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(gitScript), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(ghScript), 0755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	return
}

func readLog(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestReviewNoReviewerDraft(t *testing.T) {
	_, local := newGitRepo(t)
	gitLog, ghLog := stubPath(t)

	if _, err := runDispatch(t, local, "review"); err != nil {
		t.Fatalf("review: %v", err)
	}
	gl := readLog(t, gitLog)
	if !strings.Contains(gl, "push") {
		t.Errorf("review should git push: %q", gl)
	}
	gh := readLog(t, ghLog)
	if !strings.Contains(gh, "pr view") {
		t.Errorf("review should gh pr view: %q", gh)
	}
	if !strings.Contains(gh, "pr create") {
		t.Errorf("review should gh pr create: %q", gh)
	}
	if !strings.Contains(gh, "--draft") {
		t.Errorf("review without reviewer should be --draft: %q", gh)
	}
}

func TestReviewReviewerFlags(t *testing.T) {
	for _, flag := range []struct {
		name string
		args []string
	}{
		{"dash-r", []string{"-r", "alice"}},
		{"dash-m", []string{"-m", "alice"}},
		{"long", []string{"--reviewer", "alice"}},
		{"equals", []string{"--reviewer=alice"}},
	} {
		t.Run(flag.name, func(t *testing.T) {
			_, local := newGitRepo(t)
			_, ghLog := stubPath(t)

			args := append([]string{"review"}, flag.args...)
			if _, err := runDispatch(t, local, args[0], args[1:]...); err != nil {
				t.Fatalf("review %v: %v", flag.args, err)
			}
			gh := readLog(t, ghLog)
			if !strings.Contains(gh, "--reviewer") || !strings.Contains(gh, "alice") {
				t.Errorf("review %v missing reviewer in gh log: %q", flag.args, gh)
			}
			if strings.Contains(gh, "--draft") {
				t.Errorf("review %v should NOT be --draft: %q", flag.args, gh)
			}
		})
	}
}

func TestUploadDispatch(t *testing.T) {
	_, local := newGitRepo(t)
	gitLog, ghLog := stubPath(t)

	if _, err := runDispatch(t, local, "upload"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !strings.Contains(readLog(t, gitLog), "push") {
		t.Errorf("upload should push")
	}
	if !strings.Contains(readLog(t, ghLog), "pr create") {
		t.Errorf("upload should create PR")
	}
}

func TestUploadchainDispatch(t *testing.T) {
	_, local := newGitRepo(t)
	gitLog, ghLog := stubPath(t)

	if _, err := runDispatch(t, local, "uploadchain"); err != nil {
		t.Fatalf("uploadchain: %v", err)
	}
	if !strings.Contains(readLog(t, gitLog), "push") {
		t.Errorf("uploadchain should push")
	}
	if !strings.Contains(readLog(t, ghLog), "pr create") {
		t.Errorf("uploadchain should create PR")
	}
}
