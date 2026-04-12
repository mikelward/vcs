// vcs-hg translates unified VCS subcommands into Mercurial commands.
//
// Usage: vcs-hg [--hg-path=PATH] <subcommand> [args...]
//
// By default, uses "chg" if available on PATH, falling back to "hg".
// Use --hg-path to override (useful for callers that cache the lookup).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mikelward/vcs/runner"
)

// hgCmd is the resolved path to hg or chg.
var hgCmd string

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--list-commands" {
		listCommands()
		return
	}

	args := os.Args[1:]

	// Check for --hg-path flag before subcommand.
	if len(args) > 0 && strings.HasPrefix(args[0], "--hg-path=") {
		hgCmd = strings.TrimPrefix(args[0], "--hg-path=")
		args = args[1:]
	} else if len(args) > 1 && args[0] == "--hg-path" {
		hgCmd = args[1]
		args = args[2:]
	} else {
		hgCmd = findHg()
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: vcs-hg [--hg-path=PATH] <subcommand> [args...]")
		os.Exit(1)
	}
	subcmd := args[0]
	args = args[1:]

	err := dispatch(subcmd, args)
	os.Exit(runner.ExitCode(err))
}

// findHg returns "chg" if available, otherwise "hg".
// Prints a warning to stderr if chg is not found, since it is
// significantly faster than plain hg.
func findHg() string {
	if p := runner.FindCommand("chg"); p != "" {
		return p
	}
	fmt.Fprintln(os.Stderr, "vcs-hg: warning: chg not found, falling back to hg (expect slower performance)")
	return "hg"
}

func hg(args ...string) error {
	return runner.Run(hgCmd, args...)
}

