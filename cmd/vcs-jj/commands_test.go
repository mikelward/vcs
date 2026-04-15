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
			t.Errorf("command %q not handled by vcs-jj dispatch", cmd)
		}
	}
}

//
// base / at_tip / branch / outgoing / pending
//

func TestBase(t *testing.T) {
	repo := newJJRepo(t)

	out, err := runDispatch(t, repo, "base")
	if err != nil {
		t.Fatalf("base: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("base empty on fresh repo")
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(line, "@ ") {
			t.Errorf("base on fresh repo should have no @ line: %q", line)
		}
	}

	writeFile(t, repo, "b.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "jj base test commit")
	out, _ = runDispatch(t, repo, "base")
	if !strings.Contains(out, "jj base test commit") {
		t.Errorf("base missing desc: %q", out)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(line, "@ ") {
			t.Errorf("undescribed @: no @ line expected: %q", line)
		}
	}

	jjRun(t, repo, "describe", "-m", "wc description")
	out, _ = runDispatch(t, repo, "base")
	hasAt, hasStar := false, false
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(line, "@ ") {
			hasAt = true
		}
		if strings.HasPrefix(line, "* ") {
			hasStar = true
		}
	}
	if !hasAt || !hasStar {
		t.Errorf("described wc: want both @ and *; got %q", out)
	}
}

func TestAtTip(t *testing.T) {
	repo := newJJRepo(t)
	if _, err := runDispatch(t, repo, "at_tip"); err != nil {
		t.Errorf("at_tip on fresh repo: %v", err)
	}

	writeFile(t, repo, "a.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "A")
	parentID := jjOut(t, repo, "log", "--no-graph", "-r", "@-", "-T", "change_id.shortest()")
	writeFile(t, repo, "b.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "B")
	jjRun(t, repo, "edit", parentID)
	if _, err := runDispatch(t, repo, "at_tip"); err == nil {
		t.Errorf("at_tip editing non-tip: want error")
	}
}

func TestBranch(t *testing.T) {
	repo := newJJRepo(t)
	out, err := runDispatch(t, repo, "branch")
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("branch should be empty: %q", out)
	}
}

func TestOutgoing(t *testing.T) {
	repo := newJJRepo(t)
	out, _ := runDispatch(t, repo, "outgoing")
	if strings.TrimSpace(out) != "" {
		t.Errorf("outgoing empty repo: %q", out)
	}
	writeFile(t, repo, "f.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "jj test commit")
	out, _ = runDispatch(t, repo, "outgoing")
	if !strings.Contains(out, "jj test commit") {
		t.Errorf("outgoing missing: %q", out)
	}
}

func TestPending(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "pending commit")
	out, _ := runDispatch(t, repo, "pending")
	if !strings.Contains(out, "pending commit") {
		t.Errorf("pending: %q", out)
	}
}

//
// commit / amend / recommit / reword / describe
//

func TestCommit(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "c.txt", "x\n")
	if _, err := runDispatch(t, repo, "commit", "-m", "jj commit test"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	desc := jjOut(t, repo, "log", "--no-graph", "-r", "@-", "-T", "description")
	if !strings.Contains(desc, "jj commit test") {
		t.Errorf("commit desc = %q", desc)
	}
}

func TestAmend(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "base.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "base")
	writeFile(t, repo, "addon.txt", "y\n")
	if _, err := runDispatch(t, repo, "amend"); err != nil {
		t.Fatalf("amend: %v", err)
	}
	show := jjOut(t, repo, "file", "show", "addon.txt", "-r", "@-")
	if !strings.Contains(show, "y") {
		t.Errorf("amend did not squash into parent: %q", show)
	}
}

func TestRecommit(t *testing.T) {
	repo := newJJRepo(t)
	if _, err := runDispatch(t, repo, "recommit", "-m", "jj recommit msg"); err != nil {
		t.Fatalf("recommit: %v", err)
	}
	desc := jjOut(t, repo, "log", "--no-graph", "-r", "@", "-T", "description")
	if !strings.Contains(desc, "jj recommit msg") {
		t.Errorf("recommit: %q", desc)
	}
}

func TestReword(t *testing.T) {
	repo := newJJRepo(t)
	if _, err := runDispatch(t, repo, "reword", "jj reword msg"); err != nil {
		t.Fatalf("reword: %v", err)
	}
	desc := jjOut(t, repo, "log", "--no-graph", "-r", "@", "-T", "description")
	if !strings.Contains(desc, "jj reword msg") {
		t.Errorf("reword: %q", desc)
	}
}

func TestDescribe(t *testing.T) {
	repo := newJJRepo(t)
	if _, err := runDispatch(t, repo, "describe", "-m", "jj describe test"); err != nil {
		t.Fatalf("describe: %v", err)
	}
	desc := jjOut(t, repo, "log", "--no-graph", "-r", "@", "-T", "description")
	if !strings.Contains(desc, "jj describe test") {
		t.Errorf("describe: %q", desc)
	}
}

//
// revert / undo / status
//

func TestRevertNoArgs(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "original\n")
	jjRun(t, repo, "commit", "-m", "add f")
	writeFile(t, repo, "f.txt", "changed\n")

	if _, err := runDispatch(t, repo, "revert"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(repo, "f.txt"))
	if string(got) != "original\n" {
		t.Errorf("revert should restore working copy; f.txt = %q", got)
	}
}

func TestRevertFileArg(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "keep.txt", "keep\n")
	writeFile(t, repo, "revertme.txt", "original\n")
	jjRun(t, repo, "commit", "-m", "add two")
	writeFile(t, repo, "keep.txt", "changed-keep\n")
	writeFile(t, repo, "revertme.txt", "changed-revert\n")

	if _, err := runDispatch(t, repo, "revert", "revertme.txt"); err != nil {
		t.Fatalf("revert file: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(repo, "revertme.txt"))
	if string(got) != "original\n" {
		t.Errorf("revertme.txt = %q", got)
	}
	got, _ = os.ReadFile(filepath.Join(repo, "keep.txt"))
	if string(got) != "changed-keep\n" {
		t.Errorf("keep.txt = %q (should be preserved)", got)
	}
}

func TestRevertThenUndo(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "original\n")
	jjRun(t, repo, "commit", "-m", "add f")
	writeFile(t, repo, "f.txt", "changed\n")
	jjRun(t, repo, "commit", "-m", "modify f")

	if _, err := runDispatch(t, repo, "revert", "-r", "@-", "--destination", "@"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	out := jjOut(t, repo, "log", "--no-graph", "-r", "children(@)", "-T", "description")
	if !strings.Contains(strings.ToLower(out), "revert") {
		t.Errorf("revert should create revert commit: %q", out)
	}

	if _, err := runDispatch(t, repo, "undo"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	out = jjOut(t, repo, "log", "--no-graph", "-r", "all()", "-T", "description")
	if strings.Contains(strings.ToLower(out), "revert") {
		t.Errorf("undo should remove revert: %q", out)
	}
}

func TestStatus(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "s.txt", "x\n")
	out, _ := runDispatch(t, repo, "status")
	if !strings.Contains(out, "s.txt") {
		t.Errorf("status missing file: %q", out)
	}
	if !strings.Contains(out, "A") {
		t.Errorf("status missing A prefix: %q", out)
	}

	jjRun(t, repo, "commit", "-m", "status test")
	out, _ = runDispatch(t, repo, "status")
	if strings.TrimSpace(out) != "" {
		t.Errorf("status after commit: %q", out)
	}

	writeFile(t, repo, "d.txt", "x\n")
	jjRun(t, repo, "describe", "-m", "already described")
	out, _ = runDispatch(t, repo, "status")
	if strings.TrimSpace(out) != "" {
		t.Errorf("status for described @: want empty, got %q", out)
	}
}

//
// show / diffstat / addremove / drop
//

func TestShow(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "show target")
	out, _ := runDispatch(t, repo, "show", "@-")
	if !strings.Contains(out, "show target") {
		t.Errorf("show: %q", out)
	}
}

func TestDiffstat(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "d.txt", "x\n")
	out, _ := runDispatch(t, repo, "diffstat")
	if !strings.Contains(out, "d.txt") {
		t.Errorf("diffstat: %q", out)
	}
}

