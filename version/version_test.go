package version

import (
	"strings"
	"testing"
)

func TestStringHasName(t *testing.T) {
	out := String("vcs")
	if !strings.HasPrefix(out, "vcs ") {
		t.Errorf("expected output to start with %q, got %q", "vcs ", out)
	}
}

func TestMultilineHasFields(t *testing.T) {
	out := Multiline("vcs")
	for _, want := range []string{"vcs", "version:", "commit:", "built:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected multiline output to contain %q, got %q", want, out)
		}
	}
}

func TestReadReturnsSomething(t *testing.T) {
	// Just check it doesn't panic and returns populated fields; the exact
	// values depend on how the test binary was built.
	info := Read()
	if info.Version == "" || info.Commit == "" || info.Date == "" {
		t.Errorf("expected all fields to be populated, got %+v", info)
	}
}
