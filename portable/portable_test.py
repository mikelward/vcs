"""Tests for the portable vcs: the sh front end and vcs-py.

Run with `make test`, or `python3 -m unittest discover -s portable -p
'*_test.py'`. Tests needing hg, jj, dash or Go skip when they are missing.
"""

import concurrent.futures
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
VCS_SH = os.path.join(HERE, "vcs")
VCS_PY = os.path.join(HERE, "vcs-py")

# The front end under each /bin/sh we expect (dash on Debian and Ubuntu,
# bash on Fedora and macOS), plus vcs-py on its own.
SH_IMPLS = [argv + [VCS_SH] for argv in (["dash"], ["bash", "--posix"])
            if shutil.which(argv[0])] or [["sh", VCS_SH]]
ALL_IMPLS = SH_IMPLS + [[sys.executable, VCS_PY]]

HOURS = 3600


def impl_name(impl):
    return os.path.basename(impl[0]) + " " + os.path.basename(impl[-1])


class Env(object):
    """A scratch HOME with a fixed identity for git, hg and jj."""

    def __init__(self):
        self.tmp = tempfile.mkdtemp(prefix="vcs-portable-test-")
        self.env = dict(os.environ)
        for k in list(self.env):
            if k.startswith("GIT_") or k in ("VCS_DRY_RUN", "HGRCPATH"):
                del self.env[k]
        self.env.update({
            "HOME": self.tmp,
            "GIT_AUTHOR_NAME": "T", "GIT_AUTHOR_EMAIL": "t@example.com",
            "GIT_COMMITTER_NAME": "T", "GIT_COMMITTER_EMAIL": "t@example.com",
            "GIT_CONFIG_NOSYSTEM": "1",
            "HGUSER": "T <t@example.com>", "HGPLAIN": "1",
            "JJ_USER": "T", "JJ_EMAIL": "t@example.com",
        })
        with open(os.path.join(self.tmp, ".gitconfig"), "w") as f:
            f.write("[init]\n\tdefaultBranch = main\n"
                    "[advice]\n\tdetachedHead = false\n"
                    "[core]\n\texcludesFile = %s\n"
                    % os.path.join(self.tmp, ".gitexclude"))
        # The detection cache isn't part of the project; conf's
        # gitexclude ignores it as a dotfile.
        with open(os.path.join(self.tmp, ".gitexclude"), "w") as f:
            f.write(".vcs_cache\n")
        with open(os.path.join(self.tmp, ".hgexclude"), "w") as f:
            f.write("syntax: glob\n.vcs_cache\n")
        with open(os.path.join(self.tmp, ".hgrc"), "w") as f:
            f.write("[ui]\nignore = %s\n"
                    % os.path.join(self.tmp, ".hgexclude"))

    def close(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def path(self, *parts):
        return os.path.join(self.tmp, *parts)

    def sh(self, cmd, cwd):
        subprocess.check_call(cmd, shell=True, cwd=cwd, env=self.env,
                              stdout=subprocess.DEVNULL,
                              stderr=subprocess.DEVNULL)

    def vcs(self, impl, args, cwd, extra_env=None):
        env = dict(self.env)
        env["PWD"] = cwd
        env.update(extra_env or {})
        p = subprocess.run(impl + args, cwd=cwd, env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           universal_newlines=True)
        return p.returncode, p.stdout, p.stderr


def age(path, seconds):
    t = time.time() - seconds
    os.utime(path, (t, t))


def make_git(env, name="proj"):
    """A clone of an origin with two commits, so origin/HEAD exists."""
    origin = env.path(name + "-origin")
    os.makedirs(origin)
    env.sh("git init -q && echo a > a && git add a && git commit -qm one"
           " && echo b > b && git add b && git commit -qm two", origin)
    env.sh("git clone -q %s %s" % (origin, name), env.tmp)
    root = env.path(name)
    env.sh("git fetch -q", root)
    env.sh("git remote set-url origin https://github.com/example/proj.git",
           root)
    return root


class Base(unittest.TestCase):
    def setUp(self):
        self.e = Env()
        self.addCleanup(self.e.close)

    def each(self, impls, args, cwd, extra_env=None):
        for impl in impls:
            yield impl_name(impl), self.e.vcs(impl, args, cwd, extra_env)

    def assertAll(self, impls, args, cwd, out, rc=0, extra_env=None):
        for name, (code, stdout, stderr) in self.each(impls, args, cwd,
                                                      extra_env):
            with self.subTest(impl=name, args=args):
                self.assertEqual((code, stdout), (rc, out), stderr)

    def prompt(self, cwd, fmt=None, impls=ALL_IMPLS, extra_env=None,
               want=""):
        args = ["prompt-info", "--color=never"]
        if fmt:
            args.append("--format=" + fmt)
        self.assertAll(impls, args, cwd, want + "\n", extra_env=extra_env)


class CommandListTest(Base):
    def test_lists_match_commands_go(self):
        with open(os.path.join(REPO, "commands.go")) as f:
            want = re.findall(r'^\t"(\w+)",$', f.read(), re.M)
        self.assertGreater(len(want), 50)
        self.assertAll(ALL_IMPLS, ["--list-commands"], self.e.tmp,
                       "\n".join(want) + "\n")


class DetectTest(Base):
    def test_outside_a_repo(self):
        for name, (rc, out, err) in self.each(ALL_IMPLS, ["detect"],
                                              self.e.tmp):
            with self.subTest(impl=name):
                self.assertEqual(rc, 1)
                self.assertEqual(err,
                                 "vcs: no version control system detected\n")

    def test_git(self):
        root = make_git(self.e)
        sub = os.path.join(root, "sub")
        os.mkdir(sub)
        for name, args, want in (("detect", ["detect"], "git\n"),
                                 ("rootdir", ["rootdir"], root + "\n"),
                                 ("backend", ["backend"], "git\n"),
                                 ("hosting", ["hosting"], "github\n")):
            for cwd in (root, sub):
                self.assertAll(ALL_IMPLS, args, cwd, want)
                os.remove(os.path.join(cwd, ".vcs_cache"))

    def test_cache_is_written_and_trusted(self):
        root = make_git(self.e)
        self.assertAll(SH_IMPLS[:1], ["detect"], root, "git\n")
        with open(os.path.join(root, ".vcs_cache")) as f:
            self.assertEqual(f.read(), "git git github\n%s\n" % root)
        with open(os.path.join(root, ".vcs_cache"), "w") as f:
            f.write("hg - -\n/some where\n")
        self.assertAll(ALL_IMPLS, ["detect"], root, "hg\n")
        self.assertAll(ALL_IMPLS, ["rootdir"], root, "/some where\n")
        self.assertAll(ALL_IMPLS, ["backend"], root, "")

    def test_worktree_hosting(self):
        root = make_git(self.e)
        self.e.sh("git worktree add -q ../wt", root)
        self.assertAll(ALL_IMPLS, ["hosting"], self.e.path("wt"), "github\n")

    def test_forced_vcs(self):
        root = make_git(self.e)
        self.assertAll(ALL_IMPLS, ["--vcs=hg", "detect"], root, "hg\n")
        self.assertAll(ALL_IMPLS, ["--vcs", "hg", "rootdir"], root,
                       root + "\n")

    def test_path_and_rootdir_arguments_go_to_python(self):
        root = make_git(self.e)
        self.assertAll(ALL_IMPLS, ["detect", root + "/a"], self.e.tmp,
                       "git\n")
        self.assertAll(ALL_IMPLS, ["rootdir", "x/../y"], root,
                       root + "/y\n")

    def test_jj_secondary_workspace_backend(self):
        main = self.e.path("main", ".jj", "repo", "store")
        os.makedirs(main)
        with open(os.path.join(main, "type"), "w") as f:
            f.write("piper\n")
        os.makedirs(self.e.path("second", ".jj"))
        with open(self.e.path("second", ".jj", "repo"), "w") as f:
            f.write("../../main/.jj/repo")
        self.assertAll(ALL_IMPLS, ["backend"], self.e.path("second"),
                       "piper\n")

    def test_clearcache(self):
        root = make_git(self.e)
        sub = os.path.join(root, "sub")
        os.mkdir(sub)
        for impl in ALL_IMPLS:
            for d in (root, sub):
                self.e.vcs(impl, ["detect"], d)
            self.assertAll([impl], ["clearcache"], root, "")
            self.assertFalse(os.path.exists(os.path.join(sub, ".vcs_cache")))


class GitPromptTest(Base):
    def setUp(self):
        Base.setUp(self)
        self.root = make_git(self.e)

    def test_clean_and_up_to_date(self):
        self.prompt(self.root, want="proj main")

    def test_dirty(self):
        with open(os.path.join(self.root, "new"), "w") as f:
            f.write("x")
        self.prompt(self.root, want="proj main *")

    def test_subdir(self):
        sub = os.path.join(self.root, "a b", "c")
        os.makedirs(sub)
        self.prompt(sub, fmt="{project} {subdir} {branch}",
                    want="proj a b/c main")

    def test_behind_upstream(self):
        self.e.sh("git reset -q --hard HEAD~", self.root)
        self.prompt(self.root, want="proj main pull")

    def test_stale_fetch_is_behind(self):
        age(os.path.join(self.root, ".git", "FETCH_HEAD"), 5 * HOURS)
        self.prompt(self.root, want="proj main pull")

    def test_never_fetched_is_not_behind(self):
        os.remove(os.path.join(self.root, ".git", "FETCH_HEAD"))
        self.prompt(self.root, want="proj main")

    def test_detached(self):
        self.e.sh("git checkout -q HEAD~", self.root)
        self.prompt(self.root, fmt="{project} {branch}", want="proj")

    def test_worktree(self):
        self.e.sh("git worktree add -q -b feature ../wt", self.root)
        wt = self.e.path("wt")
        self.prompt(wt, fmt="{project} {branch} {behind}", want="wt feature")
        # A worktree has its own FETCH_HEAD, beside its own HEAD.
        fetch_head = subprocess.check_output(
            ["git", "rev-parse", "--path-format=absolute", "--git-path",
             "FETCH_HEAD"], cwd=wt, universal_newlines=True).strip()
        self.assertIn(os.path.join(".git", "worktrees"), fetch_head)
        open(fetch_head, "w").close()
        age(fetch_head, 5 * HOURS)
        self.prompt(wt, fmt="{project} {branch} {behind}",
                    want="wt feature pull")

    def test_format_escapes_and_spacing(self):
        self.prompt(self.root, fmt=r"[{project}]  {status}\n{branch}\t.  ",
                    want="[proj]\nmain\t.")

    def test_color(self):
        with open(os.path.join(self.root, "new"), "w") as f:
            f.write("x")
        want = "\033[32mproj\033[0m main \033[33m*\033[0m\n"
        self.assertAll(ALL_IMPLS, ["prompt-info", "--color", "always"],
                       self.root, want)

    def test_failed_status_is_reported(self):
        with open(os.path.join(self.root, ".git", "index"), "w") as f:
            f.write("not an index")
        for name, (rc, out, err) in self.each(
                ALL_IMPLS, ["prompt-info", "--color=never"], self.root):
            with self.subTest(impl=name):
                self.assertEqual((rc, out), (0, "proj main\n"))
                self.assertIn("fatal:", err)
                self.assertIn("vcs prompt-info: git status in proj failed, "
                              "so its status is unknown", err)

    def test_unknown_flag(self):
        for name, (rc, out, err) in self.each(
                ALL_IMPLS, ["prompt-info", "--bogus"], self.root):
            with self.subTest(impl=name):
                self.assertEqual((rc, err),
                                 (1, "vcs prompt-info: unknown flag: --bogus\n"))


@unittest.skipUnless(shutil.which("hg"), "hg not installed")
class HgPromptTest(Base):
    def setUp(self):
        Base.setUp(self)
        self.root = self.e.path("hgproj")
        os.mkdir(self.root)
        self.e.sh("hg init && echo a > a && hg add -q a && hg commit -qm one",
                  self.root)

    def test_clean(self):
        self.prompt(self.root, want="hgproj default")

    def test_dirty_on_a_branch(self):
        self.e.sh("hg branch -q feature && echo b >> a", self.root)
        self.prompt(self.root, want="hgproj feature *")
        self.prompt(self.root, fmt="{project} {status}", want="hgproj *")

    def test_stale_changelog_is_behind(self):
        age(os.path.join(self.root, ".hg", "store", "00changelog.i"),
            5 * HOURS)
        self.prompt(self.root, want="hgproj default pull")


@unittest.skipUnless(shutil.which("jj"), "jj not installed")
class JjPromptTest(Base):
    def setUp(self):
        Base.setUp(self)
        make_git(self.e, "src")
        self.e.sh("jj git clone %s jjproj" % self.e.path("src-origin"),
                  self.e.tmp)
        self.root = self.e.path("jjproj")

    def test_clean(self):
        self.prompt(self.root, want="jjproj")

    def test_dirty_until_described(self):
        with open(os.path.join(self.root, "new"), "w") as f:
            f.write("x")
        self.prompt(self.root, want="jjproj *")
        self.prompt(self.root, fmt="{project} {status}", want="jjproj *")
        self.e.sh("jj describe -m work", self.root)
        self.prompt(self.root, want="jjproj")

    def test_behind_remote_bookmark(self):
        self.e.sh("jj new 'main@origin-'", self.root)
        self.prompt(self.root, want="jjproj pull")


class PiperPromptTest(Base):
    """A non-git jj backend, faked: jj is a stub that records its calls."""

    def setUp(self):
        Base.setUp(self)
        self.root = self.e.path("piper")
        os.makedirs(os.path.join(self.root, ".jj", "repo", "store"))
        with open(os.path.join(self.root, ".jj", "repo", "store", "type"),
                  "w") as f:
            f.write("piper\n")
        bindir = self.e.path("bin")
        os.mkdir(bindir)
        self.calls = self.e.path("calls")
        # `op log` prints self.oplog and exits $JJ_STUB_EXIT.
        self.oplog = self.e.path("oplog")
        with open(os.path.join(bindir, "jj"), "w") as f:
            f.write('#!/bin/sh\necho "$*" >> %s\n'
                    'case "$*" in *"op log"*)\n'
                    '    cat %s 2>/dev/null\n'
                    '    test "${JJ_STUB_EXIT:-0}" = 0 || echo "Error: stub" >&2\n'
                    '    exit "${JJ_STUB_EXIT:-0}" ;;\n'
                    'esac\n' % (self.calls, self.oplog))
        os.chmod(os.path.join(bindir, "jj"), 0o755)
        self.path = {"PATH": bindir + os.pathsep + os.environ["PATH"]}
        self.marker = os.path.join(self.root, ".jj", "vcs-synced")

    def check(self, want):
        self.prompt(self.root, fmt="{project} {behind}", extra_env=self.path,
                    want=want)

    def test_reminder_follows_the_marker(self):
        open(self.marker, "w").close()
        self.check("piper")
        age(self.marker, 5 * HOURS)
        self.check("piper pull")
        self.assertFalse(os.path.exists(self.calls),
                         "with a marker, the reminder must not run jj")

    def op_log_calls(self):
        try:
            with open(self.calls) as f:
                return sum("op log" in line for line in f)
        except FileNotFoundError:
            return 0

    def seed(self, synced_ago, want, stub_exit="0"):
        """For each implementation: with no marker, and the op log's last
        sync synced_ago seconds ago (None for none), the prompt shows want.
        Returns the marker's mtimes, None where none was made."""
        with open(self.oplog, "w") as f:
            if synced_ago is not None:
                f.write(time.strftime("%Y%m%d%H%M.%S\n",
                                      time.localtime(time.time() - synced_ago)))
                f.write("197001010000.00\n")  # An older sync: not used.
        env = dict(self.path, JJ_STUB_EXIT=stub_exit)
        mtimes = []
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                if os.path.exists(self.marker):
                    os.remove(self.marker)
                before = self.op_log_calls()
                self.prompt(self.root, fmt="{project} {behind}", impls=[impl],
                            extra_env=env, want=want)
                self.assertEqual(self.op_log_calls(), before + 1)
                mtimes.append(os.stat(self.marker).st_mtime
                              if os.path.exists(self.marker) else None)
                # The next prompt asks jj again only if no marker was made.
                self.prompt(self.root, fmt="{project} {behind}", impls=[impl],
                            extra_env=env, want=want)
                self.assertEqual(self.op_log_calls(),
                                 before + (1 if mtimes[-1] is not None else 2))
        return mtimes

    def test_missing_marker_is_dated_from_the_last_sync(self):
        for mtime in self.seed(HOURS, "piper"):
            self.assertAlmostEqual(mtime, time.time() - HOURS, delta=120)

    def test_missing_marker_after_an_old_sync_reminds(self):
        self.seed(5 * HOURS, "piper pull")

    def test_missing_marker_and_no_sync_reminds(self):
        for mtime in self.seed(None, "piper pull"):
            self.assertLess(mtime, time.time() - 20 * 365 * 24 * HOURS)

    def test_failed_op_log_makes_no_marker(self):
        self.assertEqual(self.seed(HOURS, "piper pull", stub_exit="1"),
                         [None] * len(ALL_IMPLS))

    def test_failed_op_log_is_reported(self):
        env = dict(self.path, JJ_STUB_EXIT="1")
        for name, (rc, out, err) in self.each(
                ALL_IMPLS, ["prompt-info", "--color=never",
                            "--format={project} {behind}"], self.root, env):
            with self.subTest(impl=name):
                self.assertEqual((rc, out), (0, "piper pull\n"))
                self.assertIn("Error: stub", err)
                self.assertIn("vcs prompt-info: jj op log in piper failed, "
                              "so the last sync is unknown", err)

    def test_pull_records_the_sync(self):
        rc, out, err = self.e.vcs(ALL_IMPLS[0], ["-n", "pull"], self.root,
                                  self.path)
        self.assertEqual((rc, err), (0, "+ jj --no-pager sync\n"))
        self.assertFalse(os.path.exists(self.marker))
        rc, out, err = self.e.vcs(ALL_IMPLS[0], ["pull"], self.root,
                                  self.path)
        self.assertEqual(rc, 0, err)
        self.check("piper")


class UnmergedTest(Base):
    def test_unmerged_git(self):
        root = make_git(self.e)
        want = "+ git --no-pager branch -v --no-merged origin/main\n"
        for name, (rc, out, err) in self.each(ALL_IMPLS, ["-n", "unmerged"],
                                              root):
            with self.subTest(impl=name):
                self.assertEqual((rc, err), (0, want))


    def test_other_subcommands_go_to_python(self):
        root = make_git(self.e)
        for name, (rc, out, err) in self.each(SH_IMPLS, ["-n", "status"],
                                              root):
            with self.subTest(impl=name):
                self.assertEqual(
                    (rc, err),
                    (0, "+ git --no-pager status --short "
                        "--untracked-files=all\n"))


class AutoFetchTest(Base):
    """auto-fetch, run to completion (VCS_AUTO_FETCH_WAIT) so the tests can
    look at what the background fetch did."""

    def setUp(self):
        Base.setUp(self)
        self.root = make_git(self.e)
        # A fetch that can succeed offline.
        self.e.sh("git remote set-url origin %s" % self.e.path("proj-origin"),
                  self.root)
        self.fetch_head = os.path.join(self.root, ".git", "FETCH_HEAD")
        self.state = self.e.path("state")
        self.log = os.path.join(self.state, "vcs", "auto-fetch.log")
        self.env = {"VCS_AUTO_FETCH_WAIT": "1", "XDG_STATE_HOME": self.state}

    def fetch(self, impls, want, args=(), cwd=None, env=None):
        self.assertAll(impls, ["auto-fetch", "--verbose"] + list(args),
                       cwd or self.root, want + "\n",
                       extra_env=dict(self.env, **(env or {})))

    def test_outside_a_repo(self):
        self.fetch(ALL_IMPLS, "not-in-repo", cwd=self.e.tmp)

    def test_fresh_fetch_is_left_alone(self):
        self.fetch(ALL_IMPLS, "fresh")
        self.assertFalse(os.path.exists(self.log))

    def test_stale_fetch_is_refreshed_and_logged(self):
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                age(self.fetch_head, 2 * HOURS)
                self.fetch([impl], "fetched")
                self.assertLess(time.time() - os.stat(self.fetch_head).st_mtime,
                                60)
        with open(self.log) as f:
            log = f.read()
        self.assertEqual(log.count("proj: started git -C %s fetch --quiet"
                                   % self.root), len(ALL_IMPLS), log)
        self.assertEqual(log.count("proj: ended with exit status 0"),
                         len(ALL_IMPLS), log)

    def test_failed_fetch_is_logged(self):
        self.e.sh("git remote set-url origin %s" % self.e.path("nowhere"),
                  self.root)
        age(self.fetch_head, 2 * HOURS)
        self.fetch(ALL_IMPLS[:1], "fetched")
        with open(self.log) as f:
            log = f.read()
        self.assertIn("proj: ended with exit status 128", log)

    def test_max_age(self):
        age(self.fetch_head, 40 * 60)
        self.fetch(ALL_IMPLS, "fresh", ["--max-age", "1h"])
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                age(self.fetch_head, 40 * 60)
                self.fetch([impl], "fetched", ["--max-age=30m"])
        age(self.fetch_head, 40 * 60)
        self.fetch(ALL_IMPLS, "fresh", ["--max-age=0.75h"])
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                age(self.fetch_head, 30)
                self.fetch([impl], "fetched", ["--max-age=1s"])
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                age(self.fetch_head, 2)
                self.fetch([impl], "fetched", ["--max-age=0m"])

    def test_bad_flags(self):
        for args, err in ((["--max-age=soon"], "invalid --max-age"),
                          (["--max-age"], "--max-age needs a value"),
                          (["--bogus"], "unknown flag: --bogus")):
            for name, (rc, out, stderr) in self.each(
                    ALL_IMPLS, ["auto-fetch"] + args, self.root):
                with self.subTest(impl=name, args=args):
                    self.assertEqual(rc, 2)
                    self.assertIn(err, stderr)

    def test_dry_run_prints_the_fetch(self):
        age(self.fetch_head, 2 * HOURS)
        for name, (rc, out, err) in self.each(
                ALL_IMPLS, ["-n", "auto-fetch", "--verbose"], self.root):
            with self.subTest(impl=name):
                self.assertEqual((rc, out, err), (
                    0, "fetched\n",
                    "+ git -C %s fetch --quiet\n" % self.root))
        self.assertFalse(os.path.exists(self.log))

    def test_a_running_fetch_is_not_doubled(self):
        import fcntl
        age(self.fetch_head, 2 * HOURS)
        fd = os.open(os.path.join(self.root, ".git", "vcs-fetch.lock"),
                     os.O_CREAT | os.O_WRONLY)
        self.addCleanup(os.close, fd)
        fcntl.flock(fd, fcntl.LOCK_EX)
        # vcs-py says so; the sh front end has already handed over by then.
        self.fetch([ALL_IMPLS[-1]], "fresh")
        self.assertFalse(os.path.exists(self.log))

    def test_no_credential_dialogs(self):
        """An askpass program (here core.askPass) must not be asked: no one
        is there to answer it, and an open dialog holds the fetch lock."""
        import http.server
        import threading

        class Challenge(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(401)
                self.send_header("WWW-Authenticate", 'Basic realm="x"')
                self.send_header("Content-Length", "0")
                self.end_headers()

            def log_message(self, *args):
                pass  # Keep the test output clean.

        server = http.server.HTTPServer(("127.0.0.1", 0), Challenge)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        asked = self.e.path("asked")
        askpass = self.e.path("askpass")
        with open(askpass, "w") as f:
            f.write('#!/bin/sh\necho "$1" >> %s\necho x\n' % asked)
        os.chmod(askpass, 0o755)
        self.e.sh("git remote set-url origin http://127.0.0.1:%d/repo.git"
                  " && git config core.askPass %s"
                  % (server.server_port, askpass), self.root)
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                age(self.fetch_head, 2 * HOURS)
                self.fetch([impl], "fetched")
        self.assertFalse(os.path.exists(asked), "askpass was asked")
        with open(self.log) as f:
            log = f.read()
        self.assertEqual(log.count("proj: ended with exit status 128"),
                         len(ALL_IMPLS), log)

    def test_unsupported(self):
        p4 = self.e.path("depot")
        os.mkdir(p4)
        open(os.path.join(p4, ".p4config"), "w").close()
        self.fetch(ALL_IMPLS, "unsupported", cwd=p4)

    def test_piper_syncs_and_records_it(self):
        root = self.e.path("piper")
        os.makedirs(os.path.join(root, ".jj", "repo", "store"))
        with open(os.path.join(root, ".jj", "repo", "store", "type"),
                  "w") as f:
            f.write("piper\n")
        bindir = self.e.path("bin")
        os.mkdir(bindir)
        calls = self.e.path("calls")
        with open(os.path.join(bindir, "jj"), "w") as f:
            f.write('#!/bin/sh\necho "$*" >> %s\n' % calls)
        os.chmod(os.path.join(bindir, "jj"), 0o755)
        marker = os.path.join(root, ".jj", "vcs-synced")
        env = {"PATH": bindir + os.pathsep + os.environ["PATH"]}
        for impl in ALL_IMPLS:
            with self.subTest(impl=impl_name(impl)):
                open(marker, "w").close()
                age(marker, 2 * HOURS)
                self.fetch([impl], "fetched", cwd=root, env=env)
                self.assertLess(time.time() - os.stat(marker).st_mtime, 60)
                self.fetch([impl], "fresh", cwd=root, env=env)
        with open(calls) as f:
            self.assertEqual(f.read(), ("--repository %s piper pull\n" % root)
                             * len(ALL_IMPLS))


class PromptLineTest(Base):
    def test_line(self):
        root = make_git(self.e)
        self.assertAll([ALL_IMPLS[-1]],
                       ["prompt-line", "--hostname=box", "--shpool=s",
                        "--no-ssh", "--color=never"], root,
                       "box [s] proj main\n")
        self.assertAll([ALL_IMPLS[-1]],
                       ["prompt-line", "--hostname=box", "--color=never"],
                       self.e.tmp, "box shpool ~ SSH\n",
                       extra_env={"SHPOOL_SESSION_NAME": "",
                                  "SSH_AUTH_SOCK": ""})


def build_go(env):
    out = env.path("go-bin")
    os.mkdir(out)
    for b in ("vcs", "vcs-git", "vcs-hg", "vcs-jj", "vcs-p4"):
        subprocess.check_call(["go", "build", "-o", os.path.join(out, b),
                               "./cmd/" + b], cwd=REPO)
    return out


@unittest.skipUnless(shutil.which("go"), "go not installed")
class DryRunParityTest(Base):
    """Every subcommand prints, under --dry-run, what the Go vcs prints."""

    ARGS = ([], ["f1"], ["-m", "msg"], ["a", "b"], ["--reviewer=r", "x"])
    # Writes to the ignore file even under --dry-run, in both versions.
    SKIP = {"ignore"}

    def run_parity(self, root, extra_env=None):
        gobin = build_go(self.e)
        env = dict(extra_env or {})
        env["PATH"] = gobin + os.pathsep + os.environ["PATH"]
        with open(os.path.join(REPO, "commands.go")) as f:
            commands = re.findall(r'^\t"(\w+)",$', f.read(), re.M)
        cases = [(c, a) for c in commands + ["mv", "rm"]
                 if c not in self.SKIP for a in self.ARGS]

        def both(case):
            argv = ["-n", case[0]] + case[1]
            return (case,
                    self.e.vcs([os.path.join(gobin, "vcs")], argv, root, env),
                    self.e.vcs(ALL_IMPLS[0], argv, root, env))

        def norm(result):
            # Error text comes from different OS-error wrappers ("no such
            # file" in Go, "No such file" in Python); commands must match.
            rc, out, err = result
            return rc, out, "\n".join(
                line if line.startswith("+ ") else line.lower()
                for line in err.split("\n"))

        with concurrent.futures.ThreadPoolExecutor(8) as pool:
            for case, go, py in pool.map(both, cases):
                with self.subTest(case=case):
                    self.assertEqual(norm(py), norm(go))

    def test_git(self):
        self.run_parity(make_git(self.e))

    @unittest.skipUnless(shutil.which("hg"), "hg not installed")
    def test_hg(self):
        root = self.e.path("hgproj")
        os.mkdir(root)
        self.e.sh("hg init && echo a > a && hg add -q a && hg commit -qm one",
                  root)
        self.run_parity(root)

    @unittest.skipUnless(shutil.which("jj"), "jj not installed")
    def test_jj(self):
        make_git(self.e, "src")
        self.e.sh("jj git clone %s jjproj" % self.e.path("src-origin"),
                  self.e.tmp)
        self.run_parity(self.e.path("jjproj"))


class LintTest(unittest.TestCase):
    @unittest.skipUnless(shutil.which("shellcheck"), "shellcheck missing")
    def test_shellcheck(self):
        subprocess.check_call(["shellcheck", "-s", "sh", VCS_SH])


if __name__ == "__main__":
    unittest.main()