func capture(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func dispatch(subcmd string, args []string) error {
	switch subcmd {
	case "absorb":
		return hg(append([]string{"--config", "extensions.absorb=", "absorb", "--apply-changes"}, args...)...)
	case "add":
		return hg(append([]string{"add"}, args...)...)
	case "addremove":
		return hg(append([]string{"addremove"}, args...)...)
	case "amend":
		return hg(append([]string{"amend"}, args...)...)
	case "annotate":
		return hg(append([]string{"annotate"}, args...)...)
	case "at_tip":
		return hgAtTip()
	case "base":
		return hg(append([]string{"--pager", "never", "log", "-r", ".", "--template", "{onelinesummary}\\n"}, args...)...)
	case "blame":
		return hg(append([]string{"blame"}, args...)...)
	case "branch":
		return hg("branch")
	case "branches":
		return hg("branches")
	case "change", "describe":
		return hgChange(args)
	case "changed":
		return hgChanged(args)
	case "changelog":
		return hg(append([]string{"log", "--template", "{onelinesummary}\\n"}, args...)...)
	case "changes":
		return hg(append([]string{"diff"}, args...)...)
	case "checkout", "goto":
		if subcmd == "goto" {
			return hg(append([]string{"update"}, args...)...)
		}
		return hg(append([]string{"checkout"}, args...)...)
	case "commit":
		return hg(append([]string{"commit"}, args...)...)
	case "commitforce":
		return hg(append([]string{"--config", "hooks.precommit=", "--config", "hooks.pre-commit=", "commit"}, args...)...)
	case "copy":
		return hg(append([]string{"copy"}, args...)...)
	case "diffedit":
		return hg(append([]string{"histedit"}, args...)...)
	case "diffs":
		return hg(append([]string{"diff"}, args...)...)
	case "diffstat":
		return hg(append([]string{"diff", "--stat"}, args...)...)
	case "drop":
		return hg(append([]string{"prune"}, args...)...)
	case "evolve":
		return hg(append([]string{"evolve"}, args...)...)
	case "fastforward":
		return hgFastforward()
	case "fetchtime":
		return hgFetchtime()
	case "fix":
		return hg(append([]string{"fix"}, args...)...)
	case "graft":
		return hg(append([]string{"graft"}, args...)...)
	case "graph":
		return hgGraph(args)
	case "histedit":
		return hg("histedit")
	case "ignore":
		return hgIgnore(args)
	case "incoming":
		return hg(append([]string{"incoming", "--template", "{onelinesummary}\\n"}, args...)...)
	case "lint":
		return hg(append([]string{"lint"}, args...)...)
	case "map":
		return hgMap(args)
	case "mergetool", "resolve":
		return hg(append([]string{"resolve"}, args...)...)
	case "move", "rename", "mv":
		return hg(append([]string{"rename"}, args...)...)
	case "next":
		return hg(append([]string{"update", "-r", "min(children(.))"}, args...)...)
	case "outgoing":
		return hgOutgoing(args)
	case "pending":
		return hg("--pager", "never", "status")
	case "pick":
		return hg(append([]string{"graft"}, args...)...)
	case "precommit":
		return hg(append([]string{"precommit"}, args...)...)
	case "presubmit":
		return hg(append([]string{"presubmit"}, args...)...)
	case "prev":
		return hg(append([]string{"update", "-r", ".^"}, args...)...)
	case "pull":
		return hg(append([]string{"pull", "--update", "--rebase"}, args...)...)
	case "push":
		return hg(append([]string{"push"}, args...)...)
	case "rebase":
		return hg(append([]string{"rebase"}, args...)...)
	case "recommit":
		return hg(append([]string{"commit", "--amend"}, args...)...)
	case "remove", "rm":
		return hg(append([]string{"remove"}, args...)...)
	case "restore", "revert":
		return hg(append([]string{"revert"}, args...)...)
	case "review":
		fmt.Fprintln(os.Stderr, "hg review not supported")
		return fmt.Errorf("not supported")
	case "reword":
		return hgReword(args)
	case "rootdir":
		return hg("root")
	case "show":
		return hg(append([]string{"export"}, args...)...)
	case "split":
		return hg(append([]string{"split"}, args...)...)
	case "squash":
		return hg(append([]string{"fold"}, args...)...)
	case "status":
		return hg(append([]string{"status"}, args...)...)
	case "submit":
		return hg("submit")
	case "submitforce":
		return hg("--config", "hooks.preoutgoing=", "--config", "hooks.pre-push=", "submit")
	case "track":
		return hg(append([]string{"add"}, args...)...)
	case "unamend":
		return hg(append([]string{"unamend"}, args...)...)
	case "uncommit":
		return hg(append([]string{"--config", "extensions.uncommit=", "uncommit"}, args...)...)
	case "undo":
		return hg(append([]string{"undo"}, args...)...)
	case "unknown":
		return hg("status", "--unknown", "--deleted")
	case "untrack":
		return hg(append([]string{"forget"}, args...)...)
	case "upload":
		if len(args) == 0 {
			return hg("push", "-r", ".")
		}
		return hg(append([]string{"push"}, args...)...)
	case "uploadchain":
		return hg(append([]string{"uploadchain"}, args...)...)
	default:
		return fmt.Errorf("unknown hg subcommand: %s", subcmd)
	}
}

func hgAtTip() error {
	out, err := capture(hgCmd, "--pager", "never", "log", "-r", ". and last(heads(branch(.)))", "--template", "x")
	if err != nil {
		return err
	}
	if out == "" {
		return fmt.Errorf("not at tip")
	}
	return nil
}

func hgMap(args []string) error {
	if err := hgAtTip(); err == nil {
		return hgOutgoing(args)
	}
	return hgGraph(nil)
}

func hgOutgoing(args []string) error {
	return hg(append([]string{"--pager", "never", "--quiet", "log", "-r", "draft() and not obsolete()", "--template", "{onelinesummary}\\n"}, args...)...)
}

func hgChange(args []string) error {
	if len(args) == 0 {
		return hg("commit", "--amend", "-e")
	}
	return hg(append([]string{"commit", "--amend"}, args...)...)
}

func hgChanged(args []string) error {
	if len(args) == 0 {
		return hg("status", "--no-status")
	}
	return hg(append([]string{"log", "--template", "{files}\\n"}, args...)...)
}

func hgReword(args []string) error {
	if len(args) == 0 {
		return hg("commit", "--amend", "-e")
	}
	return hg(append([]string{"commit", "--amend"}, args...)...)
}

func hgFastforward() error {
	if err := hg("sync", "--tool=internal:fail"); err != nil {
		_ = hg("rebase", "--abort")
		return err
	}
	return nil
}

func hgFetchtime() error {
	root, err := capture(hgCmd, "root")
	if err != nil {
		return err
	}
	changelog := root + "/.hg/store/00changelog.i"
	fi, err := os.Stat(changelog)
	if err != nil {
		return err
	}
	fmt.Println(fi.ModTime().Unix())
	return nil
}

func hgGraph(args []string) error {
	if len(args) == 0 {
		return hg("--pager", "never", "log", "--graph", "--template", "oneline", "-r", "draft() and not obsolete()")
	}
	return hg(append([]string{"--pager", "never", "log", "--graph", "--template", "oneline"}, args...)...)
}

func hgIgnore(args []string) error {
	root, err := capture(hgCmd, "root")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(root+"/.hgignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, a := range args {
		fmt.Fprintln(f, a)
	}
	return nil
}
