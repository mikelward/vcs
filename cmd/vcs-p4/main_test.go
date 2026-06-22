package main

import (
	"strings"
	"testing"

	"github.com/mikelward/vcs/runner"
)

func TestDispatchReturnsErrorForUnknown(t *testing.T) {
	err := dispatch("nonexistent_command_xyz", nil)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestDispatchDryRun(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("add", []string{"foo.txt"})
	})
	if err != nil {
		t.Fatalf("dispatch dry-run: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " add foo.txt"
	if !strings.Contains(out, want) {
		t.Errorf("dry-run output missing %q; got: %q", want, out)
	}
}

func TestDiffstatUsesSummaryFlag(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("diffstat", nil)
	})
	if err != nil {
		t.Fatalf("dispatch diffstat: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " diff -ds"
	if !strings.Contains(out, want) {
		t.Errorf("diffstat dry-run output missing %q; got: %q", want, out)
	}
}

func TestRevertWithNoArgsAddsFilespec(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("revert", nil)
	})
	if err != nil {
		t.Fatalf("dispatch revert: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " revert -Si '//$P4CLIENT/...'"
	if !strings.Contains(out, want) {
		t.Errorf("bare revert dry-run output missing %q; got: %q", want, out)
	}
}

func TestRevertWithArgsPassesThrough(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("revert", []string{"foo.txt"})
	})
	if err != nil {
		t.Fatalf("dispatch revert foo.txt: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " revert foo.txt"
	if !strings.Contains(out, want) {
		t.Errorf("revert with arg dry-run output missing %q; got: %q", want, out)
	}
	if strings.Contains(out, "revert ...") {
		t.Errorf("revert with arg should not add ... filespec; got: %q", out)
	}
}

func TestPendingDryRunScopesToClient(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("pending", nil)
	})
	if err != nil {
		t.Fatalf("dispatch pending: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " changes -s pending -c '$P4CLIENT'"
	if !strings.Contains(out, want) {
		t.Errorf("pending dry-run output missing %q; got: %q", want, out)
	}
}

func TestRestoreWithArgsPassesThrough(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("restore", []string{"foo.txt"})
	})
	if err != nil {
		t.Fatalf("dispatch restore foo.txt: %v\n%s", err, out)
	}
	want := "+ " + p4Cmd + " revert foo.txt"
	if !strings.Contains(out, want) {
		t.Errorf("restore dry-run output missing %q; got: %q", want, out)
	}
}
