package main

import (
	"strings"
	"testing"

	vcs "github.com/mikelward/vcs"
	"github.com/mikelward/vcs/runner"
)

func TestAllCommandsHandled(t *testing.T) {
	old := runner.DryRun
	runner.DryRun = true
	t.Cleanup(func() { runner.DryRun = old })

	for _, cmd := range vcs.Commands {
		_, err := captureIO(t, func() error { return dispatch(cmd, nil) })
		if err != nil && strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "subcommand") {
			t.Errorf("command %q not handled by vcs-p4 dispatch", cmd)
		}
	}
}
