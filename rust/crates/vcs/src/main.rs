//! `vcs` detects the VCS for the current directory and dispatches to the
//! appropriate `vcs-*` binary via `execvp`. This slice supports the
//! happy-path subset of flags needed to route `vcs status` through to
//! `vcs-git`.

use std::env;
use std::ffi::CString;
#[cfg(unix)]
use std::os::unix::ffi::OsStrExt;
use std::path::PathBuf;
use std::process::ExitCode;

fn main() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();

    let mut force_vcs: Option<String> = None;
    let mut dry_run = false;
    let mut i = 0;
    while i < args.len() {
        let a = &args[i];
        if let Some(v) = a.strip_prefix("--vcs=") {
            force_vcs = Some(v.to_owned());
            i += 1;
        } else if a == "--vcs" && i + 1 < args.len() {
            force_vcs = Some(args[i + 1].clone());
            i += 2;
        } else if matches!(a.as_str(), "-n" | "--dry-run" | "--simulate") {
            dry_run = true;
            i += 1;
        } else {
            break;
        }
    }
    let rest = &args[i..];

    if dry_run {
        // Child binaries read this at startup the same way the Go ones do.
        env::set_var("VCS_DRY_RUN", "1");
    }

    if rest.is_empty() {
        eprintln!("usage: vcs [--vcs=NAME] [-n|--dry-run] <subcommand> [args...]");
        return ExitCode::from(1);
    }

    let cwd = match env::current_dir() {
        Ok(p) => p,
        Err(err) => {
            eprintln!("vcs: cannot get working directory: {err}");
            return ExitCode::from(1);
        }
    };

    let info = match force_vcs {
        Some(v) => vcsdetect::Info {
            vcs: v,
            root_dir: cwd.clone(),
        },
        None => match vcsdetect::detect(&cwd) {
            Ok(info) => info,
            Err(_) => {
                eprintln!("vcs: no version control system detected");
                return ExitCode::from(1);
            }
        },
    };

    let binary = format!("vcs-{}", info.vcs);
    let Some(path) = which(&binary) else {
        eprintln!("vcs: {binary} not found on PATH");
        return ExitCode::from(1);
    };

    let mut argv: Vec<CString> = Vec::with_capacity(rest.len() + 1);
    #[cfg(unix)]
    argv.push(CString::new(path.as_os_str().as_bytes()).expect("path has NUL"));
    #[cfg(not(unix))]
    argv.push(CString::new(path.to_string_lossy().as_bytes()).expect("path has NUL"));
    for a in rest {
        argv.push(CString::new(a.as_bytes()).expect("arg has NUL"));
    }
    exec(&path, &argv)
}

#[cfg(unix)]
fn exec(path: &std::path::Path, argv: &[CString]) -> ExitCode {
    let c_path = CString::new(path.as_os_str().as_bytes()).expect("path has NUL");
    let mut ptrs: Vec<*const libc::c_char> = argv.iter().map(|s| s.as_ptr()).collect();
    ptrs.push(std::ptr::null());
    // SAFETY: execvp expects a NUL-terminated argv array of NUL-terminated
    // C strings. We construct both above; on success this never returns.
    unsafe {
        libc::execvp(c_path.as_ptr(), ptrs.as_ptr());
    }
    let err = std::io::Error::last_os_error();
    eprintln!("vcs: exec {}: {err}", path.display());
    ExitCode::from(1)
}

#[cfg(not(unix))]
fn exec(path: &std::path::Path, argv: &[CString]) -> ExitCode {
    // Non-Unix fallback: spawn and forward exit code.
    let args: Vec<String> = argv
        .iter()
        .skip(1)
        .map(|c| c.to_string_lossy().into_owned())
        .collect();
    match std::process::Command::new(path).args(&args).status() {
        Ok(status) => ExitCode::from(status.code().unwrap_or(1) as u8),
        Err(err) => {
            eprintln!("vcs: exec {}: {err}", path.display());
            ExitCode::from(1)
        }
    }
}

fn which(name: &str) -> Option<PathBuf> {
    let path = env::var_os("PATH")?;
    for dir in env::split_paths(&path) {
        let candidate = dir.join(name);
        if let Ok(meta) = std::fs::metadata(&candidate) {
            if meta.is_file() {
                return Some(candidate);
            }
        }
    }
    None
}