func TestAddremove(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "x\n")
	if _, err := runDispatch(t, repo, "addremove"); err != nil {
		t.Errorf("addremove: %v", err)
	}
	out, _ := runDispatch(t, repo, "status")
	if !strings.Contains(out, "f.txt") {
		t.Errorf("auto-track missing f.txt in status: %q", out)
	}
}

func TestDrop(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "f.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "jj drop target")
	dropID := jjOut(t, repo, "log", "--no-graph", "-r", "@-", "-T", "change_id.shortest()")
	if _, err := runDispatch(t, repo, "drop", dropID); err != nil {
		t.Fatalf("drop: %v", err)
	}
	out := jjOut(t, repo, "log", "--no-graph", "-r", "all()", "-T", "description")
	if strings.Contains(out, "jj drop target") {
		t.Errorf("drop: commit still present: %q", out)
	}
}

//
// rename / remove / copy / mv / rm
//

func TestRename(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "r1.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "add r1")
	if _, err := runDispatch(t, repo, "rename", "r1.txt", "r2.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "r2.txt")); err != nil {
		t.Errorf("rename target missing")
	}
	if _, err := os.Stat(filepath.Join(repo, "r1.txt")); err == nil {
		t.Errorf("rename source still present")
	}
}

func TestRenameWrongArgs(t *testing.T) {
	repo := newJJRepo(t)
	_, err := runDispatch(t, repo, "rename", "only-one")
	if err == nil {
		t.Errorf("rename with 1 arg: want error")
	}
	if err != nil && !strings.Contains(err.Error(), "usage") {
		t.Errorf("error should mention usage: %v", err)
	}
}

func TestRemove(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "r.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "add r")
	if _, err := runDispatch(t, repo, "remove", "r.txt"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "r.txt")); err == nil {
		t.Errorf("remove did not delete")
	}
}

