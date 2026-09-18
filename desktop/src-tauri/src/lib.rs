use std::path::{Path, PathBuf};
use std::process::Child;
use std::time::Duration;

use tauri::webview::{DownloadEvent, WebviewWindowBuilder};
use tauri::{Manager, WebviewUrl};

mod browser;
mod capture;
mod hls;
mod oss_upload;
mod player;
mod updater;

pub(crate) const DESKTOP_ORIGIN: &str = "https://media.himym.us.ci";

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
            dirs.push(PathBuf::from(&program_files).join("mpv"));
            dirs.push(PathBuf::from(&program_files).join("MPV Player"));
        }
        dirs.push(PathBuf::from(r"C:\Program Files\mpv"));
        dirs.push(PathBuf::from(r"C:\Program Files\MPV Player"));
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

pub(crate) fn subtitle_extension(file_name: &str) -> &'static str {
    let lower = file_name.to_ascii_lowercase();
    if lower.ends_with(".ass") || lower.ends_with(".ssa") {
        "ass"
    } else if lower.ends_with(".vtt") {
        "vtt"
    } else {
        "srt"
    }
}

pub(crate) fn subtitle_temp_path(file_name: &str) -> PathBuf {
    std::env::temp_dir().join(format!("media-hub-sub.chi.{}", subtitle_extension(file_name)))
}

pub(crate) fn write_subtitle_temp(bytes: &[u8], file_name: &str) -> Result<PathBuf, String> {
    if bytes.is_empty() {
        return Err("字幕文件为空。".into());
    }
    if bytes.len() > 8 * 1024 * 1024 {
        return Err("字幕文件过大。".into());
    }
    let path = subtitle_temp_path(file_name);
    std::fs::write(&path, bytes).map_err(|_| "无法准备字幕。".to_string())?;
    Ok(path)
}

pub(crate) fn decode_subtitle_base64(value: &str) -> Result<Vec<u8>, String> {
    use base64::Engine;
    base64::engine::general_purpose::STANDARD
        .decode(value.trim())
        .map_err(|_| "字幕内容无效。".to_string())
}

pub(crate) const HUB_OSC_LUA: &str = include_str!("../mpv/hub-osc.lua");

