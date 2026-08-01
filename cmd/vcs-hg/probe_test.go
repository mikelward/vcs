package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHgProbeOutcomes pins the distinction TestMain draws between an hg that
// is absent and one that is present but will not run. Collapsing them is what
// made `go test ./cmd/vcs-hg/` print ok having executed nothing, so the two
// outcomes are asserted rather than described.
//
// It re-runs this test binary as a subprocess with a doctored PATH, because
// the behavior under test lives in TestMain and cannot be reached any other
// way from inside a test.
func TestHgProbeOutcomes(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skipf("cannot locate the test binary: %v", err)
	}

	// The child runs one hg-free test, so the run exercises the probe and
	// nothing else. -test.run keeps it from re-entering this test.
	run := func(t *testing.T, path string) (string, int) {
		t.Helper()
		cmd := exec.Command(self, "-test.run=TestAllCommandsHandled", "-test.v")
		cmd.Env = append(os.Environ(), "PATH="+path)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return string(out), exit.ExitCode()
		}
		t.Fatalf("running %s: %v\n%s", self, err, out)
		return "", 0
	}

	t.Run("present but not runnable fails the run", func(t *testing.T) {
		dir := t.TempDir()
		stub := filepath.Join(dir, "hg")
		if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 127\n"), 0755); err != nil {
			t.Fatal(err)
		}

		out, code := run(t, dir)
		if code == 0 {
			t.Errorf("a broken hg must fail the run, got exit 0\n%s", out)
		}
		if !strings.Contains(out, "will not run") {
			t.Errorf("output should name the unusable binary, got:\n%s", out)
		}
	})

	// exec.LookPath answers ErrNotFound for this case too, so a probe built on
	// LookPath alone files a broken install under "absent" and skips its way
	// to a green package — the outcome this whole file is here to rule out.
	t.Run("present but not executable fails the run", func(t *testing.T) {
		dir := t.TempDir()
		stub := filepath.Join(dir, "hg")
		if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := exec.LookPath(stub); err == nil {
			t.Skip("a file without execute bits is still executable here")
		}

		out, code := run(t, dir)
		if code == 0 {
			t.Errorf("an hg that cannot be executed must fail the run, got exit 0\n%s", out)
		}
		if !strings.Contains(out, "will not run") {
			t.Errorf("output should name the unusable binary, got:\n%s", out)
		}
	})

	t.Run("absent skips only the dependent tests", func(t *testing.T) {
		dir := t.TempDir() // empty: nothing named hg or chg is reachable
		if _, err := exec.LookPath("hg"); err == nil {
			// Confirms the doctored PATH is what makes hg unreachable, not
			// luck — if hg were findable here the case would prove nothing.
			t.Setenv("PATH", dir)
			if _, err := exec.LookPath("hg"); err == nil {
				t.Skip("hg is reachable even with an empty PATH; cannot test the absent case")
			}
		}

		out, code := run(t, dir)
		if code != 0 {
			t.Errorf("an absent hg is not a failure, got exit %d\n%s", code, out)
		}
		// The hg-free tests must still run: gating the whole package on hg is
		// what the old os.Exit(0) did, and it hid this coverage too.
		if !strings.Contains(out, "PASS: TestAllCommandsHandled") {
			t.Errorf("hg-free tests should still run without hg, got:\n%s", out)
		}
	})
}
