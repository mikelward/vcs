// Package version holds build metadata for the vcs binaries, read from
// the VCS stamps Go embeds via debug.ReadBuildInfo.
package version

import (
	"fmt"
	"runtime/debug"
)

// Info is the set of build metadata fields we surface.
type Info struct {
	Version string // module version, e.g. "(devel)" or "v1.2.3"
	Commit  string // git commit SHA, with "-dirty" suffix if the tree was modified
	Date    string // commit timestamp in RFC 3339
}

// Read returns build metadata from the embedded Go module VCS stamps.
// Fields default to "unknown" when a binary is built outside a VCS checkout.
func Read() Info {
	info := Info{Version: "unknown", Commit: "unknown", Date: "unknown"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	if bi.Main.Version != "" {
		info.Version = bi.Main.Version
	}
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if s.Value != "" {
				info.Commit = s.Value
			}
		case "vcs.time":
			if s.Value != "" {
				info.Date = s.Value
			}
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if modified && info.Commit != "unknown" {
		info.Commit += "-dirty"
	}
	return info
}

// String returns a one-line summary suitable for "--version" output.
func String(name string) string { return Read().Line(name) }

// Multiline returns a multi-line "name: value" summary suitable for a
// dedicated `version` subcommand.
func Multiline(name string) string { return Read().Block(name) }

// Line formats the receiver as a one-line summary.
func (i Info) Line(name string) string {
	return fmt.Sprintf("%s %s (commit %s, built %s)", name, i.Version, i.Commit, i.Date)
}

// Block formats the receiver as a multi-line "name: value" summary.
func (i Info) Block(name string) string {
	return fmt.Sprintf("%s\nversion: %s\ncommit:  %s\nbuilt:   %s", name, i.Version, i.Commit, i.Date)
}