func TestCopy(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "c1.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "add c1")
	if _, err := runDispatch(t, repo, "copy", "c1.txt", "c2.txt"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "c2.txt")); err != nil {
		t.Errorf("copy target missing")
	}
	if _, err := os.Stat(filepath.Join(repo, "c1.txt")); err != nil {
		t.Errorf("copy source gone")
	}
}

func TestMv(t *testing.T) {
	repo := newJJRepo(t)
	writeFile(t, repo, "m1.txt", "x\n")
	jjRun(t, repo, "commit", "-m", "add m1")
	if _, err := runDispatch(t, repo, "mv", "m1.txt", "m2.txt"); err != nil {
		t.Fatalf("mv: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "m2.txt")); err != nil {
		t.Errorf("mv target missing")
	}
	if _, err := os.Stat(filepath.Join(repo, "m1.txt")); err == nil {
		t.Errorf("mv source still present")
	}
}

//
// ignore / rootdir
//

func TestIgnore(t *testing.T) {
	repo := newJJRepo(t)
	if _, err := runDispatch(t, repo, "ignore", "*.log", "build/"); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if !strings.Contains(string(got), "*.log") || !strings.Contains(string(got), "build/") {
		t.Errorf(".gitignore: %q", got)
	}
}

func TestRootdir(t *testing.T) {
	repo := newJJRepo(t)
	sub := filepath.Join(repo, "sub")
	os.MkdirAll(sub, 0755)
	out, err := runDispatch(t, sub, "rootdir")
	if err != nil {
		t.Fatalf("rootdir: %v", err)
	}
	want, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(out))
	if got != want {
		t.Errorf("rootdir = %q, want %q", got, want)
	}
}

//
// Backend-dependent dispatch (push/pull/submit/fastforward/review/upload).
// newJJRepo was created with `jj git init` so backend() is "git". The
// handlers call `jj git push` / `jj git fetch`. Stub jj on PATH to capture
// what dispatch invokes.
//

func stubJJ(t *testing.T) (jjLog, ghLog string) {
	t.Helper()
	dir := t.TempDir()
	jjLog = filepath.Join(dir, "jj.log")
	ghLog = filepath.Join(dir, "gh.log")
	jjScript := "#!/bin/sh\necho \"$@\" >> " + jjLog + "\nexit 0\n"
	ghScript := "#!/bin/sh\necho \"$@\" >> " + ghLog + "\n" +
		"case \"$1 $2\" in\n" +
		"  'pr view') exit 1 ;;\n" +
		"esac\nexit 0\n"
	os.WriteFile(filepath.Join(dir, "jj"), []byte(jjScript), 0755)
	os.WriteFile(filepath.Join(dir, "gh"), []byte(ghScript), 0755)
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

func TestPushPullSubmit(t *testing.T) {
	repo := newJJRepo(t)
	jjLog, _ := stubJJ(t)

	if _, err := runDispatch(t, repo, "push"); err != nil {
		t.Fatalf("push: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git push") {
		t.Errorf("push should call jj git push: %q", readLog(t, jjLog))
	}

	os.Truncate(jjLog, 0)
	if _, err := runDispatch(t, repo, "pull"); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git fetch") {
		t.Errorf("pull should call jj git fetch: %q", readLog(t, jjLog))
	}

	os.Truncate(jjLog, 0)
	if _, err := runDispatch(t, repo, "fastforward"); err != nil {
		t.Fatalf("fastforward: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git fetch") {
		t.Errorf("fastforward should call jj git fetch: %q", readLog(t, jjLog))
	}

	os.Truncate(jjLog, 0)
	if _, err := runDispatch(t, repo, "submit"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git push") {
		t.Errorf("submit should call jj git push: %q", readLog(t, jjLog))
	}
}

func TestPresubmitGitBackendNotSupported(t *testing.T) {
	repo := newJJRepo(t)
	stubJJ(t)
	_, err := runDispatch(t, repo, "presubmit")
	if err == nil {
		t.Errorf("presubmit on git backend: want error")
	}
}

func TestReviewGitBackend(t *testing.T) {
	repo := newJJRepo(t)
	jjLog, _ := stubJJ(t)
	if _, err := runDispatch(t, repo, "review"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git push") {
		t.Errorf("review should jj git push: %q", readLog(t, jjLog))
	}
}

func TestUploadUploadchain(t *testing.T) {
	repo := newJJRepo(t)
	jjLog, _ := stubJJ(t)

	if _, err := runDispatch(t, repo, "upload"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git push") {
		t.Errorf("upload git backend should jj git push: %q", readLog(t, jjLog))
	}

	os.Truncate(jjLog, 0)
	if _, err := runDispatch(t, repo, "uploadchain"); err != nil {
		t.Fatalf("uploadchain: %v", err)
	}
	if !strings.Contains(readLog(t, jjLog), "git push") {
		t.Errorf("uploadchain git backend should jj git push: %q", readLog(t, jjLog))
	}
}
