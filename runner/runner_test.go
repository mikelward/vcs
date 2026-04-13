package runner

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr runs fn with os.Stderr redirected, returning what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w

	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()

	fn()
	w.Close()
	os.Stderr = old
	return string(<-done)
}

func withDryRun(t *testing.T, fn func()) {
	t.Helper()
	old := DryRun
	DryRun = true
	t.Cleanup(func() { DryRun = old })
	fn()
}

func TestRunDryRun(t *testing.T) {
	var out string
	withDryRun(t, func() {
		out = captureStderr(t, func() {
			if err := Run("git", "commit", "-m", "hello world"); err != nil {
				t.Fatalf("Run returned error in dry-run mode: %v", err)
			}
		})
	})
	if !strings.Contains(out, "+ git commit -m 'hello world'") {
		t.Errorf("expected quoted dry-run output, got: %q", out)
	}
}

func TestExecDryRun(t *testing.T) {
	var out string
	withDryRun(t, func() {
		out = captureStderr(t, func() {
			if err := Exec("git", "status"); err != nil {
				t.Fatalf("Exec returned error in dry-run mode: %v", err)
			}
		})
	})
	if !strings.Contains(out, "+ git status") {
		t.Errorf("expected dry-run output, got: %q", out)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo", "foo"},
		{"foo.bar", "foo.bar"},
		{"--flag=value", "--flag=value"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"", "''"},
		{"a*b", "'a*b'"},
	}
	for _, c := range cases {
		if got := shellQuote(c.in); got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunNotDryRun(t *testing.T) {
	// Sanity check that a non-dry-run invocation of a real command still
	// works end-to-end. Uses "true" (always available on unix).
	if DryRun {
		t.Skip("DryRun set from environment")
	}
	if err := Run("true"); err != nil {
		t.Fatalf("Run(true): %v", err)
	}
}
