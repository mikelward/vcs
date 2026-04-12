// vcs-git translates unified VCS subcommands into git commands.
//
// Usage: vcs-git <subcommand> [args...]
//
// Most subcommands map directly to git commands. Some (like amend, commit)
// have special argument handling to match the bash implementation's behavior.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mikelward/vcs/runner"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--list-commands" {
		listCommands()
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: vcs-git <subcommand> [args...]")
		os.Exit(1)
	}
	subcmd := os.Args[1]
	args := os.Args[2:]

	err := dispatch(subcmd, args)
	os.Exit(runner.ExitCode(err))
}

func dispatch(subcmd string, args []string) error {
	switch subcmd {
	case "absorb":
		return git("absorb", args...)
	case "add":
		return git("add", append([]string{"--intent-to-add"}, args...)...)
	case "addremove":
		return git("add", append([]string{"--all"}, args...)...)
	case "amend":
		return gitAmend(args)
	case "annotate", "blame":
		return git("blame", args...)
	case "at_tip":
		return gitAtTip()
	case "base":
		return gitBase(args)
	case "branch":
		return gitBranch()
	case "branches":
		return git("branch", args...)
	case "change":
		return git("commit", append([]string{"--amend"}, args...)...)
	case "changed":
		return git("diff", append([]string{"--name-only"}, args...)...)
	case "changelog":
		return git("log", append([]string{"--oneline"}, args...)...)
	case "changes":
		return git("diff", args...)
	case "checkout", "goto":
		return git("checkout", args...)
	case "commit":
		return gitCommit(args)
	case "commitforce":
		return gitCommit(append([]string{"--no-verify"}, args...))
	case "copy":
		return gitCopy(args)
	case "describe":
		return git("commit", append([]string{"--amend", "--only", "--allow-empty"}, args...)...)
	case "diffedit":
		return git("rebase", append([]string{"--interactive"}, args...)...)
	case "diffs":
		return git("diff", args...)
	case "diffstat":
		return git("diff", append([]string{"--stat"}, args...)...)
	case "drop":
		if len(args) < 1 {
			return fmt.Errorf("drop requires a commit argument")
		}
		return git("rebase", "--onto", args[0]+"~", args[0])
	case "evolve":
		fmt.Fprintln(os.Stderr, "no automatic evolve in git; use: git rebase --onto <new> <old> <branch>")
		return fmt.Errorf("not supported")
	case "fastforward":
		return git("pull", "--ff-only")
	case "fetchtime":
		return gitFetchtime()
	case "fix":
		return git("fix", args...)
	case "graft":
		return git("cherry-pick", args...)
	case "graph":
		return gitGraph(args)
	case "histedit":
		return git("rebase", "--interactive")
	case "ignore":
		return gitIgnore(args)
	case "incoming":
		return git("log", append([]string{"--oneline", "HEAD..@{upstream}"}, args...)...)
	case "lint":
		return git("lint", args...)
	case "map":
		return gitMap(args)
	case "mergetool", "resolve":
		return git("mergetool", args...)
	case "move", "rename":
		return git("mv", args...)
	case "next":
		return gitNext()
	case "outgoing":
		return runner.Run("git", append([]string{"--no-pager", "log", "--oneline", "@{upstream}..HEAD"}, args...)...)
	case "pending":
		return gitPending(args)
	case "pick":
		return git("cherry-pick", args...)
	case "precommit":
		return gitHook("pre-commit", args)
	case "presubmit":
		return gitHook("pre-push", args)
	case "prev":
		return git("checkout", "HEAD~")
	case "pull":
		return git("pull", append([]string{"--rebase"}, args...)...)
	case "push":
		return git("push", args...)
	case "rebase":
		return git("rebase", args...)
	case "recommit":
		return git("amend", args...)
	case "remove", "rm":
		return git("rm", args...)
	case "restore":
		return git("checkout", append([]string{"--"}, args...)...)
	case "revert":
		return gitRevert(args)
	case "review", "upload", "uploadchain":
		return gitReview(args)
	case "reword":
		return git("commit", "--amend", "--only", "--allow-empty")
	case "rootdir":
		return git("rootdir")
	case "show":
		return git("show", args...)
	case "split":
		return git("rebase", append([]string{"-i"}, args...)...)
	case "squash":
		return git("merge", append([]string{"--squash"}, args...)...)
	case "status":
		return git("status", append([]string{"--short", "--untracked-files=all"}, args...)...)
	case "submit":
		return git("push", args...)
	case "submitforce":
		return git("push", append([]string{"--no-verify"}, args...)...)
	case "track":
		return git("add", append([]string{"--intent-to-add"}, args...)...)
	case "unamend":
		return git("reset", "--mixed", "HEAD@{1}")
	case "uncommit":
		return git("reset", "--soft", "HEAD~")
	case "undo":
		return git("reset", "--mixed", "HEAD~")
	case "unknown":
		return gitUnknown()
	case "untrack":
		return git("rm", append([]string{"--cached"}, args...)...)
	default:
		return fmt.Errorf("unknown git subcommand: %s", subcmd)
	}
}

func git(cmd string, args ...string) error {
	return runner.Run("git", append([]string{cmd}, args...)...)
}

