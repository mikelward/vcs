//! Subprocess execution helpers shared by the `vcs-*` binaries.
//!
//! Mirrors the Go `runner` package: inherits stdio, supports a process-wide
//! dry-run mode toggled at startup from `VCS_DRY_RUN`, and exposes
//! [`exit_code`] for translating a child's `ExitStatus` into a numeric
//! exit code suitable for `process::exit`.

use std::io::{self, Write};
use std::process::{Command, ExitStatus};
use std::sync::atomic::{AtomicBool, Ordering};

static DRY_RUN: AtomicBool = AtomicBool::new(false);

/// Returns whether dry-run mode is enabled.
pub fn dry_run() -> bool {
    DRY_RUN.load(Ordering::Relaxed)
}

/// Enables or disables dry-run mode for subsequent [`run`] calls.
pub fn set_dry_run(v: bool) {
    DRY_RUN.store(v, Ordering::Relaxed);
}

/// Initialises dry-run from the `VCS_DRY_RUN` environment variable. Called
/// once from `main` so tests that set the env after process start aren't
/// affected.
pub fn init_from_env() {
    if std::env::var_os("VCS_DRY_RUN").is_some_and(|v| !v.is_empty()) {
        set_dry_run(true);
    }
}

/// Either a successful exit status from a child process or a failure to
/// spawn it. The error variant carries the program name so [`exit_code`]
/// can map it to a conventional exit code without a separate context arg.
#[derive(Debug)]
pub enum RunError {
    Spawn { program: String, err: io::Error },
    NonZero(ExitStatus),
}

impl std::fmt::Display for RunError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            RunError::Spawn { program, err } => write!(f, "{program}: {err}"),
            RunError::NonZero(s) => write!(f, "exit status {s}"),
        }
    }
}

impl std::error::Error for RunError {}

/// Runs `program` with `args`, inheriting stdin/stdout/stderr. In dry-run
/// mode prints the command to stderr and returns `Ok(())` without executing.
pub fn run<I, S>(program: &str, args: I) -> Result<(), RunError>
where
    I: IntoIterator<Item = S>,
    S: AsRef<str>,
{
    let args: Vec<String> = args.into_iter().map(|s| s.as_ref().to_owned()).collect();
    if dry_run() {
        print_command(program, &args);
        return Ok(());
    }
    let status = Command::new(program)
        .args(&args)
        .status()
        .map_err(|err| RunError::Spawn {
            program: program.to_owned(),
            err,
        })?;
    if status.success() {
        Ok(())
    } else {
        Err(RunError::NonZero(status))
    }
}

/// Maps a [`Result`] from [`run`] (or any equivalent) into the conventional
/// process exit code. `Ok` is 0, a non-zero exit propagates the child's code,
/// and a spawn error (or signal death) is 1.
pub fn exit_code(result: &Result<(), RunError>) -> i32 {
    match result {
        Ok(()) => 0,
        Err(RunError::NonZero(status)) => status.code().unwrap_or(1),
        Err(RunError::Spawn { .. }) => 1,
    }
}

/// Writes a shell-quoted `+ program args...` line to stderr — the same
/// `set -x` style trace the Go runner prints in dry-run mode.
pub fn print_command(program: &str, args: &[String]) {
    let mut line = String::from("+ ");
    line.push_str(&shell_quote(program));
    for a in args {
        line.push(' ');
        line.push_str(&shell_quote(a));
    }
    let stderr = io::stderr();
    let mut h = stderr.lock();
    let _ = writeln!(h, "{line}");
}

fn shell_quote(s: &str) -> String {
    if s.is_empty() {
        return "''".to_owned();
    }
    if s.chars().all(safe_char) {
        return s.to_owned();
    }
    let mut out = String::with_capacity(s.len() + 2);
    out.push('\'');
    for c in s.chars() {
        if c == '\'' {
            out.push_str("'\\''");
        } else {
            out.push(c);
        }
    }
    out.push('\'');
    out
}

fn safe_char(c: char) -> bool {
    matches!(c, 'a'..='z' | 'A'..='Z' | '0'..='9'
        | '/' | '.' | '_' | '-' | ':' | '=' | ',' | '@' | '+' | '%')
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn shell_quote_safe() {
        assert_eq!(shell_quote("status"), "status");
        assert_eq!(shell_quote("foo-bar.txt"), "foo-bar.txt");
    }

    #[test]
    fn shell_quote_unsafe() {
        assert_eq!(shell_quote(""), "''");
        assert_eq!(shell_quote("a b"), "'a b'");
        assert_eq!(shell_quote("it's"), "'it'\\''s'");
    }

    #[test]
    fn exit_code_ok() {
        assert_eq!(exit_code(&Ok(())), 0);
    }
}
