package main

import (
	"strings"
	"testing"

	"github.com/mikelward/vcs/runner"
)

func TestFindHg(t *testing.T) {
	// findHg should return something (at least "hg" as fallback).
	result := findHg()
	if result == "" {
		t.Error("findHg returned empty string")
	}
}

func TestDispatchReturnsErrorForUnknown(t *testing.T) {
	hgCmd = "hg" // ensure it's set
	err := dispatch("nonexistent_command_xyz", nil)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestDispatchForwardsArgs verifies that subcommands which used to drop
// user arguments now forward them to hg, matching vcs-git and vcs-jj.
// Dry-run mode prints the command instead of running it.
func TestDispatchForwardsArgs(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	tests := []struct {
		subcmd string
		args   []string
		want   string
	}{
		{"submit", []string{"-r", "."}, "submit -r ."},
		{"submitforce", []string{"-r", "."}, "--config hooks.preoutgoing= --config hooks.pre-push= submit -r ."},
		{"pending", []string{"--modified"}, "status --modified"},
		{"branches", []string{"--closed"}, "branches --closed"},
	}
	for _, tt := range tests {
		out, err := captureIO(t, func() error { return dispatch(tt.subcmd, tt.args) })
		if err != nil {
			t.Errorf("%s dry-run: %v\n%s", tt.subcmd, err, out)
			continue
		}
		if !strings.Contains(out, tt.want) {
			t.Errorf("%s dry-run output missing %q; got: %q", tt.subcmd, tt.want, out)
		}
	}
}
