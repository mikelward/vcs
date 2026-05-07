// Package promptinfo gathers VCS prompt information in a single invocation,
// replacing 6-12 shell forks per prompt with 1 process. Branch is read from
// files; status (and the derived "behind upstream" signal) requires a
// subprocess.
package promptinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mikelward/vcs/runner"
	"github.com/mikelward/vcs/vcsdetect"
)

// DefaultFormat is the default format string for prompt-info output.
const DefaultFormat = `{project} {subdir} {branch} {status} {behind}`

// fetchStaleThreshold is how old FETCH_HEAD must be to be considered stale,
// used as a fallback for repos with no configured upstream (where the
// ahead/behind comparison can't be made).
const fetchStaleThreshold = 24 * time.Hour

// Result holds the gathered prompt information.
type Result struct {
	Project string // filepath.Base(rootDir)
	Subdir  string // cwd relative to rootDir
	Branch  string // current branch (empty for jj)
	Status  string // "*" if there are uncommitted changes, else ""
	Behind  bool   // true if upstream has commits we don't, or (no upstream) FETCH_HEAD is stale
}

// ParseFields extracts {field} names from a format string.
func ParseFields(format string) map[string]bool {
	fields := map[string]bool{}
	re := regexp.MustCompile(`\{(\w+)\}`)
	for _, m := range re.FindAllStringSubmatch(format, -1) {
		fields[m[1]] = true
	}
	return fields
}

// Gather collects prompt information for the given VCS info, only computing
// requested fields.
// Options controls optional behavior of Gather.
type Options struct {
	HgPath string // path to hg/chg binary (empty = auto-detect)
}

func Gather(info *vcsdetect.Info, fields map[string]bool, opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{}
	}
	r := &Result{}

	// status (and the derived behind/upstream signal) is the only slow
	// (subprocess-forking) field. Launch it up front so it runs
	// concurrently with the file-read fields below.
	var wg sync.WaitGroup
	var sbStatus string
	var sbBehind, sbHasUpstream bool
	if fields["status"] || fields["behind"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sbStatus, sbBehind, sbHasUpstream = getStatusAndBehind(info, opts, fields["status"], fields["behind"])
		}()
	}

	if fields["project"] {
		r.Project = filepath.Base(info.RootDir)
	}

	if fields["subdir"] {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(info.RootDir, cwd); err == nil && rel != "." {
				r.Subdir = rel
			}
		}
	}

	if fields["branch"] {
		r.Branch = getBranch(info)
	}

	wg.Wait()
	if fields["status"] {
		r.Status = sbStatus
	}
	if fields["behind"] {
		if sbBehind {
			r.Behind = true
		} else if !sbHasUpstream {
			// No upstream (or non-git VCS): fall back to FETCH_HEAD mtime
			// so repos that can't compare refs still get a nag if the
			// fetch marker has gone stale.
			r.Behind = getFetchStale(fetchHeadPath(info))
		}
	}
	return r, nil
}

