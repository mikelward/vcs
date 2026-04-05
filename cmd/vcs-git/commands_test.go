package main

import (
	"strings"
	"testing"

	vcs "github.com/mikelward/vcs"
)

func TestAllCommandsHandled(t *testing.T) {
	for _, cmd := range vcs.Commands {
		err := dispatch(cmd, nil)
		// Commands will fail because git isn't set up in the test env,
		// but they must NOT return "unknown ... subcommand".
		if err != nil && strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "subcommand") {
			t.Errorf("command %q is not handled by vcs-git dispatch", cmd)
		}
	}
}
