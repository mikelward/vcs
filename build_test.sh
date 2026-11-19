#!/bin/sh
#
# Tests for the Makefile's build command: binaries must not embed the
# directory they were built in (usually under the user's home), two
# builds of the same source must be byte-identical, and a change to the
# build command must rebuild existing binaries.
#
# `make test` passes the Makefile's GOBUILD in the environment, so this
# checks the flags `make` really uses.
#

: "${GOBUILD:?run from make test, which sets GOBUILD}"

_failures=0
_passes=0

_tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$_tmp"' EXIT

_srcdir=$(pwd -P)

# Build from two different directories so a leaked path would also make
# the outputs differ.
cp -R "$_srcdir" "$_tmp/a"
cp -R "$_srcdir" "$_tmp/b"
for _d in a b; do
    if ! (cd "$_tmp/$_d" && $GOBUILD -o "$_tmp/vcs-$_d" ./cmd/vcs); then
        printf 'FAIL: %s build failed\n' "$_d"
        exit 1
    fi
done

if grep -q -F "$_tmp" "$_tmp/vcs-a"; then
    _failures=$((_failures + 1))
    printf 'FAIL: binary contains its build directory\n'
else
    _passes=$((_passes + 1))
fi

if cmp -s "$_tmp/vcs-a" "$_tmp/vcs-b"; then
    _passes=$((_passes + 1))
else
    _failures=$((_failures + 1))
    printf 'FAIL: builds from different directories differ\n'
fi

# Binaries built before a change to the build command must be rebuilt:
# with sources older than the binary and the Makefile newer, make must
# consider the binary out of date.
mkdir "$_tmp/c"
cp -R "$_srcdir/." "$_tmp/c"
find "$_tmp/c" -name '*.go' -exec touch -t 200001010000 {} +
for _b in vcs vcs-git vcs-hg vcs-jj vcs-p4; do
    : >"$_tmp/c/$_b"
    touch -t 200101010000 "$_tmp/c/$_b"
    if make -q -C "$_tmp/c" "$_b" >/dev/null 2>&1; then
        _failures=$((_failures + 1))
        printf 'FAIL: %s is not rebuilt when the Makefile changes\n' "$_b"
    else
        _passes=$((_passes + 1))
    fi
done

printf 'build_test: %d passed, %d failed\n' "$_passes" "$_failures"
test "$_failures" -eq 0
