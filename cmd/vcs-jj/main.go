// vcs-jj translates unified VCS subcommands into Jujutsu commands.
//
// Usage: vcs-jj <subcommand> [args...]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mikelward/vcs/runner"
	"github.com/mikelward/vcs/vcsdetect"
)

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
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: vcs-jj [-n|--dry-run] <subcommand> [args...]")
		os.Exit(1)
	}
	subcmd := args[0]
	subArgs := args[1:]

	err := dispatch(subcmd, subArgs)
	os.Exit(runner.ExitCode(err))
}

func isDryRunFlag(a string) bool {
	return a == "-n" || a == "--dry-run" || a == "--simulate"
}

func jj(args ...string) error {
	return runner.Run("jj", args...)
}

func capture(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func backend() string {
	dir, _ := os.Getwd()
	info, err := vcsdetect.Detect(dir)
	if err != nil {
		return ""
	}
	return info.Backend
}

func hosting() string {
	dir, _ := os.Getwd()
	info, err := vcsdetect.Detect(dir)
	if err != nil {
		return ""
	}
	return info.Hosting
}

func dispatch(subcmd string, args []string) error {
	switch subcmd {
	case "absorb":
		return jj(append([]string{"absorb"}, args...)...)
	case "add":
		return jj(append([]string{"file", "track"}, args...)...)
	case "addremove":
		// jj auto-tracks all files; nothing to do.
		return nil
	case "amend":
		return jj(append([]string{"squash"}, args...)...)
	case "annotate":
		return jj(append([]string{"file", "annotate"}, args...)...)
	case "at_tip":
		return jjAtTip()
	case "base":
		return jjBase(args)
	case "blame":
		return jj(append([]string{"file", "blame"}, args...)...)
	case "branch":
		// no current bookmark in jj
		return nil
	case "branches":
		return jj("bookmark", "list")
	case "change", "describe", "recommit":
		return jj(append([]string{"describe"}, args...)...)
	case "changed":
		return jj(append([]string{"diff", "--summary"}, args...)...)
	case "changelog":
		return jj(append([]string{"log", "--template=builtin_log_oneline"}, args...)...)
	case "changes", "diffs":
		return jj(append([]string{"diff"}, args...)...)
	case "checkout", "goto":
		return jj(append([]string{"new"}, args...)...)
	case "commit":
		return jj(append([]string{"commit"}, args...)...)
	case "commitforce":
		return jj(append([]string{"commit"}, args...)...)
	case "copy":
		return runner.Run("cp", args...)
	case "diffedit":
		return jj(append([]string{"diffedit"}, args...)...)
	case "diffstat":
		return jj(append([]string{"diff", "--stat"}, args...)...)
	case "drop":
		return jj(append([]string{"abandon"}, args...)...)
	case "evolve":
		fmt.Fprintln(os.Stderr, "jj automatically rebases descendants; nothing to do")
		return nil
	case "fastforward":
		return jjFastforward(args)
	case "fetchtime":
		return jjFetchtime()
	case "fix":
		return jj(append([]string{"fix"}, args...)...)
	case "graft":
		return jj(append([]string{"duplicate"}, args...)...)
	case "graph":
		return jjGraph(args)
	case "histedit":
		fmt.Fprintln(os.Stderr, "no interactive histedit in jj; use: jj squash, jj split, jj edit")
		return fmt.Errorf("not supported")
	case "ignore":
		return jjIgnore(args)
	case "incoming":
		return jj(append([]string{"op", "log"}, args...)...)
	case "lint":
		return jj(append([]string{"fix"}, args...)...)
	case "map":
		return jjMap(args)
	case "mergetool", "resolve":
		return jj(append([]string{"resolve"}, args...)...)
	case "move", "rename", "mv":
		return jjRename(args)
	case "next":
		return jj(append([]string{"next"}, args...)...)
	case "outgoing":
		return jjOutgoing(args)
	case "pending":
		return jj(append([]string{"--no-pager", "log", "-r", "mutable() ~ empty()"}, args...)...)
	case "pick":
		return jj(append([]string{"duplicate"}, args...)...)
	case "precommit":
		return jj(append([]string{"fix"}, args...)...)
	case "presubmit":
		return jjPresubmit(args)
	case "prev":
		return jj(append([]string{"prev"}, args...)...)
	case "pull":
		return jjPull(args)
	case "push":
		return jjPush(args)
	case "rebase":
		return jj(append([]string{"rebase"}, args...)...)
	case "remove", "rm":
		return jjRemove(args)
	case "restore":
		return jj(append([]string{"restore"}, args...)...)
	case "revert":
		return jjRevert(args)
	case "review", "upload", "uploadchain":
		return jjReview(args)
	case "reword":
		return jjReword(args)
	case "rootdir":
		return jj("workspace", "root")
	case "show":
		return jj(append([]string{"show"}, args...)...)
	case "split":
		return jj(append([]string{"split"}, args...)...)
	case "squash":
		return jj(append([]string{"squash"}, args...)...)
	case "status":
		return jjStatus(args)
	case "submit":
		return jjPush(args)
	case "submitforce":
		return jjPush(args)
	case "track":
		return jj(append([]string{"file", "track"}, args...)...)
	case "unamend":
		return jj(append([]string{"undo"}, args...)...)
	case "uncommit":
		return jj(append([]string{"squash", "--from", "@-", "--into", "@"}, args...)...)
	case "undo":
		return jj(append([]string{"undo"}, args...)...)
	case "unknown":
		return jj(append([]string{"file", "list", "--untracked"}, args...)...)
	case "untrack":
		return jj(append([]string{"untrack"}, args...)...)
	default:
		return fmt.Errorf("unknown jj subcommand: %s", subcmd)
	}
}

func jjAtTip() error {
	out, err := capture("jj", "--no-pager", "log", "--no-graph", "-r", `children(@) | (children(@-) ~ @)`, "--template", `"x"`)
	if err != nil {
		return err
	}
	if out != "" {
		return fmt.Errorf("not at tip")
	}
	return nil
}

func jjBase(args []string) error {
	tmpl := `if(self.contained_in("@"), if(description.first_line(), "@ " ++ change_id.shortest() ++ " " ++ description.first_line() ++ "\n"), "* " ++ change_id.shortest() ++ " " ++ description.first_line() ++ "\n")`
	return jj(append([]string{"--no-pager", "log", "--no-graph", "-r", "@|@-", "--template", tmpl}, args...)...)
}

func jjMap(args []string) error {
	if err := jjAtTip(); err == nil {
		return jjOutgoing(args)
	}
	return jjGraph(nil)
}

func jjOutgoing(args []string) error {
	return jj(append([]string{"--no-pager", "log", "--no-graph", "-r", "mutable() ~ empty() ~ ancestors(remote_bookmarks())", "--template", `change_id.shortest() ++ " " ++ description.first_line() ++ "\n"`}, args...)...)
}

func jjGraph(args []string) error {
	tmpl := `change_id.shortest() ++ " " ++ if(description, description.first_line(), "(no description set)") ++ if(bookmarks, " [" ++ bookmarks.join(", ") ++ "]", "") ++ "\n"`
	if len(args) == 0 {
		return jj("log", "--template", tmpl, "-r", "mutable() ~ empty() ~ ancestors(remote_bookmarks())")
	}
	return jj(append([]string{"log", "--template", tmpl}, args...)...)
}

func jjFastforward(args []string) error {
	if backend() == "git" {
		return jj(append([]string{"git", "fetch"}, args...)...)
	}
	return jj("piper", "pull")
}

func jjFetchtime() error {
	root, err := capture("jj", "workspace", "root")
	if err != nil {
		return err
	}
	fetchHead := root + "/.jj/repo/store/git/FETCH_HEAD"
	fi, err := os.Stat(fetchHead)
	if err != nil {
		return err
	}
	fmt.Println(fi.ModTime().Unix())
	return nil
}

func jjIgnore(args []string) error {
	root, err := capture("jj", "workspace", "root")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(root+"/.gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, a := range args {
		fmt.Fprintln(f, a)
	}
	return nil
}

func jjPresubmit(args []string) error {
	if backend() == "git" {
		fmt.Fprintln(os.Stderr, "no presubmit for git-backed repos; run tests locally")
		return fmt.Errorf("not supported")
	}
	return jj(append([]string{"piper", "presubmit"}, args...)...)
}

func jjPull(args []string) error {
	if backend() == "git" {
		return jj(append([]string{"git", "fetch"}, args...)...)
	}
	return jj(append([]string{"sync"}, args...)...)
}

func jjPush(args []string) error {
	if backend() == "git" {
		return jj(append([]string{"git", "push"}, args...)...)
	}
	return jj(append([]string{"upload"}, args...)...)
}

func jjRemove(args []string) error {
	if err := runner.Run("rm", args...); err != nil {
		return err
	}
	return jj(append([]string{"file", "untrack"}, args...)...)
}

func jjRename(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: rename <source> <dest>")
	}
	return runner.Run("mv", args[0], args[1])
}

