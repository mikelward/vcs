//! Integration test mirroring `TestStatus` from `cmd/vcs-git/commands_test.go`:
//!
//! 1. Stage a new file — `status` shows `A s.txt`.
//! 2. Add an untracked file — `status` shows `?? untracked.txt`.
//! 3. Commit everything — `status` is clean.
//!
//! Drives the binary end-to-end via `cargo run`'s output binary path so we
//! exercise the real argv path through `dispatch` → `runner::run` → `git`.

use std::path::{Path, PathBuf};
use std::process::Command;

fn bin_path() -> PathBuf {
    // Cargo sets CARGO_BIN_EXE_<name> for integration tests of bin crates.
    PathBuf::from(env!("CARGO_BIN_EXE_vcs-git"))
}

fn git(dir: &Path, args: &[&str]) {
    let out = Command::new("git")
        .args(args)
        .current_dir(dir)
        .output()
        .expect("spawn git");
    assert!(
        out.status.success(),
        "git {args:?} failed: {}",
        String::from_utf8_lossy(&out.stderr)
    );
}

fn run_status(dir: &Path) -> String {
    let out = Command::new(bin_path())
        .arg("status")
        .current_dir(dir)
        .output()
        .expect("spawn vcs-git");
    assert!(
        out.status.success(),
        "vcs-git status failed: {}",
        String::from_utf8_lossy(&out.stderr)
    );
    String::from_utf8(out.stdout).expect("utf8")
}

fn tmpdir() -> tempdir::TempDir {
    tempdir::TempDir::new()
}

#[test]
fn status_reflects_staged_untracked_and_clean() {
    let tmp = tmpdir();
    let local = tmp.path();

    git(local, &["init", "-q", "-b", "main"]);
    git(local, &["config", "commit.gpgsign", "false"]);
    git(local, &["config", "user.email", "test@example.com"]);
    git(local, &["config", "user.name", "Test User"]);
    git(local, &["commit", "--allow-empty", "-q", "-m", "initial"]);

    std::fs::write(local.join("s.txt"), b"x").unwrap();
    git(local, &["add", "s.txt"]);
    let out = run_status(local);
    assert!(out.contains("s.txt"), "status missing file: {out:?}");
    assert!(out.contains('A'), "status missing A prefix: {out:?}");

    std::fs::write(local.join("untracked.txt"), b"u").unwrap();
    let out = run_status(local);
    assert!(
        out.contains("untracked.txt"),
        "status missing untracked: {out:?}"
    );
    assert!(out.contains("??"), "status missing ?? prefix: {out:?}");

    git(local, &["add", "-A"]);
    git(local, &["commit", "-q", "-m", "cleanup"]);
    let out = run_status(local);
    assert!(out.trim().is_empty(), "status clean: {out:?}");
}

// Minimal tempdir helper — keeps the crate dep-free aside from libc on
// vcs's side. Used only by tests.
mod tempdir {
    use std::path::{Path, PathBuf};

    pub struct TempDir(PathBuf);

    impl TempDir {
        pub fn new() -> Self {
            let p = std::env::temp_dir().join(format!(
                "vcs-git-test-{}-{}",
                std::process::id(),
                nanos()
            ));
            std::fs::create_dir_all(&p).expect("mkdir tempdir");
            Self(p)
        }
        pub fn path(&self) -> &Path {
            &self.0
        }
    }

    impl Drop for TempDir {
        fn drop(&mut self) {
            let _ = std::fs::remove_dir_all(&self.0);
        }
    }

    fn nanos() -> u128 {
        use std::time::{SystemTime, UNIX_EPOCH};
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_nanos())
            .unwrap_or(0)
    }
}
