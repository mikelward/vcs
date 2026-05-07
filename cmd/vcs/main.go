// vcs detects the version control system for the current directory
// and dispatches to the appropriate vcs-* binary.
//
// Usage: vcs [flags] <subcommand> [args...]
//
// Flags:
//
//	--vcs=NAME            Force VCS type (git, hg, jj) instead of autodetecting.
//	--hg-path=PATH        Path to hg/chg binary (passed through to vcs-hg).
//	-n, --dry-run,        Print the command that would be run instead of
//	    --simulate        executing it.
//
// Special subcommands:
//
//	detect [path]
//	            Print the detected VCS name. If path is given, detect the
//	            VCS for that file or directory instead of the current
//	            working directory.
//	rootdir     Print the repository root directory.
//	backend     Print the VCS backend (e.g. "git" for jj-on-git).
//	hosting     Print the hosting platform (e.g. "github").
//	prompt-info Print all prompt info in one invocation (see --format, --color).
//	prompt-line Print the full preprompt first line (host + dir + auth).
//	auto-fetch  Spawn a detached background fetch when the repo's fetch
//	            marker is older than --max-age (default 1h). Silent on
//	            no-op paths; intended to be called from shell prompt
//	            hooks after a cd.
//	clearcache  Remove .vcs_cache files under the current directory.
//	version     Print the vcs version, git commit, and build date.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mikelward/vcs/autofetch"
	"github.com/mikelward/vcs/promptinfo"
	"github.com/mikelward/vcs/promptline"
	"github.com/mikelward/vcs/vcsdetect"
	"github.com/mikelward/vcs/version"
)

func main() {
	args := os.Args[1:]

	var forceVCS string
	var hgPath string
	var dryRun bool
	var passthrough []string

	// Parse our flags, collect remaining args.
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "--vcs=") {
			forceVCS = strings.TrimPrefix(a, "--vcs=")
			i++
		} else if a == "--vcs" && i+1 < len(args) {
			forceVCS = args[i+1]
			i += 2
		} else if strings.HasPrefix(a, "--hg-path=") {
			hgPath = a // pass through as-is
			i++
		} else if a == "--hg-path" && i+1 < len(args) {
			hgPath = "--hg-path=" + args[i+1]
			i += 2
		} else if a == "-n" || a == "--dry-run" || a == "--simulate" {
			dryRun = true
			i++
		} else if a == "--version" || a == "-V" {
			fmt.Println(version.String("vcs"))
			return
		} else {
			passthrough = args[i:]
			break
		}
	}

	if dryRun {
		os.Setenv("VCS_DRY_RUN", "1")
	}

	if len(passthrough) == 0 {
		fmt.Fprintln(os.Stderr, "usage: vcs [--vcs=NAME] [--hg-path=PATH] [-n|--dry-run] <subcommand> [args...]")
		os.Exit(1)
	}

	subcmd := passthrough[0]
	subArgs := passthrough[1:]

	// Handle special subcommands that don't dispatch to vcs-*.
	switch subcmd {
	case "--list-commands":
		listCommands()
		return
	case "detect":
		dir, err := detectDir(subArgs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vcs detect:", err)
			os.Exit(1)
		}
		info := detectAt(forceVCS, dir)
		fmt.Println(info.VCS)
		return
	case "rootdir":
		info := detect(forceVCS)
		if len(subArgs) > 0 {
			for _, arg := range subArgs {
				fmt.Println(filepath.Join(info.RootDir, arg))
			}
		} else {
			fmt.Println(info.RootDir)
		}
		return
	case "backend":
		info := detect(forceVCS)
		if info.Backend != "" {
			fmt.Println(info.Backend)
		}
		return
	case "hosting":
		info := detect(forceVCS)
		if info.Hosting != "" {
			fmt.Println(info.Hosting)
		}
		return
	case "prompt-info":
		promptInfo(forceVCS, hgPath, subArgs)
		return
	case "prompt-line":
		promptLineCmd(forceVCS, hgPath, subArgs)
		return
	case "auto-fetch":
		autoFetchCmd(forceVCS, hgPath, subArgs)
		return
	case "clearcache":
		clearCache()
		return
	case "version":
		fmt.Println(version.Multiline("vcs"))
		return
	}

	// Detect VCS and dispatch.
	info := detect(forceVCS)
	binary := "vcs-" + info.VCS

	// Build args for the vcs-* binary.
	var execArgs []string
	if info.VCS == "hg" && hgPath != "" {
		execArgs = append(execArgs, hgPath)
	}
	execArgs = append(execArgs, subcmd)
	execArgs = append(execArgs, subArgs...)

	// Try to exec the binary (replaces this process).
	path, err := exec.LookPath(binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vcs: %s not found on PATH\n", binary)
		os.Exit(1)
	}
	argv := append([]string{path}, execArgs...)
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "vcs: exec %s: %v\n", binary, err)
		os.Exit(1)
	}
}