func capture(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// getFetchStale checks the modification time of a fetch marker file and returns
// whether it is stale (older than fetchStaleThreshold).
func getFetchStale(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(fi.ModTime()) > fetchStaleThreshold
}

func getBranch(info *vcsdetect.Info) string {
	switch info.VCS {
	case "git":
		// Read .git/HEAD directly to avoid forking.
		data, err := os.ReadFile(filepath.Join(info.RootDir, ".git", "HEAD"))
		if err != nil {
			return ""
		}
		head := strings.TrimSpace(string(data))
		if strings.HasPrefix(head, "ref: refs/heads/") {
			return strings.TrimPrefix(head, "ref: refs/heads/")
		}
		return "" // detached HEAD
	case "hg":
		// Read .hg/branch directly to avoid forking.
		data, err := os.ReadFile(filepath.Join(info.RootDir, ".hg", "branch"))
		if err != nil {
			return "default" // hg default when file doesn't exist
		}
		return strings.TrimSpace(string(data))
	}
	// jj: always empty
	return ""
}

// getStatusAndBehind runs the VCS status/behind probes and returns:
//   - status: "*" if there are uncommitted changes, else ""
//   - behind: true if fetched upstream state is ahead of the current checkout
//   - hasUpstream: true if fetched upstream state is available for comparison
//
// For git, --branch --porcelain prints a leading branch line like
//
//	## main...origin/main [behind 3]
//
// when an upstream is configured. The ahead/behind brackets only appear when
// the local branch is non-empty relative to upstream. hasUpstream is true
// whenever the "..." separator is present, regardless of ahead/behind state.
//
// For hg, summary reports both dirty state and whether the working copy can be
// updated to a fetched branch head.
//
// For jj, fetched remote bookmarks are the upstream signal. A remote bookmark
// outside ancestors(@) means there is fetched work that the current checkout
// does not contain.
func getStatusAndBehind(info *vcsdetect.Info, opts *Options, needStatus, needBehind bool) (status string, behind bool, hasUpstream bool) {
	var out string
	switch info.VCS {
	case "git":
		out, _ = capture("git", "-C", info.RootDir, "status", "--branch", "--porcelain", "--untracked-files=all")
		lines := strings.SplitN(out, "\n", 2)
		var branchLine, rest string
		if len(lines) > 0 {
			branchLine = lines[0]
		}
		if len(lines) > 1 {
			rest = lines[1]
		}
		if strings.Contains(branchLine, "...") {
			hasUpstream = true
			if strings.Contains(branchLine, "behind ") {
				behind = true
			}
		}
		if strings.TrimSpace(rest) != "" {
			status = "*"
		}
		return
	case "jj":
		var desc, diff, bookmarkStates string
		var jjwg sync.WaitGroup
		if needStatus {
			// Legacy behavior: a non-empty description on @ means "clean"
			// (the user committed work); otherwise check diff. Since jj
			// has ~tens-of-ms startup per invocation, run both in parallel
			// rather than serially.
			jjwg.Add(2)
			go func() {
				defer jjwg.Done()
				desc, _ = capture("jj", "-R", info.RootDir, "log", "--no-graph", "-r", "@", "-T", "description")
			}()
			go func() {
				defer jjwg.Done()
				diff, _ = capture("jj", "-R", info.RootDir, "diff", "--summary")
			}()
		}
		if needBehind {
			jjwg.Add(1)
			go func() {
				defer jjwg.Done()
				tmpl := `if(self.contained_in("ancestors(@)"), "upstream\n", "behind\n")`
				bookmarkStates, _ = capture("jj", "-R", info.RootDir, "log", "--no-graph", "-r", "remote_bookmarks()", "-T", tmpl)
			}()
		}
		jjwg.Wait()
		if needStatus && desc == "" {
			out = diff
		}
		if needBehind {
			for _, line := range strings.Split(bookmarkStates, "\n") {
				switch strings.TrimSpace(line) {
				case "behind":
					behind = true
					hasUpstream = true
				case "upstream":
					hasUpstream = true
				}
			}
		}
	case "hg":
		if needBehind {
			out, _ = capture(resolveHg(opts), "-R", info.RootDir, "summary")
			status, behind, hasUpstream = parseHgSummary(out, needStatus)
			return
		}
		out, _ = capture(resolveHg(opts), "-R", info.RootDir, "status")
	}
	if strings.TrimSpace(out) != "" {
		status = "*"
	}
	return
}

func parseHgSummary(out string, needStatus bool) (status string, behind bool, hasUpstream bool) {
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "commit:"):
			commit := strings.TrimSpace(strings.TrimPrefix(line, "commit:"))
			if needStatus && commit != "(clean)" {
				status = "*"
			}
		case strings.HasPrefix(line, "update:"):
			hasUpstream = true
			update := strings.TrimSpace(strings.TrimPrefix(line, "update:"))
			if update != "" && update != "(current)" {
				behind = true
			}
		}
	}
	return status, behind, hasUpstream
}

func fetchHeadPath(info *vcsdetect.Info) string {
	switch info.VCS {
	case "git":
		return filepath.Join(info.RootDir, ".git", "FETCH_HEAD")
	case "jj":
		return filepath.Join(info.RootDir, ".jj", "repo", "store", "git", "FETCH_HEAD")
	case "hg":
		return filepath.Join(info.RootDir, ".hg", "store", "00changelog.i")
	}
	return ""
}

func resolveHg(opts *Options) string {
	if opts.HgPath != "" {
		return opts.HgPath
	}
	if p := runner.FindCommand("chg"); p != "" {
		return p
	}
	return "hg"
}

// colorWrap wraps text in ANSI color codes. Returns empty string if text is empty.
func colorWrap(text, colorCode string) string {
	if text == "" {
		return ""
	}
	return colorCode + text + "\033[0m"
}

// Format substitutes {field} placeholders in the format string with values
// from Result. It collapses multiple spaces around empty fields and strips
// trailing empty lines.
func Format(r *Result, format string, color bool) string {
	s := strings.ReplaceAll(format, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")

	project := r.Project
	subdir := r.Subdir
	branch := r.Branch
	status := r.Status
	behind := ""
	if r.Behind {
		behind = "pull"
	}

	if color {
		project = colorWrap(project, "\033[32m")
		subdir = colorWrap(subdir, "\033[34m")
		status = colorWrap(status, "\033[33m")
		behind = colorWrap(behind, "\033[33m")
	}

	s = strings.ReplaceAll(s, "{project}", project)
	s = strings.ReplaceAll(s, "{subdir}", subdir)
	s = strings.ReplaceAll(s, "{branch}", branch)
	s = strings.ReplaceAll(s, "{status}", status)
	s = strings.ReplaceAll(s, "{behind}", behind)

	// Collapse multiple spaces into one on each line, and trim trailing spaces.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		lines[i] = strings.TrimRight(line, " ")
	}
	s = strings.Join(lines, "\n")

	// Strip trailing empty lines.
	s = strings.TrimRight(s, "\n")

	return s
}
