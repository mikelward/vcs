// Package version holds build-time metadata shared by the vcs binaries.
//
// Values are populated via -ldflags "-X github.com/mikelward/vcs/version.Version=..."
// at build time (see the Makefile). When built without ldflags (e.g. "go build"
// or "go install") the defaults below apply.
package version

import (
	"fmt"
	"runtime/debug"
)

var (
	// Version is the release version, e.g. "v1.2.3" or "v1.2.3-4-gabcdef-dirty".
	Version = "dev"

	// Commit is the git commit hash the binary was built from.
	Commit = "unknown"

	// BuildDate is an RFC 3339 UTC timestamp of when the binary was built.
	BuildDate = "unknown"
)

// String returns a one-line summary suitable for "vcs --version" output.
func String(name string) string {
	return fmt.Sprintf("%s %s (commit %s, built %s)", name, info().Version, info().Commit, info().BuildDate)
}

// Multiline returns a multi-line "name: value" summary suitable for a
// dedicated `version` subcommand.
func Multiline(name string) string {
	i := info()
	return fmt.Sprintf("%s\nversion: %s\ncommit:  %s\nbuilt:   %s", name, i.Version, i.Commit, i.BuildDate)
}

type versionInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// info returns the build metadata, falling back to debug.ReadBuildInfo when
// ldflags were not supplied (e.g. "go install" from a checkout).
func info() versionInfo {
	v := versionInfo{Version: Version, Commit: Commit, BuildDate: BuildDate}
	if v.Version != "dev" && v.Commit != "unknown" && v.BuildDate != "unknown" {
		return v
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	if v.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		v.Version = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if v.Commit == "unknown" && s.Value != "" {
				v.Commit = s.Value
			}
		case "vcs.time":
			if v.BuildDate == "unknown" && s.Value != "" {
				v.BuildDate = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" && v.Commit != "unknown" {
				v.Commit += "-dirty"
			}
		}
	}
	return v
}
