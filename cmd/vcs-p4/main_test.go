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
