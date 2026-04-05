package main

import (
	"fmt"

	vcs "github.com/mikelward/vcs"
)

func listCommands() {
	for _, cmd := range vcs.Commands {
		fmt.Println(cmd)
	}
}
