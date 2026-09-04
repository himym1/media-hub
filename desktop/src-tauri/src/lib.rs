use std::path::{Path, PathBuf};
use std::process::Child;
use std::time::Duration;

mod browser;
mod player;

pub(crate) fn is_supported_playback_url(url: &str) -> bool {
    if url.contains('\n') || url.contains('\r') || url.contains('\0') {
        return false;
    }
    url.starts_with("https://") || url.starts_with("http://")
}

pub(crate) fn sanitized_user_agent(value: &str) -> Option<&str> {
    let trimmed = value.trim();
    if trimmed.is_empty() || trimmed.len() > 512 {
        return None;
    }
    if trimmed.bytes().any(|byte| byte < 32 || byte == 127) {
        return None;
    }
    Some(trimmed)
}

fn mpv_binary_name() -> &'static str {
    if cfg!(windows) {
        "mpv.exe"
    } else {
        "mpv"
    }
}

fn extra_mpv_dirs() -> Vec<PathBuf> {
    let mut dirs = Vec::new();
    if cfg!(windows) {
        if let Ok(local) = std::env::var("LOCALAPPDATA") {
            dirs.push(PathBuf::from(local).join(r"Microsoft\WinGet\Links"));
        }
        if let Ok(home) = std::env::var("USERPROFILE") {
            dirs.push(PathBuf::from(&home).join(r"scoop\shims"));
            dirs.push(PathBuf::from(home).join(r"scoop\apps\mpv\current"));
        }
        if let Ok(program_files) = std::env::var("ProgramFiles") {
            dirs.push(PathBuf::from(program_files).join("mpv"));
        }
        dirs.push(PathBuf::from(r"C:\Program Files\mpv"));
        dirs.push(PathBuf::from(r"C:\Program Files (x86)\mpv"));
        dirs.push(PathBuf::from(r"C:\ProgramData\chocolatey\bin"));
    } else {
        dirs.push(PathBuf::from("/opt/homebrew/bin"));
        dirs.push(PathBuf::from("/usr/local/bin"));
        dirs.push(PathBuf::from("/usr/bin"));
        if let Ok(home) = std::env::var("HOME") {
            dirs.push(PathBuf::from(home).join(".local/bin"));
        }
    }
    dirs
}

pub(crate) fn resolve_mpv() -> Option<PathBuf> {
    let name = mpv_binary_name();
    if let Some(found) = find_on_path(name) {
        return Some(found);
    }
    extra_mpv_dirs()
        .into_iter()
        .map(|dir| dir.join(name))
        .find(|candidate| candidate.is_file())
}

fn find_on_path(name: &str) -> Option<PathBuf> {
    let path_var = std::env::var_os("PATH")?;
    for dir in std::env::split_paths(&path_var) {
        let candidate = dir.join(name);
        if candidate.is_file() {
            return Some(candidate);
        }
    }
    None
}

pub(crate) fn command_path_with_extras() -> Option<std::ffi::OsString> {
    let mut dirs = extra_mpv_dirs();
    if let Ok(path) = std::env::var("PATH") {
        for dir in std::env::split_paths(&path) {
            dirs.push(dir);
        }
    }
    std::env::join_paths(dirs).ok()
}

pub(crate) fn wait_child_started(child: &mut Child, linger: Duration) -> Result<(), String> {
    std::thread::sleep(linger);
    match child.try_wait() {
        Ok(Some(_)) => Err("mpv 未能打开这路流。".into()),
        Ok(None) => Ok(()),
        Err(_) => Err("无法确认 mpv 是否已启动。".into()),
    }
}

