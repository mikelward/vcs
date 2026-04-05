// Package runner provides helpers for executing VCS subcommands.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Run executes the given command with args, inheriting stdin/stdout/stderr.
// It exits with the command's exit code.
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Exec replaces the current process with the given command (unix exec).
// Falls back to Run on systems where exec isn't available.
func Exec(name string, args ...string) error {
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
