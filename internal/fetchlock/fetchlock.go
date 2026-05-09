// Package fetchlock serialises concurrent git fetches within a repo
// using an advisory flock on vcs-fetch.lock so that a background
// auto-fetch and an interactive vcs pull never write FETCH_HEAD
// concurrently (which produces "Cannot rebase onto multiple branches").
package fetchlock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const lockName = "vcs-fetch.lock"

// ErrLocked is returned by TryLock when the lock is already held.
var ErrLocked = errors.New("fetch lock held by another process")

// TryLock attempts a non-blocking exclusive lock on vcs-fetch.lock in
// gitDir. Returns ErrLocked immediately if another process holds it.
// On success the caller must either Close the file (to release the lock)
// or pass it to the child process via cmd.ExtraFiles — the OS releases
// the lock when all file descriptors referencing the open-file-description
// are closed, so the child holds the lock until it exits.
func TryLock(gitDir string) (*os.File, error) {
	return openAndLock(gitDir, syscall.LOCK_EX|syscall.LOCK_NB)
}

// Lock acquires a blocking exclusive lock on vcs-fetch.lock in gitDir,
// waiting until any current holder releases it. The caller must Close
// the returned file when done.
func Lock(gitDir string) (*os.File, error) {
	return openAndLock(gitDir, syscall.LOCK_EX)
}

func openAndLock(gitDir string, how int) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(gitDir, lockName), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if how&syscall.LOCK_NB != 0 && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return f, nil
}
