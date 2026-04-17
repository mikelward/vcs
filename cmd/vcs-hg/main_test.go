package main

import (
	"testing"
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

func TestHasRebaseDest(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"--update"}, false},
		{[]string{"-d", "tip"}, true},
		{[]string{"--dest", "tip"}, true},
		{[]string{"--dest=tip"}, true},
		{[]string{"--update", "-d", "tip"}, true},
	}
	for _, c := range cases {
		if got := hasRebaseDest(c.args); got != c.want {
			t.Errorf("hasRebaseDest(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}
