use std::path::{Path, PathBuf};
use std::process::Command;

fn is_supported_playback_url(url: &str) -> bool {
    if url.contains('\n') || url.contains('\r') || url.contains('\0') {
        return false;
    }
    url.starts_with("https://") || url.starts_with("http://")
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

fn resolve_mpv() -> Option<PathBuf> {
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

fn spawn_mpv(mpv: &Path, url: &str, title: &str, start_position_ms: u64) -> Result<(), String> {
    let mut cmd = Command::new(mpv);
    cmd.arg("--force-window=yes")
        .arg("--keep-open=no")
        .arg(format!("--title={}", title.replace(['\n', '\r'], " ")))
        .arg("--no-terminal");
    if start_position_ms > 0 {
        cmd.arg(format!("--start={:.3}", start_position_ms as f64 / 1000.0));
    }
    cmd.arg(url);
    cmd.spawn()
        .map(|_| ())
        .map_err(|_| "无法启动 mpv。".to_string())
}

#[tauri::command]
fn play_native(url: String, title: String, start_position_ms: u64) -> Result<(), String> {
    if !is_supported_playback_url(&url) {
        return Err("unsupported playback url".into());
    }
    let Some(mpv) = resolve_mpv() else {
        return Err("未找到 mpv。请先安装 mpv 并确保在 PATH 中。".into());
    };
    spawn_mpv(&mpv, &url, &title, start_position_ms)
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![play_native])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;

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
}