pub(crate) fn mpv_args(
    title: &str,
    start_position_ms: u64,
    user_agent: Option<&str>,
    wid: Option<i64>,
    ipc: Option<&Path>,
    input_conf: Option<&Path>,
) -> Vec<String> {
    let mut args = vec![
        "--force-window=yes".to_string(),
        "--keep-open=no".to_string(),
        "--ytdl=no".to_string(),
        "--osc=no".to_string(),
        "--no-border".to_string(),
        "--focus-on=never".to_string(),
        format!("--title={}", title.replace(['\n', '\r'], " ")),
        "--no-terminal".to_string(),
    ];
    if let Some(agent) = user_agent.and_then(sanitized_user_agent) {
        // Only --user-agent. --http-header-fields is a comma list and would
        // split a normal Mozilla UA into bogus headers, so 115 rejects the URL.
        args.push(format!("--user-agent={agent}"));
    }
    if let Some(wid) = wid {
        args.push(format!("--wid={wid}"));
    }
    if let Some(ipc) = ipc {
        args.push(format!("--input-ipc-server={}", ipc.display()));
    }
    if let Some(input_conf) = input_conf {
        args.push(format!("--input-conf={}", input_conf.display()));
    }
    if start_position_ms > 0 {
        args.push(format!("--start={:.3}", start_position_ms as f64 / 1000.0));
    }
    args
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(player::PlayerState::default())
        .setup(|app| {
            player::create_surface(app)?;
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            browser::open_in_app,
            player::play_native,
            player::layout_native,
            player::stop_native,
            player::native_control,
            player::native_status,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::process::{Command, Stdio};

    #[test]
    fn accepts_http_playback_urls() {
        assert!(is_supported_playback_url("https://cdn.example/movie.mkv"));
        assert!(is_supported_playback_url("http://127.0.0.1:8096/video"));
    }

    #[test]
    fn rejects_non_http_or_injected_urls() {
        assert!(!is_supported_playback_url("file:///tmp/movie.mkv"));
        assert!(!is_supported_playback_url("javascript:alert(1)"));
        assert!(!is_supported_playback_url("https://cdn.example/movie.mkv\n--script=/tmp/x"));
    }

    #[test]
    fn extra_paths_include_host_defaults() {
        let dirs = extra_mpv_dirs();
        assert!(!dirs.is_empty());
        if cfg!(windows) {
            assert!(dirs.iter().any(|dir| dir.ends_with("mpv")));
        } else {
            assert!(dirs.iter().any(|dir| dir == Path::new("/opt/homebrew/bin")));
        }
    }

    #[test]
    fn desktop_window_loads_production_origin() {
        let config: serde_json::Value =
            serde_json::from_str(include_str!("../tauri.conf.json")).expect("tauri.conf.json");
        assert_eq!(config["build"]["devUrl"], "https://media.himym.us.ci");
        assert_eq!(config["build"]["frontendDist"], "https://media.himym.us.ci");
        assert_eq!(
            config["app"]["windows"][0]["url"],
            "https://media.himym.us.ci"
        );
        let capabilities: serde_json::Value =
            serde_json::from_str(include_str!("../capabilities/default.json")).expect("capabilities");
        assert_eq!(capabilities["windows"], serde_json::json!(["main", "browser"]));
    }

    #[test]
    fn resolve_mpv_returns_a_real_file_when_installed() {
        if let Some(found) = resolve_mpv() {
            assert!(found.is_file());
            assert!(found
                .file_name()
                .and_then(|name| name.to_str())
                .is_some_and(|name| name.starts_with("mpv")));
        }
    }

    #[test]
    fn user_agent_rejects_control_characters() {
        assert!(sanitized_user_agent("Mozilla/5.0 MediaHub").is_some());
        assert!(sanitized_user_agent("Mozilla/5.0\n--script=/tmp/x").is_none());
        assert!(sanitized_user_agent("").is_none());
        assert!(sanitized_user_agent("   ").is_none());
    }

    #[test]
    fn mpv_args_keep_comma_user_agent_out_of_header_lists() {
        let agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)";
        let args = mpv_args("范海辛", 0, Some(agent), Some(42), None, None);
        assert!(args.iter().any(|arg| arg == &format!("--user-agent={agent}")));
        assert!(args.iter().any(|arg| arg == "--wid=42"));
        assert!(args.iter().all(|arg| !arg.starts_with("--http-header-fields")));
    }

    #[test]
    fn wait_child_started_rejects_immediate_exit() {
        let mut child = Command::new("false")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .expect("spawn false");
        let result = wait_child_started(&mut child, Duration::from_millis(30));
        assert_eq!(result, Err("mpv 未能打开这路流。".into()));
    }

    #[test]
    fn wait_child_started_accepts_still_running() {
        let mut child = Command::new("sleep")
            .arg("2")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .expect("spawn sleep");
        let result = wait_child_started(&mut child, Duration::from_millis(30));
        let _ = child.kill();
        let _ = child.wait();
        assert_eq!(result, Ok(()));
    }
}
