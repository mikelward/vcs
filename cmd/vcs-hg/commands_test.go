package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vcs "github.com/mikelward/vcs"
)

func TestAllCommandsHandled(t *testing.T) {
	for _, cmd := range vcs.Commands {
		_, err := captureIO(t, func() error { return dispatch(cmd, nil) })
		if err != nil && strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "subcommand") {
			t.Errorf("command %q not handled by vcs-hg dispatch", cmd)
		}
	}
}

//
// base / graph / at_tip / map / outgoing / incoming / pending
//

func TestBase(t *testing.T) {
	_, local := newHgRepo(t)
	out, err := runDispatch(t, local, "base")
	if err != nil {
		t.Fatalf("base: %v\n%s", err, out)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("base missing initial: %q", out)
	}

	writeFile(t, local, "b.txt", "x\n")
	hgRun(t, local, "add", "b.txt")
	hgRun(t, local, "commit", "-m", "hg base test commit")
	out, _ = runDispatch(t, local, "base")
	if !strings.Contains(out, "hg base test commit") {
		t.Errorf("base after new commit: %q", out)
	}
}

func TestAtTip(t *testing.T) {
	_, local := newHgRepo(t)
	if _, err := runDispatch(t, local, "at_tip"); err != nil {
		t.Errorf("at_tip at head: %v", err)
	}

	// Add a second commit, then update to rev 0.
	writeFile(t, local, "t.txt", "x\n")
	hgRun(t, local, "add", "t.txt")
	hgRun(t, local, "commit", "-m", "c2")
	hgRun(t, local, "update", "-r", "0")
	if _, err := runDispatch(t, local, "at_tip"); err == nil {
		t.Errorf("at_tip not at head: want error")
	}
}

func TestBranch(t *testing.T) {
	_, local := newHgRepo(t)
	out, err := runDispatch(t, local, "branch")
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	if strings.TrimSpace(out) != "default" {
		t.Errorf("branch = %q, want default", strings.TrimSpace(out))
	}
}

func TestGraph(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "g.txt", "x\n")
	hgRun(t, local, "add", "g.txt")
	hgRun(t, local, "commit", "-m", "graph test")

	// Explicit args: limit to 2.
	out, _ := runDispatch(t, local, "graph", "-l", "2")
	if !strings.Contains(out, "graph test") || !strings.Contains(out, "initial commit") {
		t.Errorf("graph -l 2: %q", out)
	}

	// No args: only drafts. After clone the pulled rev is public, so only
	// the new "graph test" commit (draft) should appear.
	out, _ = runDispatch(t, local, "graph")
	if !strings.Contains(out, "graph test") {
		t.Errorf("graph should show draft: %q", out)
	}
	if strings.Contains(out, "initial commit") {
		t.Errorf("graph should exclude public initial: %q", out)
	}
}

func TestOutgoingIncoming(t *testing.T) {
	remote, local := newHgRepo(t)

	// No unpushed commits.
	out, _ := runDispatch(t, local, "outgoing")
	if strings.TrimSpace(out) != "" {
		t.Errorf("outgoing clean: %q", out)
	}

	writeFile(t, local, "new.txt", "x\n")
	hgRun(t, local, "add", "new.txt")
	hgRun(t, local, "commit", "-m", "local commit")
	out, err := runDispatch(t, local, "outgoing")
	if err != nil {
		t.Fatalf("outgoing: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("outgoing should have output")
	}

	// incoming: create a remote commit, fetch implicit via hg incoming.
	writeFile(t, remote, "r.txt", "x\n")
	hgRun(t, remote, "add", "r.txt")
	hgRun(t, remote, "commit", "-m", "remote commit")
	out, _ = runDispatch(t, local, "incoming")
	if !strings.Contains(out, "remote commit") {
		t.Errorf("incoming: %q", out)
	}
}

func TestPending(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "file.txt", "modified\n")
	out, _ := runDispatch(t, local, "pending")
	if strings.TrimSpace(out) == "" {
		t.Errorf("pending should show modified file")
	}
}

//
// commit / recommit / reword / describe
//

func TestCommit(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "c.txt", "x\n")
	hgRun(t, local, "add", "c.txt")
	_, err := runDispatch(t, local, "commit", "-m", "hg commit test", "-u", "test <t@t>")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{desc}"); got != "hg commit test" {
		t.Errorf("commit desc = %q", got)
	}
}

func TestRecommitChangesMessage(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "r.txt", "x\n")
	hgRun(t, local, "add", "r.txt")
	hgRun(t, local, "commit", "-m", "pre-recommit")
	_, err := runDispatch(t, local, "recommit", "-m", "after", "-u", "t <t@t>")
	if err != nil {
		t.Fatalf("recommit: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{desc}"); got != "after" {
		t.Errorf("recommit desc = %q", got)
	}
}

