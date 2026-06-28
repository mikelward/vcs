// vcs-p4 translates unified VCS subcommands into Perforce (p4/g4) commands.
//
// Usage: vcs-p4 <subcommand> [args...]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mikelward/vcs/runner"
	"github.com/mikelward/vcs/version"
)

// p4Cmd holds the resolved binary name ("g4" or "p4").
var p4Cmd string

func main() {
	args := os.Args[1:]
	// Consume leading dry-run flags before the subcommand.
	for len(args) > 0 && isDryRunFlag(args[0]) {
		runner.DryRun = true
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "--list-commands" {
		listCommands()
		return
	}
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-V") {
		fmt.Println(version.String("vcs-p4"))
		return
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: vcs-p4 [-n|--dry-run] <subcommand> [args...]")
		os.Exit(1)
	}
	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == "version" {
		fmt.Println(version.Multiline("vcs-p4"))
		return
	}

	p4Cmd = findP4()

	err := dispatch(subcmd, subArgs)
	runner.PrintError("vcs-p4", err)
	os.Exit(runner.ExitCode(err))
}

func isDryRunFlag(a string) bool {
	return a == "-n" || a == "--dry-run" || a == "--simulate"
}

// findP4 returns "g4" if available, otherwise "p4".
func findP4() string {
	if p := runner.FindCommand("g4"); p != "" {
		return "g4"
	}
	return "p4"
}

func p4(args ...string) error {
	return runner.Run(p4Cmd, args...)
}

func capture(name string, args ...string) (string, error) {
	if runner.DryRun {
		return "", nil
	}
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func dispatch(subcmd string, args []string) error {
	switch subcmd {
	case "absorb":
		return fmt.Errorf("absorb not supported in Perforce")
	case "add", "track":
		return p4(append([]string{"add"}, args...)...)
	case "addremove":
		return p4(append([]string{"reconcile"}, args...)...)
	case "amend", "recommit":
		return p4(append([]string{"change"}, args...)...)
	case "annotate", "blame":
		return p4(append([]string{"annotate"}, args...)...)
	case "at_tip":
		// Check if we are synced to the latest changelist.
		return p4AtTip()
	case "base":
		return p4Base(args)
	case "branch":
		// Perforce has no branch concept that maps cleanly; the old
		// client-name mapping wasn't a branch. Refuse rather than mislead.
		// TODO: try reporting the current stream (e.g. parse %Stream% from
		// `p4 client -o`) once validated against a real depot.
		return fmt.Errorf("branch not supported in Perforce")
	case "branches":
		// Refuse rather than mislead; `p4 clients` listed workspaces, not
		// branches.
		// TODO: try `p4 streams` here — streams are Perforce's lines of
		// development, but may return nothing on classic (non-stream) depots,
		// so leave unsupported until validated against a real depot.
		return fmt.Errorf("branches not supported in Perforce")
	case "change", "describe", "reword":
		return p4(append([]string{"change"}, args...)...)
	case "changed":
		return p4(append([]string{"opened"}, args...)...)
	case "changelog":
		return p4(append([]string{"changes"}, args...)...)
	case "changes", "diffs":
		return p4(append([]string{"diff"}, args...)...)
	case "checkout", "goto", "pull":
		return p4(append([]string{"sync"}, args...)...)
	case "commit", "commitforce", "push", "submit", "submitforce":
		return p4(append([]string{"submit"}, args...)...)
	case "copy":
		return p4Copy(args)
	case "count":
		return p4Count()
	case "diffedit":
		return fmt.Errorf("diffedit not supported in Perforce")
	case "diffstat":
		return p4(append([]string{"diff", "-ds"}, args...)...)
	case "drop":
		// `drop` removes a commit from history (git rebase --onto, hg prune,
		// jj abandon). Perforce can't rewrite submitted history, and the old
		// `p4 revert` mapping silently discarded pending changes instead —
		// wrong and destructive. Refuse; use `vcs revert`/`vcs restore` to
		// discard workspace changes explicitly.
		return fmt.Errorf("drop not supported in Perforce; use 'vcs revert' to discard pending changes")
	case "evolve":
		return fmt.Errorf("evolve not supported in Perforce")
	case "fastforward":
		// Perforce has no fast-forward-only update. Plain `p4 sync` can
		// schedule resolves on open files and clobber workspace changes,
		// which violates fastforward's "fast, never risk merge conflicts"
		// contract. Refuse rather than do something risky; use `pull`/`sync`
		// (mapped to `p4 sync`) explicitly if that's what you want.
		return fmt.Errorf("fastforward not supported in Perforce; use 'vcs pull' to sync")
	case "fetchtime":
		return p4Fetchtime()
	case "fix":
		return p4Fix(args)
	case "graft", "pick":
		return p4(append([]string{"integrate"}, args...)...)
	case "graph":
		return p4(append([]string{"changes"}, args...)...)
	case "histedit":
		return fmt.Errorf("histedit not supported in Perforce")
	case "ignore":
		return p4Ignore(args)
	case "incoming", "unpulled":
		return p4(append([]string{"sync", "-n"}, args...)...)
	case "lint":
		return p4Fix(args)
	case "map":
		return p4Map(args)
	case "mergetool", "resolve":
		return p4(append([]string{"resolve"}, args...)...)
	case "move", "rename":
		return p4(append([]string{"move"}, args...)...)
	case "next", "prev":
		return fmt.Errorf("%s not supported in Perforce", subcmd)
	case "outgoing", "unpushed", "unmerged":
		return p4(append([]string{"opened"}, args...)...)
	case "pending":
		return p4Pending()
	case "precommit", "presubmit":
		// Perforce handles these via server-side submit triggers, return nil
		return nil
	case "rebase":
		return fmt.Errorf("rebase not supported in Perforce")
	case "remove", "rm":
		return p4(append([]string{"delete"}, args...)...)
	case "restore":
		return p4(append([]string{"revert"}, args...)...)
	case "revert":
		return p4Revert(args)
	case "review", "upload", "uploadchain":
		return p4(append([]string{"change"}, args...)...)
	case "rootdir":
		return p4PrintRoot()
	case "show":
		return p4(append([]string{"describe"}, args...)...)
	case "split", "squash":
		return fmt.Errorf("%s not supported in Perforce", subcmd)
	case "status":
		return p4(append([]string{"status"}, args...)...)
	case "unamend", "uncommit", "undo":
		return fmt.Errorf("%s not supported in Perforce", subcmd)
	case "unknown":
		return p4("status", "-a")
	case "untrack":
		return p4(append([]string{"revert", "-k"}, args...)...)
	default:
		return fmt.Errorf("unknown Perforce subcommand: %s", subcmd)
	}
}

func p4AtTip() error {
	// Syncing with preview flag and -m 1 to cheaply check whether any updates exist.
	// `p4 sync -n -m 1` prints either a file line to sync or an "...up-to-date." message.
	// Match the suffix (rather than substring) so file paths containing "up-to-date"
	// don't false-positive.
	out, err := capture(p4Cmd, "sync", "-n", "-m", "1")
	if err != nil {
		return err
	}
	if out == "" || strings.HasSuffix(strings.TrimSpace(out), "up-to-date.") {
		return nil
	}
	return fmt.Errorf("not at tip (updates available on server)")
}

func p4Pending() error {
	client := "$P4CLIENT"
	if !runner.DryRun {
		c, err := captureP4Client()
		if err != nil {
			return err
		}
		client = c
	}
	return p4("changes", "-s", "pending", "-c", client)
}

func p4Revert(args []string) error {
	// Bare `revert` reverts every opened file in the workspace, matching
	// `git reset --hard` / `hg revert --all`. `p4 revert` requires a
	// filespec, and `...` is relative to cwd — use `//<client>/...` so
	// running from a subdirectory still covers the whole client. `-Si`
	// also reverts an open stream spec (default p4 revert leaves it).
	if len(args) != 0 {
		return p4(append([]string{"revert"}, args...)...)
	}
	client := "$P4CLIENT"
	if !runner.DryRun {
		c, err := captureP4Client()
		if err != nil {
			return err
		}
		client = c
	}
	return p4("revert", "-Si", fmt.Sprintf("//%s/...", client))
}

func p4Base(args []string) error {
	// Describe the latest submitted change affecting this client's path.
	out, err := capture(p4Cmd, "changes", "-m", "1", "#have")
	if err != nil {
		return err
	}
	if out == "" {
		return fmt.Errorf("no base changes found")
	}
	parts := strings.Fields(out)
	if len(parts) < 2 {
		return fmt.Errorf("invalid changes output: %q", out)
	}
	return p4(append([]string{"describe", "-s", parts[1]}, args...)...)
}

func captureP4Client() (string, error) {
	out, err := capture(p4Cmd, "info")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Client name:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Client name:")), nil
		}
	}
	return "", fmt.Errorf("could not find client name in info")
}

