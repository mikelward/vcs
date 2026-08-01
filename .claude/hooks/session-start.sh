#!/bin/bash
# Install the VCS backends the test suite drives. Without them `go test ./...`
# still reports ok while 34 tests in cmd/ skip themselves — a green run that
# never exercised vcs-hg or vcs-jj at all.
#
# Versions and install steps mirror .github/workflows/ci.yml exactly. Keep them
# in sync: a web session testing against a different jj than CI turns a real
# regression into a local-only mystery, or hides one. `jj revert`, for
# instance, does not exist before 0.31 — an older pin fails a passing test.
#
# Anything already on PATH is left alone; the hook re-runs on every
# SessionStart and an environment that ships its own build is not ours to
# overwrite. A version that differs from the pin is reported and kept, because
# that difference is exactly what explains a test passing here and failing in
# CI — it has to be visible rather than silently corrected.
set -euo pipefail

# Local checkouts already have their tools; only the ephemeral remote container
# starts bare.
if test "${CLAUDE_CODE_REMOTE:-}" != "true"; then
    exit 0
fi

HG_VERSION=7.2.3
JJ_VERSION=0.43.0
JJ_SHA256=59e5588583ac82b623239929368c65b90735931c0f26b5a16c1f04d5bb97643d

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# This runs before the session is usable, so an unbounded download holds up
# startup with nothing on screen to say why. A GitHub outage should fail in a
# few minutes with a message, not hang until something upstream gives up.
curl_opts=(--fail --silent --show-error --location
           --connect-timeout 10 --max-time 300)

install_jj() {
    tarball="jj-v${JJ_VERSION}-x86_64-unknown-linux-musl.tar.gz"
    url="https://github.com/jj-vcs/jj/releases/download/v${JJ_VERSION}/${tarball}"
    curl "${curl_opts[@]}" -o "$tmp/$tarball" "$url" || return 1
    # Same checksum CI enforces: a substituted tarball should fail loudly here.
    (cd "$tmp" && echo "${JJ_SHA256}  ${tarball}" | sha256sum --check --strict -) || return 1
    mkdir -p "$tmp/jj-dist"
    tar xzf "$tmp/$tarball" -C "$tmp/jj-dist" || return 1
    # Prove it runs before publishing it. Once a broken file is in
    # /usr/local/bin, `command -v jj` succeeds forever after: this hook stops
    # retrying, and the suite runs the jj tests against a binary that cannot
    # execute rather than skipping them cleanly.
    "$tmp/jj-dist/jj" --version >/dev/null 2>&1 || return 1
    install -m 755 "$tmp/jj-dist/jj" /usr/local/bin/jj || return 1
}

# Print what an already-installed backend reports and move on. No comparison
# against the pin: the point is that the version testing actually ran against
# is on the record, so a result that disagrees with CI has an explanation
# sitting in the session log rather than needing to be rediscovered.
report_version() {
    echo "session-start: using the installed $1: $2" >&2
}

# Runs the backend for real, so one that is present but cannot execute fails
# here instead of being accepted on the strength of `command -v` alone. Its
# stderr is left alone: a probe that dies is exactly what needs explaining.
probe() {
    "$1" --version | head -1
}

# Neither backend is fatal: the suite still passes without them, just thinner.
# Report the failure so a skipped backend is not mistaken for a passing one.
# Every branch reports a version, including after an install — otherwise a
# first run, the case this hook exists for, is the one with nothing in its log.
if command -v hg >/dev/null 2>&1; then
    if _version=$(probe hg); then
        report_version mercurial "$_version"
    else
        echo "session-start: hg is on PATH but will not run; vcs-hg tests will run against a broken binary" >&2
    fi
elif pip3 install --quiet --break-system-packages "mercurial==${HG_VERSION}"; then
    if _version=$(probe hg); then
        report_version mercurial "$_version"
    else
        echo "session-start: the installed hg does not run; vcs-hg tests will skip" >&2
    fi
else
    echo "session-start: mercurial install failed; vcs-hg tests will skip" >&2
fi

if command -v jj >/dev/null 2>&1; then
    if _version=$(probe jj); then
        report_version jj "$_version"
    else
        echo "session-start: jj is on PATH but will not run; vcs-jj tests will run against a broken binary" >&2
    fi
elif install_jj; then
    if _version=$(probe jj); then
        report_version jj "$_version"
    else
        echo "session-start: the installed jj does not run; vcs-jj tests will skip" >&2
    fi
else
    echo "session-start: jj install failed; vcs-jj tests will skip" >&2
fi