func TestRewordAndDescribe(t *testing.T) {
	_, local := newHgRepo(t)
	editor := editorStub(t)
	setenv(t, "HGEDITOR", editor)
	setenv(t, "EDITOR", editor)
	writeFile(t, local, "r.txt", "x\n")
	hgRun(t, local, "add", "r.txt")
	hgRun(t, local, "commit", "-m", "pre-reword")

	if _, err := runDispatch(t, local, "reword"); err != nil {
		t.Fatalf("reword: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{desc}"); got != "edited by test" {
		t.Errorf("reword = %q", got)
	}
}

func TestDescribe(t *testing.T) {
	_, local := newHgRepo(t)
	editor := editorStub(t)
	setenv(t, "HGEDITOR", editor)
	setenv(t, "EDITOR", editor)
	writeFile(t, local, "d.txt", "x\n")
	hgRun(t, local, "add", "d.txt")
	hgRun(t, local, "commit", "-m", "pre-describe")

	if _, err := runDispatch(t, local, "describe"); err != nil {
		t.Fatalf("describe: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{desc}"); got != "edited by test" {
		t.Errorf("describe = %q", got)
	}
}

//
// revert / status / show / diffstat / addremove
//

func TestRevert(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "r.txt", "clean\n")
	hgRun(t, local, "add", "r.txt")
	hgRun(t, local, "commit", "-m", "add r")
	writeFile(t, local, "r.txt", "dirty\n")
	if _, err := runDispatch(t, local, "revert", "r.txt"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(local, "r.txt"))
	if string(got) != "clean\n" {
		t.Errorf("revert: %q", got)
	}
}

func TestRevertNoArgs(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "a.txt", "clean-a\n")
	writeFile(t, local, "b.txt", "clean-b\n")
	hgRun(t, local, "add", "a.txt", "b.txt")
	hgRun(t, local, "commit", "-m", "add two")
	writeFile(t, local, "a.txt", "dirty-a\n")
	writeFile(t, local, "b.txt", "dirty-b\n")
	if _, err := runDispatch(t, local, "revert"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(local, "a.txt"))
	if string(got) != "clean-a\n" {
		t.Errorf("a.txt = %q", got)
	}
	got, _ = os.ReadFile(filepath.Join(local, "b.txt"))
	if string(got) != "clean-b\n" {
		t.Errorf("b.txt = %q", got)
	}
}

func TestStatus(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "s.txt", "x\n")
	hgRun(t, local, "add", "s.txt")
	out, _ := runDispatch(t, local, "status")
	if !strings.Contains(out, "s.txt") || !strings.Contains(out, "A") {
		t.Errorf("status: %q", out)
	}
	hgRun(t, local, "commit", "-m", "add s")
	out, _ = runDispatch(t, local, "status")
	if strings.TrimSpace(out) != "" {
		t.Errorf("status clean: %q", out)
	}
	writeFile(t, local, "u.txt", "x\n")
	out, _ = runDispatch(t, local, "status")
	if !strings.Contains(out, "u.txt") {
		t.Errorf("status unknown: %q", out)
	}
}

func TestShow(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "s.txt", "x\n")
	hgRun(t, local, "add", "s.txt")
	hgRun(t, local, "commit", "-m", "show target")
	out, _ := runDispatch(t, local, "show")
	if !strings.Contains(out, "show target") {
		t.Errorf("show: %q", out)
	}
}

func TestDiffstat(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "d.txt", "a\nb\nc\n")
	hgRun(t, local, "add", "d.txt")
	out, _ := runDispatch(t, local, "diffstat")
	if !strings.Contains(out, "d.txt") || !strings.Contains(out, "insertion") {
		t.Errorf("diffstat: %q", out)
	}
}

func TestAddremove(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "tracked.txt", "x\n")
	hgRun(t, local, "add", "tracked.txt")
	hgRun(t, local, "commit", "-m", "add tracked")

	writeFile(t, local, "new.txt", "y\n")
	os.Remove(filepath.Join(local, "tracked.txt"))
	if _, err := runDispatch(t, local, "addremove"); err != nil {
		t.Fatalf("addremove: %v", err)
	}
	status := hgOut(t, local, "status")
	if !strings.Contains(status, "A new.txt") {
		t.Errorf("addremove should stage new.txt: %q", status)
	}
	if !strings.Contains(status, "R tracked.txt") {
		t.Errorf("addremove should stage removal: %q", status)
	}
}

