#!/bin/sh
#
# Tests for .claude/hooks/session-start.sh, the SessionStart hook that installs
# the VCS backends the suite drives.
#
# The hook installs to /usr/local/bin and needs the network, so no case here
# lets it do either for real: each puts stubs on a hermetic PATH and checks the
# hook's decisions. The PATH deliberately excludes /usr/bin — a test PATH that
# keeps it would find the real hg and jj, and every "when it is missing" case
# would silently assert the already-installed branch instead.
#

_hook="$(dirname "$0")/.claude/hooks/session-start.sh"
_failures=0
_passes=0

_check() {
    if test "$2" = "$3"; then
        _passes=$((_passes + 1))
    else
        _failures=$((_failures + 1))
        printf 'FAIL: %s\n  expected: %s\n  actual:   %s\n' "$1" "$2" "$3"
    fi
}

_contains() {
    case "$3" in
        *"$2"*) _passes=$((_passes + 1)) ;;
        *)
            _failures=$((_failures + 1))
            printf 'FAIL: %s\n  expected to contain: %s\n  actual:              %s\n' "$1" "$2" "$3"
            ;;
    esac
}

_absent() {
    case "$3" in
        *"$2"*)
            _failures=$((_failures + 1))
            printf 'FAIL: %s\n  expected NOT to contain: %s\n  actual:                  %s\n' "$1" "$2" "$3"
            ;;
        *) _passes=$((_passes + 1)) ;;
    esac
}

if ! test -x "$_hook"; then
    echo "SKIP: session-start.sh missing or not executable"
    exit 0
fi

# "$1" is the exit status curl should report. Stubs log every call so a case
# can assert what the hook did and did not reach.
_stub_dir() {
    _dir=$(mktemp -d)
    # Symlink only what stays real. A name that is also stubbed below must not
    # be linked first: the stub is written with `>`, which follows the symlink
    # and overwrites the system binary it points at.
    for _real in mktemp mkdir rm head cat chmod; do
        if _path=$(command -v "$_real" 2>/dev/null); then
            ln -sf "$_path" "$_dir/$_real"
        fi
    done

    printf '#!/bin/sh\necho "curl $*" >>"%s/calls"\nexit %s\n' "$_dir" "$1" >"$_dir/curl"
    # A real pip install leaves a runnable hg, and a real `install` leaves the
    # binary at its destination — the hook probes both afterwards, so the stubs
    # have to produce something runnable rather than just report success.
    cat >"$_dir/pip3" <<EOF
#!/bin/sh
echo "pip3 \$*" >>"$_dir/calls"
printf '#!/bin/sh\necho "Mercurial Distributed SCM (version 9.9.9)"\n' >"$_dir/hg"
chmod +x "$_dir/hg"
exit 0
EOF
    cat >"$_dir/install" <<EOF
#!/bin/sh
echo "install \$*" >>"$_dir/calls"
for _a in "\$@"; do _dest=\$_a; done
printf '#!/bin/sh\necho "jj 9.9.9"\n' >"$_dir/\${_dest##*/}"
chmod +x "$_dir/\${_dest##*/}"
exit 0
EOF
    # The stubbed curl writes nothing, so the real sha256sum would fail every
    # happy-path case. The checksum case overrides this with a failing stub.
    printf '#!/bin/sh\necho "sha256sum $*" >>"%s/calls"\nexit 0\n' "$_dir" >"$_dir/sha256sum"
    # A real extraction leaves a runnable binary, which the hook probes before
    # publishing anything — so the stub has to produce one.
    cat >"$_dir/tar" <<EOF
#!/bin/sh
echo "tar \$*" >>"$_dir/calls"
_out=
while test \$# -gt 0; do
    if test "\$1" = "-C"; then shift; _out=\$1; fi
    shift
done
test -n "\$_out" || exit 0
mkdir -p "\$_out"
printf '#!/bin/sh\necho "jj 9.9.9"\n' >"\$_out/jj"
chmod +x "\$_out/jj"
exit 0
EOF
    chmod +x "$_dir/curl" "$_dir/pip3" "$_dir/install" "$_dir/tar" "$_dir/sha256sum"
    printf '%s\n' "$_dir"
}

# Only the ephemeral remote container starts bare. Getting this wrong means
# writing to /usr/local/bin on a developer's own machine.
_stubs=$(_stub_dir 0)
CLAUDE_CODE_REMOTE='' PATH="$_stubs" "$_hook" >/dev/null 2>&1
_check "does nothing when not in a remote container" 0 $?
_check "makes no calls when not in a remote container" "" "$(cat "$_stubs/calls" 2>/dev/null)"
rm -rf "$_stubs"

