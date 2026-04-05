package vcs

import (
	"sort"
	"testing"
)

func TestCommandsAreSorted(t *testing.T) {
	if !sort.StringsAreSorted(Commands) {
		t.Error("Commands list is not sorted; keep it sorted for readability")
	}
}

func TestCommandsNotEmpty(t *testing.T) {
	if len(Commands) == 0 {
		t.Error("Commands list is empty")
	}
}