// jjRevert dispatches the "revert" subcommand. With no args, it undoes
// working-copy changes (like "git reset --hard"). If any flag is present
// (e.g. -r to pick a revision), it passes through to "jj revert", which
// creates a revert commit. Otherwise all args are treated as file paths
// and passed to "jj restore".
func jjRevert(args []string) error {
	if len(args) == 0 {
		return jj("restore")
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return jj(append([]string{"revert"}, args...)...)
		}
	}
	return jj(append([]string{"restore"}, args...)...)
}

func jjReview(args []string) error {
	var reviewFlags []string
	var pushFlags []string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-r", "-m", "--reviewer":
			if i+1 < len(args) {
				reviewFlags = append(reviewFlags, "-r", args[i+1])
				i += 2
				continue
			}
		default:
			if strings.HasPrefix(args[i], "--reviewer=") {
				reviewFlags = append(reviewFlags, "-r", strings.TrimPrefix(args[i], "--reviewer="))
			} else {
				pushFlags = append(pushFlags, args[i])
			}
		}
		i++
	}

	if backend() == "git" {
		pushArgs := append([]string{"git", "push"}, pushFlags...)
		if err := jj(pushArgs...); err != nil {
			return err
		}
		if hosting() == "github" && runner.FindCommand("gh") != "" {
			// Get current bookmark for --head flag.
			bookmark, _ := capture("jj", "bookmark", "list", "-r", "@")
			var head string
			if bookmark != "" {
				parts := strings.SplitN(bookmark, ":", 2)
				head = strings.TrimSpace(parts[0])
			}

			// Check if PR exists.
			ghViewArgs := []string{"pr", "view", "--json", "url", "-q", ".url"}
			if head != "" {
				ghViewArgs = []string{"pr", "view", head, "--json", "url", "-q", ".url"}
			}
			if err := runner.Run("gh", ghViewArgs...); err != nil {
				// Create PR.
				createArgs := []string{"pr", "create", "--fill"}
				if head != "" {
					createArgs = append(createArgs, "--head", head)
				}
				if len(reviewFlags) == 0 {
					createArgs = append(createArgs, "--draft")
				} else {
					createArgs = append(createArgs, reviewFlags...)
				}
				return runner.Run("gh", createArgs...)
			}
		}
		return nil
	}
	return jj(append([]string{"upload"}, pushFlags...)...)
}

func jjReword(args []string) error {
	if len(args) == 0 {
		return jj("describe")
	}
	return jj(append([]string{"describe", "-m"}, args...)...)
}

func jjStatus(args []string) error {
	desc, err := capture("jj", "log", "--no-graph", "-r", "@", "-T", "description")
	if err != nil {
		return err
	}
	if desc == "" {
		return jj(append([]string{"diff", "--summary"}, args...)...)
	}
	return nil
}