func detect(forceVCS string) *vcsdetect.Info {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcs: cannot get working directory:", err)
		os.Exit(1)
	}
	return detectAt(forceVCS, dir)
}

func detectAt(forceVCS, dir string) *vcsdetect.Info {
	if forceVCS != "" {
		// Still detect to get rootdir/backend/hosting, but override VCS.
		info, _ := vcsdetect.Detect(dir)
		if info == nil {
			info = &vcsdetect.Info{VCS: forceVCS}
		} else {
			info.VCS = forceVCS
		}
		return info
	}

	info, err := vcsdetect.Detect(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcs: no version control system detected")
		os.Exit(1)
	}
	return info
}

// detectDir resolves the directory to detect in from the subcommand args.
// With no args, returns the current working directory. With one arg, returns
// the path itself if it's a directory, or its parent directory if it's a file.
func detectDir(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	if len(args) > 1 {
		return "", fmt.Errorf("too many arguments")
	}
	path := args[0]
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return path, nil
	}
	return filepath.Dir(path), nil
}

func promptInfo(forceVCS string, hgPath string, args []string) {
	format := promptinfo.DefaultFormat
	colorMode := "auto"

	// Parse --format, --color, and --hg-path flags.
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "--hg-path=") {
			hgPath = strings.TrimPrefix(a, "--hg-path=")
			i++
		} else if a == "--hg-path" && i+1 < len(args) {
			hgPath = args[i+1]
			i += 2
		} else if strings.HasPrefix(a, "--format=") {
			format = strings.TrimPrefix(a, "--format=")
			i++
		} else if a == "--format" && i+1 < len(args) {
			format = args[i+1]
			i += 2
		} else if strings.HasPrefix(a, "--color=") {
			colorMode = strings.TrimPrefix(a, "--color=")
			i++
		} else if a == "--color" && i+1 < len(args) {
			colorMode = args[i+1]
			i += 2
		} else {
			fmt.Fprintf(os.Stderr, "vcs prompt-info: unknown flag: %s\n", a)
			os.Exit(1)
		}
	}

	var color bool
	switch colorMode {
	case "always":
		color = true
	case "never":
		color = false
	default: // "auto"
		fi, err := os.Stdout.Stat()
		if err == nil {
			color = fi.Mode()&os.ModeCharDevice != 0
		}
	}

	info := detect(forceVCS)
	fields := promptinfo.ParseFields(format)
	result, err := promptinfo.Gather(info, fields, &promptinfo.Options{HgPath: hgPath})
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcs prompt-info:", err)
		os.Exit(1)
	}
	output := promptinfo.Format(result, format, color)
	fmt.Println(output)
}

func clearCache() {
	filepath.Walk(".", func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.Name() == ".vcs_cache" && !fi.IsDir() {
			os.Remove(path)
		}
		return nil
	})
}