//
// rename / remove / copy / mv / rm
//

func TestRenameCopyMv(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "r1.txt", "x\n")
	hgRun(t, local, "add", "r1.txt")
	hgRun(t, local, "commit", "-m", "add r1")

	if _, err := runDispatch(t, local, "rename", "r1.txt", "r2.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "r2.txt")); err != nil {
		t.Errorf("rename target missing")
	}
	if _, err := os.Stat(filepath.Join(local, "r1.txt")); err == nil {
		t.Errorf("rename source still present")
	}
	hgRun(t, local, "commit", "-m", "rename test")

	writeFile(t, local, "c1.txt", "x\n")
	hgRun(t, local, "add", "c1.txt")
	hgRun(t, local, "commit", "-m", "add c1")
	if _, err := runDispatch(t, local, "copy", "c1.txt", "c2.txt"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "c2.txt")); err != nil {
		t.Errorf("copy missing target")
	}
	if _, err := os.Stat(filepath.Join(local, "c1.txt")); err != nil {
		t.Errorf("copy lost source")
	}
	hgRun(t, local, "commit", "-m", "cp test")

	writeFile(t, local, "m1.txt", "x\n")
	hgRun(t, local, "add", "m1.txt")
	hgRun(t, local, "commit", "-m", "add m1")
	if _, err := runDispatch(t, local, "mv", "m1.txt", "m2.txt"); err != nil {
		t.Fatalf("mv: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "m2.txt")); err != nil {
		t.Errorf("mv missing target")
	}
}

func TestRemove(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "r.txt", "x\n")
	hgRun(t, local, "add", "r.txt")
	hgRun(t, local, "commit", "-m", "add r")
	if _, err := runDispatch(t, local, "remove", "r.txt"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "r.txt")); err == nil {
		t.Errorf("remove did not delete")
	}
	status := hgOut(t, local, "status")
	if !strings.Contains(status, "R r.txt") {
		t.Errorf("remove status: %q", status)
	}
}

//
// prev / next / goto
//

func TestPrevNextGoto(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "c.txt", "x\n")
	hgRun(t, local, "add", "c.txt")
	hgRun(t, local, "commit", "-m", "c")
	head := hgOut(t, local, "log", "-r", ".", "--template", "{rev}")
	parent := hgOut(t, local, "log", "-r", ".^", "--template", "{rev}")

	if _, err := runDispatch(t, local, "prev"); err != nil {
		t.Fatalf("prev: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{rev}"); got != parent {
		t.Errorf("prev rev = %s, want %s", got, parent)
	}

	if _, err := runDispatch(t, local, "next"); err != nil {
		t.Fatalf("next: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{rev}"); got != head {
		t.Errorf("next rev = %s, want %s", got, head)
	}

	// goto
	if _, err := runDispatch(t, local, "goto", "-r", parent); err != nil {
		t.Fatalf("goto: %v", err)
	}
	if got := hgOut(t, local, "log", "-r", ".", "--template", "{rev}"); got != parent {
		t.Errorf("goto rev = %s, want %s", got, parent)
	}
}

//
// ignore / fetchtime / rootdir / unknown / track / untrack
//

func TestIgnore(t *testing.T) {
	_, local := newHgRepo(t)
	if _, err := runDispatch(t, local, "ignore", "*.log", "build/"); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(local, ".hgignore"))
	if !strings.Contains(string(got), "*.log") || !strings.Contains(string(got), "build/") {
		t.Errorf(".hgignore: %q", got)
	}
}

func TestRootdir(t *testing.T) {
	_, local := newHgRepo(t)
	// rootdir should print the repo root.
	sub := filepath.Join(local, "sub")
	os.MkdirAll(sub, 0755)
	out, err := runDispatch(t, sub, "rootdir")
	if err != nil {
		t.Fatalf("rootdir: %v", err)
	}
	want, _ := filepath.EvalSymlinks(local)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(out))
	if got != want {
		t.Errorf("rootdir = %q, want %q", got, want)
	}
}

func TestFetchtime(t *testing.T) {
	_, local := newHgRepo(t)
	out, err := runDispatch(t, local, "fetchtime")
	if err != nil {
		t.Fatalf("fetchtime: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("fetchtime empty")
	}
}

func TestUnknown(t *testing.T) {
	_, local := newHgRepo(t)
	writeFile(t, local, "u.txt", "x\n")
	out, err := runDispatch(t, local, "unknown")
	if err != nil {
		t.Fatalf("unknown: %v", err)
	}
	if !strings.Contains(out, "u.txt") {
		t.Errorf("unknown: %q", out)
	}
}
