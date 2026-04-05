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