func promptLineCmd(forceVCS, hgPath string, args []string) {
	// Top-level stores hgPath with the --hg-path= prefix for passthrough
	// to vcs-hg; strip it here since promptline.Options.HgPath wants the
	// bare path.
	hgPath = strings.TrimPrefix(hgPath, "--hg-path=")
	opts := &promptline.Options{
		HgPath:   hgPath,
		ForceVCS: forceVCS,
		Shpool:   os.Getenv("SHPOOL_SESSION_NAME"),
	}
	colorMode := "auto"

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--hostname="):
			opts.Hostname = strings.TrimPrefix(a, "--hostname=")
			i++
		case a == "--hostname" && i+1 < len(args):
			opts.Hostname = args[i+1]
			i += 2
		case a == "--production":
			opts.Production = true
			i++
		case strings.HasPrefix(a, "--shpool="):
			opts.Shpool = strings.TrimPrefix(a, "--shpool=")
			i++
		case a == "--shpool" && i+1 < len(args):
			opts.Shpool = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--hg-path="):
			opts.HgPath = strings.TrimPrefix(a, "--hg-path=")
			i++
		case a == "--hg-path" && i+1 < len(args):
			opts.HgPath = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--format="):
			opts.Format = strings.TrimPrefix(a, "--format=")
			i++
		case a == "--format" && i+1 < len(args):
			opts.Format = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--color="):
			colorMode = strings.TrimPrefix(a, "--color=")
			i++
		case a == "--color" && i+1 < len(args):
			colorMode = args[i+1]
			i += 2
		case a == "--no-ssh":
			opts.SkipAuth = true
			opts.AuthOK = true
			i++
		default:
			fmt.Fprintf(os.Stderr, "vcs prompt-line: unknown flag: %s\n", a)
			os.Exit(1)
		}
	}

	if opts.Hostname == "" {
		h := os.Getenv("HOSTNAME")
		if h == "" {
			if sh, err := os.Hostname(); err == nil {
				h = sh
			}
		}
		if idx := strings.Index(h, "."); idx >= 0 {
			h = h[:idx]
		}
		opts.Hostname = h
	}

	switch colorMode {
	case "always":
		opts.Color = true
	case "never":
		opts.Color = false
	default: // auto
		if fi, err := os.Stdout.Stat(); err == nil {
			opts.Color = fi.Mode()&os.ModeCharDevice != 0
		}
	}

	fmt.Println(promptline.Build(opts))
}

// autoFetchCmd parses flags for the auto-fetch subcommand and calls
// autofetch.Run. Output is silent on the no-op paths; --verbose prints
// a single-word action ("not-in-repo" / "fresh" / "fetched" /
// "unsupported") so shell tests and humans can verify what happened.
//
// Errors from the underlying spawn are printed to stderr but the
// process exits 0 — auto-fetch is best-effort prompt-time code, not a
// user-facing fetch tool.
func autoFetchCmd(forceVCS, hgPath string, args []string) {
	maxAge := time.Hour
	verbose := false

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--verbose" || a == "-v":
			verbose = true
			i++
		case strings.HasPrefix(a, "--max-age="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--max-age="))
			if err != nil {
				fmt.Fprintln(os.Stderr, "vcs auto-fetch: invalid --max-age:", err)
				os.Exit(2)
			}
			maxAge = d
			i++
		case a == "--max-age":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "vcs auto-fetch: --max-age needs a value")
				os.Exit(2)
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "vcs auto-fetch: invalid --max-age:", err)
				os.Exit(2)
			}
			maxAge = d
			i += 2
		default:
			fmt.Fprintln(os.Stderr, "vcs auto-fetch: unknown flag:", a)
			os.Exit(2)
		}
	}

	// main()'s flag parser stores --hg-path as the literal flag string
	// (`--hg-path=PATH`) so it can be passed through to vcs-hg verbatim.
	// autofetch.Options.HgPath wants just the path, so strip the prefix.
	bareHgPath := strings.TrimPrefix(hgPath, "--hg-path=")

	action, err := autofetch.Run(&autofetch.Options{
		MaxAge:   maxAge,
		ForceVCS: forceVCS,
		HgPath:   bareHgPath,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcs auto-fetch:", err)
	}
	if verbose {
		fmt.Println(action)
	}
}
