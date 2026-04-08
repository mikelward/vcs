// Package promptline assembles the first line of the shell prompt in a
// single process: hostname (with optional production-host coloring), shpool
// session tag, directory/VCS info, and auth warnings.
//
// It replaces the host_info + dir_info + auth_info shell functions in
// mikelward/conf:shrc, collapsing several subshell forks per prompt into
// one `vcs` invocation. The VCS directory piece is delegated to promptinfo.
package promptline

import (
	"os"
	"os/exec"
	"strings"

	"github.com/mikelward/vcs/promptinfo"
	"github.com/mikelward/vcs/vcsdetect"
)

// ANSI color codes. Kept in sync with promptinfo.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// Options controls Build.
type Options struct {
	// Hostname is the short hostname text to show. Required (empty falls
	// back to os.Hostname() + domain strip).
	Hostname string
	// Production, when true, renders the hostname in red. The caller
	// (shell) decides what counts as production — keeps policy out of
	// the binary.
	Production bool
	// Shpool is the shpool session name. Non-empty renders as "[session]"
	// with a green session name; empty renders a yellow "shpool" warning.
	Shpool string
	// NoVCS forces the no-binary fallback path for the directory part:
	// tilde-expanded cwd plus a yellow "vcs" warning. Mostly useful for
	// testing; real callers won't pass this because by definition the
	// binary is available when they call it.
	NoVCS bool
	// HgPath is passed through to promptinfo.Options.
	HgPath string
	// Format is the promptinfo format string. Empty means
	// promptinfo.DefaultFormat.
	Format string
	// ForceVCS overrides vcsdetect autodetection (mirrors `vcs --vcs=NAME`).
	ForceVCS string
	// HomeDir overrides $HOME for tilde substitution (for tests).
	HomeDir string
	// Cwd overrides os.Getwd() for the tilde fallback path (for tests).
	// Note: promptinfo.Gather reads os.Getwd() directly for the subdir
	// field, so this override only affects the non-repo path.
	Cwd string
	// Color enables ANSI color escapes.
	Color bool
	// SkipAuth disables the ssh-add check. When true, AuthOK is used
	// instead. Tests always set SkipAuth=true.
	SkipAuth bool
	// AuthOK is consulted only when SkipAuth is true. true = no warning,
	// false = emit "SSH" warning.
	AuthOK bool
}

// Build assembles the full prompt line (no leading \r, no trailing \n).
// The shell caller frames it with whatever CR/LF logic it wants.
func Build(opts *Options) string {
	host := HostPart(opts.Hostname, opts.Shpool, opts.Production, opts.Color)
	dir := dirPart(opts)
	auth := authPart(opts)

	out := host + " " + dir
	if auth != "" {
		out += " " + auth
	}
	return out
}

// HostPart formats the hostname + shpool tag. Matches the output of the
// shell's host_info function (minus the trailing newline that command
// substitution strips anyway).
//
// Not-in-shpool renders a yellow "shpool" warning so you can see at a
// glance whether the session is persistent.
func HostPart(hostname, shpool string, production, color bool) string {
	host := hostname
	if color && production {
		host = colorRed + host + colorReset
	}

	var tag string
	if shpool != "" {
		session := shpool
		if color {
			session = colorGreen + session + colorReset
		}
		tag = " [" + session + "]"
	} else {
		if color {
			tag = " " + colorYellow + "shpool" + colorReset
		} else {
			tag = " shpool"
		}
	}
	return host + tag
}

// TildeDirectory returns cwd with a leading $HOME replaced by "~". If home
// is empty or cwd is outside home, cwd is returned unchanged.
func TildeDirectory(cwd, home string) string {
	if home == "" {
		return cwd
	}
	if cwd == home {
		return "~"
	}
	if strings.HasPrefix(cwd, home+"/") {
		return "~" + strings.TrimPrefix(cwd, home)
	}
	return cwd
}

// AuthWarning formats a single auth warning. Exported for test symmetry.
func AuthWarning(label string, color bool) string {
	if color {
		return colorYellow + label + colorReset
	}
	return label
}

// dirPart formats the directory info. In a repo it delegates to promptinfo;
// outside a repo it prints tilde-expanded cwd. Mirrors shrc's dir_info
// composition: the whole directory piece is wrapped in blue so uncolored
// spans (e.g. branch name from promptinfo) inherit the dir color.
func dirPart(opts *Options) string {
	cwd := opts.Cwd
	if cwd == "" {
		c, err := os.Getwd()
		if err != nil {
			return ""
		}
		cwd = c
	}
	home := opts.HomeDir
	if home == "" {
		home = os.Getenv("HOME")
	}

	if opts.NoVCS {
		return noVCSFallback(cwd, home, opts.Color)
	}

	info, err := vcsdetect.Detect(cwd)
	if opts.ForceVCS != "" {
		if info == nil {
			info = &vcsdetect.Info{VCS: opts.ForceVCS, RootDir: cwd}
		} else {
			info.VCS = opts.ForceVCS
		}
		err = nil
	}
	if err != nil || info == nil {
		return wrapBlue(TildeDirectory(cwd, home), opts.Color)
	}

	format := opts.Format
	if format == "" {
		format = promptinfo.DefaultFormat
	}
	fields := promptinfo.ParseFields(format)
	result, perr := promptinfo.Gather(info, fields, &promptinfo.Options{HgPath: opts.HgPath})
	if perr != nil {
		return wrapBlue(TildeDirectory(cwd, home), opts.Color)
	}
	out := promptinfo.Format(result, format, opts.Color)
	if out == "" {
		return wrapBlue(TildeDirectory(cwd, home), opts.Color)
	}
	return wrapBlue(out, opts.Color)
}

func noVCSFallback(cwd, home string, color bool) string {
	dir := TildeDirectory(cwd, home)
	if color {
		return colorBlue + dir + " " + colorYellow + "vcs" + colorReset + colorReset
	}
	return dir + " vcs"
}

func wrapBlue(s string, color bool) string {
	if s == "" || !color {
		return s
	}
	return colorBlue + s + colorReset
}

func authPart(opts *Options) string {
	var ok bool
	if opts.SkipAuth {
		ok = opts.AuthOK
	} else {
		ok = sshValid()
	}
	if ok {
		return ""
	}
	return AuthWarning("SSH", opts.Color)
}

// sshValid returns true when `ssh-add -L` exits 0 (agent has at least one
// identity). Any non-zero exit or missing ssh-add is treated as "need auth".
// This still forks once, but the fork lives inside the Go binary instead
// of the shell. A future optimization is to speak the ssh-agent protocol
// over $SSH_AUTH_SOCK directly.
func sshValid() bool {
	cmd := exec.Command("ssh-add", "-L")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
