//! `vcs-git` translates unified VCS subcommands into git commands.
//!
//! This is the Rust slice's analog of `cmd/vcs-git`. Only the `status`
//! subcommand is wired up; the dispatch table is structured so adding
//! more is a one-line affair, matching the Go shape.

use std::process::ExitCode;

use runner::{exit_code, init_from_env, run, set_dry_run, RunError};

fn main() -> ExitCode {
    init_from_env();

    let mut args: Vec<String> = std::env::args().skip(1).collect();
    while args.first().is_some_and(|a| is_dry_run_flag(a)) {
        set_dry_run(true);
        args.remove(0);
    }

    let Some(subcmd) = args.first().cloned() else {
        eprintln!("usage: vcs-git [-n|--dry-run] <subcommand> [args...]");
        return ExitCode::from(1);
    };
    let rest: Vec<String> = args.into_iter().skip(1).collect();

    let result = dispatch(&subcmd, &rest);
    if let Err(RunError::Spawn { program, err }) = &result {
        eprintln!("vcs-git: {program}: {err}");
    }
    ExitCode::from(exit_code(&result) as u8)
}

fn is_dry_run_flag(a: &str) -> bool {
    matches!(a, "-n" | "--dry-run" | "--simulate")
}

fn dispatch(subcmd: &str, args: &[String]) -> Result<(), RunError> {
    match subcmd {
        "status" => git(
            "status",
            ["--short", "--untracked-files=all"]
                .into_iter()
                .map(String::from)
                .chain(args.iter().cloned()),
        ),
        other => Err(RunError::Spawn {
            program: "vcs-git".into(),
            err: std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("unknown git subcommand: {other}"),
            ),
        }),
    }
}

/// Runs `git --no-pager <cmd> <args...>`. Matches the Go `git()` helper: a
/// pager would break scriptable output across backends.
fn git<I, S>(cmd: &str, args: I) -> Result<(), RunError>
where
    I: IntoIterator<Item = S>,
    S: AsRef<str> + Into<String>,
{
    let mut full: Vec<String> = vec!["--no-pager".into(), cmd.into()];
    full.extend(args.into_iter().map(Into::into));
    run("git", full)
}
