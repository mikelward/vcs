// Package jjsync reports when a jj workspace last pulled upstream state.
//
// On a fast-moving non-git backend (e.g. piper) the trunk advances
// constantly, so "is the checkout behind?" can't be answered by a ref
// comparison — it would always read behind. The useful signal is instead
// how long it has been since the last sync. That time isn't recorded in a
// single marker file (every jj command snapshots the working copy, so file
// mtimes track activity, not syncs), so it is read from the operation log.
package jjsync

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// LastSync returns the time of the most recent operation that pulled upstream
// state into the workspace (fetch/sync/pull/import) and whether any such
// operation was found.
func LastSync(rootDir string) (time.Time, bool) {
	// %s renders the operation end time as Unix seconds, which is
	// unambiguous to parse (no timezone handling). op log is newest-first,
	// so the first matching line is the most recent sync.
	//
	// --ignore-working-copy keeps this read-only: jj snapshots the working
	// copy on most commands (including op log) by default, so without it a
	// per-prompt staleness check would create a snapshot operation every
	// time and mutate the repo just to read metadata.
	//
	// No --limit: every jj command snapshots the working copy, creating an
	// operation, so on an active workspace the last sync can sit behind
	// hundreds of intervening snapshot ops. A fixed cap would make a
	// genuinely-stale repo look like it never synced (found=false), wrongly
	// silencing the behind nag. The log is local metadata, so scanning it is
	// cheap relative to the other per-prompt jj calls.
	out, err := exec.Command("jj", "-R", rootDir, "op", "log", "--no-graph",
		"--ignore-working-copy",
		"-T", `time.end().format("%s") ++ " " ++ description ++ "\n"`).Output()
	if err != nil {
		return time.Time{}, false
	}
	return Parse(string(out))
}

// Parse scans `jj op log` output (one "<unix-seconds> <description>" line per
// operation, newest first) for the first sync-like operation and returns its
// time.
func Parse(out string) (time.Time, bool) {
	for _, line := range strings.Split(out, "\n") {
		secs, desc, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		desc = strings.ToLower(desc)
		if !(strings.Contains(desc, "fetch") || strings.Contains(desc, "sync") ||
			strings.Contains(desc, "pull") || strings.Contains(desc, "import")) {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(secs), 10, 64)
		if err != nil {
			continue
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}
