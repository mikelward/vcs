package main

import (
	"testing"
)

func TestDispatchReturnsErrorForUnknown(t *testing.T) {
	err := dispatch("nonexistent_command_xyz", nil)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestDispatchAddremoveIsNoop(t *testing.T) {
	err := dispatch("addremove", nil)
	if err != nil {
		t.Errorf("addremove should be a no-op, got error: %v", err)
	}
}

func TestDispatchBranchIsNoop(t *testing.T) {
	err := dispatch("branch", nil)
	if err != nil {
		t.Errorf("branch should be a no-op for jj, got error: %v", err)
	}
}

func TestDispatchEvolveIsNoop(t *testing.T) {
	err := dispatch("evolve", nil)
	if err != nil {
		t.Errorf("evolve should be a no-op for jj, got error: %v", err)
	}
}