func p4Copy(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("copy requires source and destination")
	}
	// Try standard integrate first, then copy files.
	dest := args[len(args)-1]
	srcs := args[:len(args)-1]
	for _, src := range srcs {
		if err := p4("integrate", src, dest); err != nil {
			// Fallback: system copy then add
			if err := runner.Run("cp", src, dest); err != nil {
				return err
			}
			if err := p4("add", dest); err != nil {
				return err
			}
		}
	}
	return nil
}

func p4Count() error {
	// Count submitted changes affecting the client path.
	out, err := capture(p4Cmd, "changes", "#have")
	if err != nil {
		return err
	}
	lines := strings.Split(out, "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	fmt.Println(count)
	return nil
}

func p4Fetchtime() error {
	// Best-effort: modified time of the client spec or .p4config.
	root, err := captureP4Root()
	if err != nil {
		return err
	}
	markers := []string{".p4config", ".citc"}
	for _, marker := range markers {
		path := filepath.Join(root, marker)
		if fi, err := os.Stat(path); err == nil {
			fmt.Println(fi.ModTime().Unix())
			return nil
		}
	}
	return fmt.Errorf("could not determine fetch time")
}

func p4Fix(args []string) error {
	// Run custom fix scripts if present, or formatters.
	root, err := captureP4Root()
	if err == nil {
		for _, hookDir := range []string{".p4/hooks/fix", ".g4/hooks/fix"} {
			script := filepath.Join(root, hookDir)
			if fi, err := os.Stat(script); err == nil && fi.Mode()&0111 != 0 {
				return runner.Run(script, args...)
			}
		}
	}
	return nil
}

func p4Ignore(args []string) error {
	root, err := captureP4Root()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, ".p4ignore"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, a := range args {
		fmt.Fprintln(f, a)
	}
	return nil
}

func p4Map(args []string) error {
	// Map shows opened files and changes.
	if err := p4("opened"); err != nil {
		return err
	}
	return p4("changes", "-m", "10")
}

func captureP4Root() (string, error) {
	out, err := capture(p4Cmd, "info")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Client root:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Client root:")), nil
		}
	}
	// Fallback to searching upwards for .citc or .p4config.
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, marker := range []string{".citc", ".p4config"} {
			if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
				return d, nil
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("could not find client root")
}

func p4PrintRoot() error {
	root, err := captureP4Root()
	if err != nil {
		return err
	}
	fmt.Println(root)
	return nil
}
