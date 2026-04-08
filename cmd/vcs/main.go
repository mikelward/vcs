// vcs detects the version control system for the current directory
// and dispatches to the appropriate vcs-* binary.
//
// Usage: vcs [flags] <subcommand> [args...]
//
// Flags:
//
//	--vcs=NAME       Force VCS type (git, hg, jj) instead of autodetecting.
//	--hg-path=PATH   Path to hg/chg binary (passed through to vcs-hg).
//
// Special subcommands:
//
//	detect      Print the detected VCS name.
//	rootdir     Print the repository root directory.
//	backend     Print the VCS backend (e.g. "git" for jj-on-git).
//	hosting     Print the hosting platform (e.g. "github").
//	prompt-info Print all prompt info in one invocation (see --format, --color).
//	prompt-line Print the full preprompt first line (host + dir + auth).
//	clearcache  Remove .vcs_cache files under the current directory.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mikelward/vcs/promptinfo"
	"github.com/mikelward/vcs/promptline"
	"github.com/mikelward/vcs/vcsdetect"
)

func main() {
	args := os.Args[1:]

	var forceVCS string
	var hgPath string
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
		} else {
			passthrough = args[i:]
			break
		}
	}

	if len(passthrough) == 0 {
		fmt.Fprintln(os.Stderr, "usage: vcs [--vcs=NAME] [--hg-path=PATH] <subcommand> [args...]")
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
		info := detect(forceVCS)
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
	case "clearcache":
		clearCache()
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
