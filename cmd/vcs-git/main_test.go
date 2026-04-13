package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mikelward/vcs/runner"
)

func TestSplitGitArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFlags []string
		wantFiles []string
	}{
		{
			name:      "no args",
			args:      nil,
			wantFlags: nil,
			wantFiles: nil,
		},
		{
			name:      "only flags",
			args:      []string{"-a", "--verbose"},
			wantFlags: []string{"-a", "--verbose"},
			wantFiles: nil,
		},
		{
			name:      "only files",
			args:      []string{"foo.go", "bar.go"},
			wantFlags: nil,
			wantFiles: []string{"foo.go", "bar.go"},
		},
		{
			name:      "flags then files",
			args:      []string{"-a", "foo.go"},
			wantFlags: []string{"-a"},
			wantFiles: []string{"foo.go"},
		},
		{
			name:      "message flag",
			args:      []string{"-m", "hello", "foo.go"},
			wantFlags: []string{"-m", "hello"},
			wantFiles: []string{"foo.go"},
		},
		{
			name:      "double dash",
			args:      []string{"-a", "--", "foo.go"},
			wantFlags: []string{"-a", "--"},
			wantFiles: []string{"foo.go"},
		},
		{
			name:      "double dash only",
			args:      []string{"--"},
			wantFlags: []string{"--"},
			wantFiles: nil,
		},
		{
			name:      "-C flag with value",
			args:      []string{"-C", "abc123", "-v"},
			wantFlags: []string{"-C", "abc123", "-v"},
			wantFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFlags, gotFiles := splitGitArgs(tt.args)
			if !reflect.DeepEqual(gotFlags, tt.wantFlags) {
				t.Errorf("flags = %v, want %v", gotFlags, tt.wantFlags)
			}
			if !reflect.DeepEqual(gotFiles, tt.wantFiles) {
				t.Errorf("files = %v, want %v", gotFiles, tt.wantFiles)
			}
		})
	}
}

func TestDispatchReturnsErrorForUnknown(t *testing.T) {
	err := dispatch("nonexistent_command_xyz", nil)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestDispatchDryRun verifies that with runner.DryRun set, dispatch prints
// the underlying git command to stderr and doesn't execute it.
func TestDispatchDryRun(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	out, err := captureIO(t, func() error {
		return dispatch("commit", []string{"-m", "hello world"})
	})
	if err != nil {
		t.Fatalf("dispatch dry-run: %v\n%s", err, out)
	}
	want := "+ git commit -m 'hello world' --all"
	if !strings.Contains(out, want) {
		t.Errorf("dry-run output missing %q; got: %q", want, out)
	}
}