func capture(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// splitGitArgs splits args into flags and positional args,
// handling -m/-C/-c/-F/-t which take a value.
func splitGitArgs(args []string) (flags []string, files []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			flags = append(flags, a)
			i++
			files = append(files, args[i:]...)
			return
		}
		switch a {
		case "-m", "-C", "-c", "-F", "-t":
			flags = append(flags, a)
			i++
			if i < len(args) {
				flags = append(flags, args[i])
			}
		default:
			if len(a) > 0 && a[0] == '-' {
				flags = append(flags, a)
			} else {
				files = append(files, args[i:]...)
				return
			}
		}
		i++
	}
	return
}

func gitAmend(args []string) error {
	flags, files := splitGitArgs(args)
	if len(files) == 0 {
		flags = append(flags, "--all")
	}
	allArgs := append([]string{"commit", "--amend", "--no-edit"}, flags...)
	allArgs = append(allArgs, files...)
	return runner.Run("git", allArgs...)
}

func gitCommit(args []string) error {
	flags, files := splitGitArgs(args)
	if len(files) == 0 {
		flags = append(flags, "--all")
	}
	allArgs := append([]string{"commit"}, flags...)
	allArgs = append(allArgs, files...)
	return runner.Run("git", allArgs...)
}

func gitAtTip() error {
	out, err := capture("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if out == "HEAD" {
		return fmt.Errorf("detached HEAD")
	}
	return nil
}

func gitBase(args []string) error {
	if err := gitAtTip(); err != nil {
		fmt.Print("(detached) ")
	}
	return runner.Run("git", append([]string{"--no-pager", "log", "-1", "--oneline", "--no-decorate"}, args...)...)
}

func gitMap(args []string) error {
	if err := gitAtTip(); err != nil {
		return gitGraph(nil)
	}
	return gitBase(args)
}

func gitBranch() error {
	out, err := capture("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if out != "HEAD" {
		fmt.Println(out)
	}
	return nil
}

func gitGraph(args []string) error {
	format := "--pretty=format:%C(auto)%h%C(auto)%d %s"
	if len(args) == 0 {
		err := runner.Run("git", "--no-pager", "log", "--graph", format, "@{upstream}..HEAD")
		if err != nil {
			return runner.Run("git", "--no-pager", "log", "--graph", format)
		}
		return nil
	}
	return runner.Run("git", append([]string{"--no-pager", "log", "--graph", format}, args...)...)
}

func gitFetchtime() error {
	gitDir, err := capture("git", "rev-parse", "--git-dir")
	if err != nil {
		return err
	}
	fi, err := os.Stat(gitDir + "/FETCH_HEAD")
	if err != nil {
		return err
	}
	fmt.Println(fi.ModTime().Unix())
	return nil
}

func gitHook(hookName string, args []string) error {
	gitDir, err := capture("git", "rev-parse", "--git-dir")
	if err != nil {
		return err
	}
	script := gitDir + "/hooks/" + hookName
	fi, err := os.Stat(script)
	if err != nil || fi.Mode()&0111 == 0 {
		fmt.Fprintf(os.Stderr, "No %s hook\n", script)
		return fmt.Errorf("no hook")
	}
	return runner.Run(script, args...)
}

func gitIgnore(args []string) error {
	rootDir, err := capture("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(rootDir+"/.gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, a := range args {
		fmt.Fprintln(f, a)
	}
	return nil
}

func gitNext() error {
	head, err := capture("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	out, err := capture("git", "rev-list", "--children", "--all")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == head {
			return runner.Run("git", "checkout", fields[1])
		}
	}
	fmt.Fprintln(os.Stderr, "no next commit")
	return fmt.Errorf("no next commit")
}

func gitPending(args []string) error {
	err := runner.Run("git", append([]string{"--no-pager", "log", "--oneline", "@{upstream}..HEAD"}, args...)...)
	if err != nil {
		return runner.Run("git", append([]string{"status", "--short"}, args...)...)
	}
	return nil
}

func gitRevert(args []string) error {
	if len(args) == 0 {
		return runner.Run("git", "reset", "--hard", "HEAD")
	}
	return runner.Run("git", append([]string{"checkout", "--"}, args...)...)
}

func gitReview(args []string) error {
	var reviewers []string
	var pushFlags []string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-r", "-m", "--reviewer":
			if i+1 < len(args) {
				reviewers = append(reviewers, args[i+1])
				i += 2
				continue
			}
		default:
			if strings.HasPrefix(args[i], "--reviewer=") {
				reviewers = append(reviewers, strings.TrimPrefix(args[i], "--reviewer="))
			} else {
				pushFlags = append(pushFlags, args[i])
			}
		}
		i++
	}

	pushArgs := append([]string{"push"}, pushFlags...)
	if err := runner.Run("git", pushArgs...); err != nil {
		return err
	}

	if runner.FindCommand("gh") != "" {
		// Check if PR exists.
		if err := runner.Run("gh", "pr", "view", "--json", "url", "-q", ".url"); err != nil {
			createArgs := []string{"pr", "create", "--fill"}
			if len(reviewers) == 0 {
				createArgs = append(createArgs, "--draft")
			} else {
				for _, r := range reviewers {
					createArgs = append(createArgs, "--reviewer", r)
				}
			}
			return runner.Run("gh", createArgs...)
		}
	}
	return nil
}

func gitUnknown() error {
	rootDir, err := capture("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	return runner.Run("git", "-C", rootDir, "ls-files", "-od", "--exclude-standard")
}

func gitCopy(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("copy requires source and destination")
	}
	if err := runner.Run("cp", args...); err != nil {
		return err
	}
	return runner.Run("git", "add", args[len(args)-1])
}
