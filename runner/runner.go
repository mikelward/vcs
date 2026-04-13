// Package runner provides helpers for executing VCS subcommands.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// DryRun is true when the VCS_DRY_RUN environment variable is set.
// In dry-run mode, Run and Exec print the command they would execute
// to stderr and return nil without running anything.
//
// The variable is exported so tests and callers can toggle it directly.
var DryRun = os.Getenv("VCS_DRY_RUN") != ""

// Run executes the given command with args, inheriting stdin/stdout/stderr.
// It exits with the command's exit code.
//
// In dry-run mode, it prints the command to stderr and returns nil without
// executing.
func Run(name string, args ...string) error {
	if DryRun {
		PrintCommand(name, args)
		return nil
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Exec replaces the current process with the given command (unix exec).
// Falls back to Run on systems where exec isn't available.
//
// In dry-run mode, it prints the command to stderr and returns nil without
// executing.
func Exec(name string, args ...string) error {
	if DryRun {
		PrintCommand(name, args)
		return nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	argv := append([]string{path}, args...)
	return syscall.Exec(path, argv, os.Environ())
}

// ExitError returns the exit code from an error, or 1 if unknown.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

// FindCommand looks for a command on PATH and returns its full path,
// or empty string if not found.
func FindCommand(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// PrintCommand writes a shell-quoted representation of name and args to
// stderr, prefixed with "+ " so it resembles `set -x` output.
func PrintCommand(name string, args []string) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	fmt.Fprintln(os.Stderr, "+ "+strings.Join(parts, " "))
}

// shellQuote returns s quoted for shell consumption. Strings made up of
// safe characters are returned as-is; anything else is single-quoted.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if !safeRune(r) {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

func safeRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '/', '.', '_', '-', ':', '=', ',', '@', '+', '%':
		return true
	}
	return false
}
