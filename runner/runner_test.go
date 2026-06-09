package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
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

func TestPrintError(t *testing.T) {
	t.Run("nil error prints nothing", func(t *testing.T) {
		out := captureStderr(t, func() { PrintError("vcs-test", nil) })
		if out != "" {
			t.Errorf("PrintError(nil) wrote %q, want nothing", out)
		}
	})

	t.Run("exit error with inherited stderr prints nothing", func(t *testing.T) {
		err := exec.Command("false").Run()
		if err == nil {
			t.Fatal("expected `false` to fail")
		}
		out := captureStderr(t, func() { PrintError("vcs-test", err) })
		if out != "" {
			t.Errorf("PrintError(ExitError) wrote %q, want nothing", out)
		}
	})

	t.Run("exit error with captured stderr prints it", func(t *testing.T) {
		// Output() captures the child's stderr on ExitError.Stderr instead
		// of inheriting it; PrintError must surface it or the diagnostic
		// is lost entirely.
		_, err := exec.Command("sh", "-c", "echo boom >&2; exit 3").Output()
		if err == nil {
			t.Fatal("expected command to fail")
		}
		out := captureStderr(t, func() { PrintError("vcs-test", err) })
		if out != "boom\n" {
			t.Errorf("PrintError(ExitError with Stderr) wrote %q, want %q", out, "boom\n")
		}
	})

	t.Run("plain error printed with prefix", func(t *testing.T) {
		out := captureStderr(t, func() {
			PrintError("vcs-test", errors.New("unknown subcommand: bogus"))
		})
		want := "vcs-test: unknown subcommand: bogus\n"
		if out != want {
			t.Errorf("PrintError wrote %q, want %q", out, want)
		}
	})
}