pub(crate) fn mpv_args(
    title: &str,
    start_position_ms: u64,
    user_agent: Option<&str>,
    ipc: Option<&Path>,
    input_conf: Option<&Path>,
    osc_script: Option<&Path>,
    sub_file: Option<&Path>,
) -> Vec<String> {
    // mpv 自己开窗口（有系统边框、自己管全屏）；画面不嵌进网页。
    // 关掉自带灰条 OSC，改走 Hub 电影风脚本。
    let mut args = vec![
        "--force-window=yes".to_string(),
        "--keep-open=no".to_string(),
        "--ytdl=no".to_string(),
        "--osc=no".to_string(),
        "--osd-bar=no".to_string(),
        "--osd-on-seek=no".to_string(),
        "--cache=yes".to_string(),
        "--network-timeout=20".to_string(),
        "--autofit-larger=90%x90%".to_string(),
        "--background=color".to_string(),
        "--background-color=#FF07080A".to_string(),
        "--cursor-autohide=1200".to_string(),
        format!("--title={}", title.replace(['\n', '\r'], " ")),
        "--no-terminal".to_string(),
        "--slang=zh,chi,zh-Hans,zh-CN,zh-TW,zh-HK".to_string(),
    ];
    if cfg!(target_os = "macos") {
        args.push("--macos-title-bar-material=ultraDark".to_string());
        // 不用系统全屏空间，进全屏后铺满整块屏幕（含菜单栏和 Dock）。
        args.push("--native-fs=no".to_string());
    }
    if let Some(agent) = user_agent.and_then(sanitized_user_agent) {
        // Only --user-agent. --http-header-fields is a comma list and would
        // split a normal Mozilla UA into bogus headers, so 115 rejects the URL.
        args.push(format!("--user-agent={agent}"));
    }
    if let Some(ipc) = ipc {
        args.push(format!("--input-ipc-server={}", ipc.display()));
    }
    if let Some(input_conf) = input_conf {
        args.push(format!("--input-conf={}", input_conf.display()));
    }
    if let Some(osc_script) = osc_script {
        args.push(format!("--script={}", osc_script.display()));
    }
    if let Some(sub_file) = sub_file {
        args.push(format!("--sub-file={}", sub_file.display()));
        args.push("--sid=auto".to_string());
        args.push("--sub-visibility=yes".to_string());
        args.push("--subs-with-matching-audio=no".to_string());
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
        .invoke_handler(tauri::generate_handler![
            browser::open_in_app,
            capture::open_page_capture,
            capture::list_page_capture,
            capture::download_page_capture,
            capture::run_page_capture,
            capture::reveal_page_capture,
            capture::upload_page_capture,
            updater::desktop_app_version,
            updater::desktop_app_platform,
            updater::install_desktop_update,
            player::play_native,
            player::attach_native_subtitle,
            player::layout_native,
            player::stop_native,
            player::native_control,
            player::native_status,
            player::toggle_native_window,
            player::set_native_cursor_visible,
            player::open_player_window,
            player::close_player_window,
        ])
        .setup(|app| {
            if app.get_webview_window("main").is_none() {
                let url = tauri::Url::parse(DESKTOP_ORIGIN)?;
                WebviewWindowBuilder::new(app, "main", WebviewUrl::External(url))
                    .title("Media Hub")
                    .inner_size(1280.0, 800.0)
                    .on_download(|_webview, event| {
                        match event {
                            DownloadEvent::Requested { url, destination } => {
                                if let Some(name) = updater::trusted_download_file_name(url.as_str()) {
                                    if let Some(dir) = updater::user_download_dir() {
                                        *destination = dir.join(name);
                                    }
                                }
                            }
                            DownloadEvent::Finished { url, path, success } => {
                                if success {
                                    if let Some(path) = path {
                                        if updater::trusted_download_file_name(url.as_str()).is_some() {
                                            updater::unblock_downloaded_file(&path);
                                        }
                                    }
                                }
                            }
                            _ => {}
                        }
                        true
                    })
                    .build()?;
            }
            Ok(())
        })
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
        assert_eq!(config["build"]["devUrl"], DESKTOP_ORIGIN);
        assert_eq!(config["build"]["frontendDist"], DESKTOP_ORIGIN);
        assert_eq!(config["app"]["windows"], serde_json::json!([]));
        assert_eq!(DESKTOP_ORIGIN, "https://media.himym.us.ci");
        let capabilities: serde_json::Value =
            serde_json::from_str(include_str!("../capabilities/default.json")).expect("capabilities");
        assert_eq!(capabilities["windows"], serde_json::json!(["main", "browser", "player", "capture"]));
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
        let args = mpv_args("范海辛", 0, Some(agent), None, None, None, None);
        assert!(args.iter().any(|arg| arg == "--cache=yes"));
        assert!(args.iter().any(|arg| arg == "--network-timeout=20"));
        assert!(args.iter().any(|arg| arg == "--slang=zh,chi,zh-Hans,zh-CN,zh-TW,zh-HK"));
        assert!(args.iter().any(|arg| arg == &format!("--user-agent={agent}")));
        assert!(args.iter().all(|arg| !arg.starts_with("--http-header-fields")));
    }

    #[test]
    fn mpv_args_attach_external_subtitle() {
        let path = PathBuf::from(r"C:\Temp\media-hub-sub.ass");
        let args = mpv_args("片", 0, None, None, None, None, Some(&path));
        assert!(args.iter().any(|arg| arg == &format!("--sub-file={}", path.display())));
        assert!(args.iter().any(|arg| arg == "--sid=auto"));
        assert!(args.iter().any(|arg| arg == "--sub-visibility=yes"));
        assert!(args.iter().any(|arg| arg == "--subs-with-matching-audio=no"));
    }

    #[test]
    fn mpv_args_open_a_normal_player_window() {
        let args = mpv_args("片", 0, None, None, None, None, None);
        assert!(args.iter().any(|arg| arg == "--osc=no"));
        assert!(args.iter().any(|arg| arg == "--osd-bar=no"));
        assert!(args.iter().any(|arg| arg == "--force-window=yes"));
        assert!(args.iter().any(|arg| arg == "--background=color"));
        #[cfg(target_os = "macos")]
        assert!(args.iter().any(|arg| arg == "--macos-title-bar-material=ultraDark"));
        #[cfg(target_os = "macos")]
        assert!(args.iter().any(|arg| arg == "--native-fs=no"));
        for embedded in ["--wid=", "--geometry=", "--no-border", "--ontop=", "--focus-on="] {
            assert!(args.iter().all(|arg| !arg.starts_with(embedded)), "{embedded} 不该再出现");
        }
    }

    #[test]
    fn mpv_args_load_hub_osc_script() {
        let path = PathBuf::from("/tmp/media-hub-osc.lua");
        let args = mpv_args("片", 0, None, None, None, Some(&path), None);
        assert!(args.iter().any(|arg| arg == &format!("--script={}", path.display())));
        assert!(HUB_OSC_LUA.contains("create_osd_overlay"));
        assert!(HUB_OSC_LUA.contains("&H99D334&"));
        assert!(HUB_OSC_LUA.contains("mbtn_left"));
        assert!(HUB_OSC_LUA.contains("table.concat(lines, \"\\n\")"));
        assert!(HUB_OSC_LUA.contains("cycle\", \"mute"));
        assert!(HUB_OSC_LUA.contains("cycle\", \"audio"));
        assert!(HUB_OSC_LUA.contains("cycle\", \"sub"));
        assert!(HUB_OSC_LUA.contains("media-hub-mpv-skip"));
        assert!(HUB_OSC_LUA.contains("set_property_number"));
        assert!(HUB_OSC_LUA.contains("set_property_bool"));
        assert!(HUB_OSC_LUA.contains("\"border\""));
    }

    #[test]
    fn subtitle_extension_from_name() {
        assert_eq!(subtitle_extension("chi.ass"), "ass");
        assert_eq!(subtitle_extension("CHI.SSA"), "ass");
        assert_eq!(subtitle_extension("a.vtt"), "vtt");
        assert_eq!(subtitle_extension("chi.srt"), "srt");
    }

    #[test]
    fn write_subtitle_temp_round_trips() {
        let path = write_subtitle_temp(b"1\n00:00:01,000 --> 00:00:02,000\nok\n", "chi.srt").expect("write");
        assert!(path.ends_with("media-hub-sub.chi.srt"));
        assert_eq!(std::fs::read(&path).expect("read"), b"1\n00:00:01,000 --> 00:00:02,000\nok\n");
        let _ = std::fs::remove_file(path);
    }

    #[test]
    fn wait_child_started_rejects_immediate_exit() {
        let mut child = if cfg!(windows) {
            Command::new("cmd")
                .args(["/C", "exit", "1"])
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
                .expect("spawn cmd exit")
        } else {
            Command::new("false")
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
                .expect("spawn false")
        };
        let result = wait_child_started(&mut child, Duration::from_millis(30));
        assert_eq!(result, Err("mpv 未能打开这路流。".into()));
    }

    #[test]
    fn wait_child_started_accepts_still_running() {
        let mut child = if cfg!(windows) {
            Command::new("ping")
                .args(["-n", "3", "127.0.0.1"])
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
                .expect("spawn ping")
        } else {
            Command::new("sleep")
                .arg("2")
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
                .expect("spawn sleep")
        };
        let result = wait_child_started(&mut child, Duration::from_millis(30));
        let _ = child.kill();
        let _ = child.wait();
        assert_eq!(result, Ok(()));
    }
}