# Re-running a session must not reinstall what is already there — the hook
# fires on every SessionStart.
_stubs=$(_stub_dir 0)
for _t in hg jj; do
    printf '#!/bin/sh\necho "%s 1.2.3"\n' "$_t" >"$_stubs/$_t"
    chmod +x "$_stubs/$_t"
done
_err=$(CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" 2>&1 >/dev/null)
_check "exits cleanly when both backends are present" 0 $?
_check "installs nothing when both backends are present" "" "$(cat "$_stubs/calls" 2>/dev/null)"
_contains "reports the installed hg version" "1.2.3" "$_err"
rm -rf "$_stubs"

# The version the tests actually ran against belongs in the log on every path,
# including a first run — which is the case this hook exists for.
_stubs=$(_stub_dir 0)
_err=$(CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" 2>&1 >/dev/null)
_contains "reports the jj version after installing it" "installed jj: jj 9.9.9" "$_err"
rm -rf "$_stubs"

# `command -v` proves a name resolves, not that the file runs. A backend that
# cannot execute makes the suite run its tests against a broken binary rather
# than skipping them, so it has to be called out.
_stubs=$(_stub_dir 0)
for _t in hg jj; do
    printf '#!/bin/sh\nexit 127\n' >"$_stubs/$_t"
    chmod +x "$_stubs/$_t"
done
_err=$(CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" 2>&1 >/dev/null)
# Named per backend: with both broken, a bare "will not run" match is
# satisfied by either one and says nothing about the other.
_contains "reports an hg that is on PATH but will not run" "hg is on PATH but will not run" "$_err"
_contains "reports a jj that is on PATH but will not run" "jj is on PATH but will not run" "$_err"
rm -rf "$_stubs"

# Publishing a binary that cannot execute is worse than installing nothing:
# `command -v jj` then succeeds forever, so the hook never retries and the
# suite runs the jj tests against the dead file instead of skipping them.
_stubs=$(_stub_dir 0)
printf '#!/bin/sh\necho "hg 1.2.3"\n' >"$_stubs/hg"
chmod +x "$_stubs/hg"
cat >"$_stubs/tar" <<EOF
#!/bin/sh
echo "tar \$*" >>"$_stubs/calls"
_out=
while test \$# -gt 0; do
    if test "\$1" = "-C"; then shift; _out=\$1; fi
    shift
done
test -n "\$_out" || exit 0
mkdir -p "\$_out"
printf '#!/bin/sh\nexit 127\n' >"\$_out/jj"
chmod +x "\$_out/jj"
exit 0
EOF
chmod +x "$_stubs/tar"
_err=$(CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" 2>&1 >/dev/null)
_contains "an unrunnable jj is reported rather than installed" "jj install failed" "$_err"
_absent "an unrunnable jj never reaches install" "install " "$(cat "$_stubs/calls" 2>/dev/null)"
rm -rf "$_stubs"

# A substituted tarball must never be extracted, let alone installed: this
# runs before the session is usable and the result goes on PATH.
_stubs=$(_stub_dir 0)
printf '#!/bin/sh\necho "sha256sum $*" >>"%s/calls"\nexit 1\n' "$_stubs" >"$_stubs/sha256sum"
printf '#!/bin/sh\necho "hg 1.2.3"\n' >"$_stubs/hg"
chmod +x "$_stubs/sha256sum" "$_stubs/hg"
_err=$(CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" 2>&1 >/dev/null)
_check "a jj checksum mismatch is reported" 0 $?
_contains "names jj when its install fails" "jj install failed" "$_err"
_absent "a tampered tarball never reaches install" "install " "$(cat "$_stubs/calls" 2>/dev/null)"
rm -rf "$_stubs"

# A stalled download would otherwise hold up session startup with nothing on
# screen to say why.
_stubs=$(_stub_dir 0)
printf '#!/bin/sh\necho "hg 1.2.3"\n' >"$_stubs/hg"
chmod +x "$_stubs/hg"
CLAUDE_CODE_REMOTE=true PATH="$_stubs" "$_hook" >/dev/null 2>&1
_check "the hook succeeds on the install path" 0 $?
_calls=$(cat "$_stubs/calls" 2>/dev/null)
_contains "bounds the connection" "--connect-timeout" "$_calls"
_contains "bounds the transfer" "--max-time" "$_calls"
rm -rf "$_stubs"

if test $_failures -gt 0; then
    echo "session-start hook: $_failures test(s) failed, $_passes passed."
    exit 1
fi
echo "session-start hook: all $_passes tests passed."
