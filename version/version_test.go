package version

import (
	"strings"
	"testing"
)

func TestStringContainsName(t *testing.T) {
	out := String("vcs")
	if !strings.HasPrefix(out, "vcs ") {
		t.Errorf("expected output to start with %q, got %q", "vcs ", out)
	}
}

func TestMultilineIncludesFields(t *testing.T) {
	out := Multiline("vcs")
	for _, want := range []string{"vcs", "version:", "commit:", "built:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected multiline output to contain %q, got %q", want, out)
		}
	}
}

func TestInfoFallsBackToBuildInfo(t *testing.T) {
	// Save and restore package-level vars so we don't affect other tests.
	origV, origC, origD := Version, Commit, BuildDate
	defer func() { Version, Commit, BuildDate = origV, origC, origD }()

	Version = "dev"
	Commit = "unknown"
	BuildDate = "unknown"

	// Just check it doesn't panic; values depend on how the test was built.
	_ = info()
}
