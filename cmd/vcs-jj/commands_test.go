package main

import (
	"strings"
	"testing"

	vcs "github.com/mikelward/vcs"
)

func TestAllCommandsHandled(t *testing.T) {
	for _, cmd := range vcs.Commands {
		err := dispatch(cmd, nil)
		if err != nil && strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "subcommand") {
			t.Errorf("command %q is not handled by vcs-jj dispatch", cmd)
		}
	}
}
