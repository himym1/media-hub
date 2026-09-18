use std::io::{BufRead, BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Mutex};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::webview::WebviewWindowBuilder;
use tauri::{AppHandle, Manager, Url, WebviewUrl, WebviewWindow, WindowEvent};

use crate::{
    command_path_with_extras, decode_subtitle_base64, is_supported_playback_url, mpv_args,
    resolve_mpv, sanitized_user_agent, write_subtitle_temp, HUB_OSC_LUA,
};

pub const PLAYER_LABEL: &str = "player";

#[derive(Default)]
pub struct PlayerState {
    child: Mutex<Option<Child>>,
}

impl PlayerState {
    fn is_running(&self) -> bool {
        let Ok(mut guard) = self.child.lock() else {
            return false;
        };
        let Some(child) = guard.as_mut() else {
            return false;
        };
        match child.try_wait() {
            Ok(Some(_)) => {
                *guard = None;
                false
            }
            Ok(None) => true,
            Err(_) => false,
        }
    }
}

/// 旧版 Web 还会带上内嵌画面的位置；mpv 现在自己开窗，收下即丢。
#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
#[allow(dead_code)]
pub struct EmbedBounds {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct NativeStatus {
    pub paused: bool,
    pub time: f64,
    pub duration: f64,
    pub volume: f64,
    pub speed: f64,
    pub zoom: f64,
    pub cursor_hover: bool,
    pub mouse_x: f64,
    pub mouse_y: f64,
    pub fullscreen: bool,
    pub running: bool,
    pub subtitles: Option<bool>,
    /// OSC 上一集/下一集：lua 写入临时文件，Hub 读走后切队列。
    pub skip: Option<String>,
}

fn host_window(app: &AppHandle) -> Result<WebviewWindow, String> {
    app.get_webview_window(PLAYER_LABEL)
        .or_else(|| app.get_webview_window("main"))
        .ok_or_else(|| "找不到应用窗口。".into())
}

pub(crate) fn sanitized_item_id(value: &str) -> Result<&str, String> {
    let trimmed = value.trim();
    if trimmed.is_empty() || trimmed.len() > 64 {
        return Err("播放条目无效。".into());
    }
    if !trimmed
        .bytes()
        .all(|byte| byte.is_ascii_alphanumeric() || byte == b'_' || byte == b'-')
    {
        return Err("播放条目无效。".into());
    }
    Ok(trimmed)
}

pub(crate) fn player_window_title(title: Option<&str>) -> String {
    let trimmed = title.unwrap_or("").replace(['\n', '\r'], " ");
    let trimmed = trimmed.trim();
    if trimmed.is_empty() || trimmed.len() > 80 {
        "播放".into()
    } else {
        trimmed.to_string()
    }
}

pub(crate) fn player_page_url(play_id: &str, series_id: Option<&str>) -> Result<Url, String> {
    let play = sanitized_item_id(play_id)?;
    let mut href = format!("{}/?view=player&play={play}", crate::DESKTOP_ORIGIN);
    if let Some(series) = series_id.map(str::trim).filter(|value| !value.is_empty()) {
        href.push_str("&series=");
        href.push_str(sanitized_item_id(series)?);
    }
    Url::parse(&href).map_err(|_| "无法打开播放窗口。".to_string())
}

fn restore_host_chrome(app: &AppHandle) {
    if let Ok(host) = host_window(app) {
        let _ = host.set_cursor_visible(true);
    }
}

fn teardown_player_window(app: &AppHandle, state: &PlayerState) {
    stop_child(state);
    restore_host_chrome(app);
}

pub(crate) fn fullscreen_command(fullscreen: Option<bool>) -> Vec<serde_json::Value> {
    match fullscreen {
        Some(value) => vec![
            serde_json::json!("set_property"),
            serde_json::json!("fullscreen"),
            serde_json::json!(value),
        ],
        None => vec![
            serde_json::json!("cycle"),
            serde_json::json!("fullscreen"),
        ],
    }
}

fn ipc_path() -> PathBuf {
    if cfg!(windows) {
        PathBuf::from(r"\\.\pipe\media-hub-mpv")
    } else {
        std::env::temp_dir().join("media-hub-mpv.sock")
    }
}

fn input_conf_path() -> PathBuf {
    std::env::temp_dir().join("media-hub-mpv-input.conf")
}

pub(crate) fn input_conf_contents() -> &'static str {
    // 左键交给 Hub OSC：进度条拖拽、单击暂停、双击全屏。f 全屏。
    "# Hub OSC owns left click.\nMBTN_LEFT ignore\n"
}

fn write_input_conf() -> Result<PathBuf, String> {
    let path = input_conf_path();
    std::fs::write(&path, input_conf_contents()).map_err(|_| "无法准备播放器。".to_string())?;
    Ok(path)
}

fn osc_script_path() -> PathBuf {
    std::env::temp_dir().join("media-hub-osc.lua")
}

fn skip_command_path() -> PathBuf {
    std::env::temp_dir().join("media-hub-mpv-skip")
}

pub(crate) fn take_skip_command() -> Option<String> {
    let path = skip_command_path();
    let raw = std::fs::read_to_string(&path).ok()?;
    let _ = std::fs::remove_file(&path);
    match raw.trim() {
        "next" | "prev" => Some(raw.trim().to_string()),
        _ => None,
    }
}

fn write_osc_script() -> Result<PathBuf, String> {
    let path = osc_script_path();
    std::fs::write(&path, HUB_OSC_LUA).map_err(|_| "无法准备播放器。".to_string())?;
    Ok(path)
}

pub(crate) fn subtitle_visibility_command(value: Option<f64>) -> Vec<serde_json::Value> {
    if let Some(value) = value {
        vec![
            serde_json::json!("set_property"),
            serde_json::json!("sub-visibility"),
            serde_json::json!(value > 0.5),
        ]
    } else {
        vec![
            serde_json::json!("cycle"),
            serde_json::json!("sub-visibility"),
        ]
    }
}

pub(crate) fn native_control_commands(
    action: &str,
    value: Option<f64>,
    mode: Option<&str>,
) -> Result<Vec<Vec<serde_json::Value>>, String> {
    match action {
        "cycle-pause" => Ok(vec![vec![
            serde_json::json!("cycle"),
            serde_json::json!("pause"),
        ]]),
        "seek" => Ok(vec![vec![
            serde_json::json!("seek"),
            serde_json::json!(value.unwrap_or(0.0)),
            serde_json::json!("absolute"),
        ]]),
        "volume" => Ok(vec![vec![
            serde_json::json!("set_property"),
            serde_json::json!("volume"),
            serde_json::json!((value.unwrap_or(1.0) * 100.0).clamp(0.0, 100.0)),
        ]]),
        "speed" => Ok(vec![vec![
            serde_json::json!("set_property"),
            serde_json::json!("speed"),
            serde_json::json!(value.unwrap_or(1.0)),
        ]]),
        "mute" => Ok(vec![vec![
            serde_json::json!("cycle"),
            serde_json::json!("mute"),
        ]]),
        "subtitles" => Ok(vec![subtitle_visibility_command(value)]),
        "cycle-audio" => Ok(vec![vec![
            serde_json::json!("cycle"),
            serde_json::json!("audio"),
        ]]),
        "zoom" => {
            let linear = value.unwrap_or(1.0).clamp(0.5, 3.0);
            Ok(vec![vec![
                serde_json::json!("set_property"),
                serde_json::json!("video-zoom"),
                serde_json::json!(linear.log2()),
            ]])
        }
        "aspect" => Ok(aspect_commands(mode.unwrap_or("fit"))),
        _ => Err("不支持的播放操作。".into()),
    }
}

fn aspect_commands(mode: &str) -> Vec<Vec<serde_json::Value>> {
    let (keepaspect, panscan) = match mode {
        "zoom" | "fixed-height" => (true, 1.0),
        "fill" => (false, 0.0),
        _ => (true, 0.0),
    };
    vec![
        vec![
            serde_json::json!("set_property"),
            serde_json::json!("keepaspect"),
            serde_json::json!(keepaspect),
        ],
        vec![
            serde_json::json!("set_property"),
            serde_json::json!("panscan"),
            serde_json::json!(panscan),
        ],
        vec![
            serde_json::json!("set_property"),
            serde_json::json!("video-unscaled"),
            serde_json::json!("no"),
        ],
    ]
}

fn linear_zoom_from_mpv() -> f64 {
    let log = ipc_number("video-zoom");
    2_f64.powf(log).clamp(0.5, 3.0)
}

fn stop_child(state: &PlayerState) {
    let _ = ipc_command(&[serde_json::json!("quit")]);
    if let Some(mut child) = state.child.lock().ok().and_then(|mut guard| guard.take()) {
        let _ = child.kill();
        wait_child_exit(child, Duration::from_millis(400));
    }
    if !cfg!(windows) {
        let _ = std::fs::remove_file(ipc_path());
    }
    for name in [
        "media-hub-sub.srt",
        "media-hub-sub.ass",
        "media-hub-sub.vtt",
        "media-hub-mpv-skip",
    ] {
        let _ = std::fs::remove_file(std::env::temp_dir().join(name));
    }
}

fn wait_child_exit(mut child: Child, timeout: Duration) {
    let (tx, rx) = mpsc::channel();
    std::thread::spawn(move || {
        let _ = child.wait();
        let _ = tx.send(());
    });
    let _ = rx.recv_timeout(timeout);
}

fn spawn_mpv(
    mpv: &Path,
    url: &str,
    title: &str,
    start_position_ms: u64,
    user_agent: Option<&str>,
    sub_file: Option<&Path>,
    state: &PlayerState,
) -> Result<(), String> {
    stop_child(state);
    if !cfg!(windows) {
        let _ = std::fs::remove_file(ipc_path());
    }
    let ipc = ipc_path();
    let input = write_input_conf()?;
    let osc = write_osc_script()?;
    let mut cmd = Command::new(mpv);
    cmd.args(mpv_args(
        title,
        start_position_ms,
        user_agent,
        Some(&ipc),
        Some(&input),
        Some(&osc),
        sub_file,
    ))
    .arg(url)
    .stdin(Stdio::null())
    .stdout(Stdio::null())
    .stderr(Stdio::null());
    if let Some(path) = command_path_with_extras() {
        cmd.env("PATH", path);
    }
    let child = cmd.spawn().map_err(|_| "无法启动播放器。".to_string())?;
    *state.child.lock().map_err(|_| "无法记录播放器。".to_string())? = Some(child);
    if sub_file.is_some() {
        std::thread::spawn(select_external_subtitle);
    }
    Ok(())
}

fn write_and_read_ipc(mut stream: impl ReadWrite, body: &str) -> Result<serde_json::Value, String> {
    stream
        .write_all(body.as_bytes())
        .map_err(|_| "无法控制播放器。".to_string())?;
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .map_err(|_| "无法控制播放器。".to_string())?;
    parse_ipc_line(&line)
}

trait ReadWrite: Read + Write {}
impl<T: Read + Write> ReadWrite for T {}

fn ipc_busy() -> &'static AtomicBool {
    static BUSY: AtomicBool = AtomicBool::new(false);
    &BUSY
}

fn ipc_command(command: &[serde_json::Value]) -> Result<serde_json::Value, String> {
    if ipc_busy()
        .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
        .is_err()
    {
        return Err("播放器尚未就绪。".to_string());
    }
    let command = command.to_vec();
    let (tx, rx) = mpsc::channel();
    std::thread::spawn(move || {
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| ipc_command_blocking(&command)))
            .unwrap_or_else(|_| Err("无法控制播放器。".to_string()));
        let _ = tx.send(result);
        ipc_busy().store(false, Ordering::SeqCst);
    });
    rx.recv_timeout(Duration::from_millis(400))
        .unwrap_or_else(|_| Err("播放器尚未就绪。".to_string()))
}

fn ipc_command_blocking(command: &[serde_json::Value]) -> Result<serde_json::Value, String> {
    let path = ipc_path();
    let payload = serde_json::json!({ "command": command });
    let body = format!("{payload}\n");
    #[cfg(windows)]
    {
        let stream = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .open(&path)
            .map_err(|_| "播放器尚未就绪。".to_string())?;
        return write_and_read_ipc(stream, &body);
    }
    #[cfg(unix)]
    {
        let stream = std::os::unix::net::UnixStream::connect(&path)
            .map_err(|_| "播放器尚未就绪。".to_string())?;
        let _ = stream.set_read_timeout(Some(Duration::from_millis(250)));
        let _ = stream.set_write_timeout(Some(Duration::from_millis(250)));
        return write_and_read_ipc(stream, &body);
    }
    #[cfg(not(any(windows, unix)))]
    {
        let _ = (path, body);
        Err("此系统暂不支持应用内播放。".into())
    }
}

fn parse_ipc_line(line: &str) -> Result<serde_json::Value, String> {
    let value: serde_json::Value =
        serde_json::from_str(line.trim()).map_err(|_| "无法读取播放状态。".to_string())?;
    if value.get("error").and_then(|error| error.as_str()) == Some("success") {
        Ok(value.get("data").cloned().unwrap_or(serde_json::Value::Null))
    } else {
        Err("无法控制播放器。".into())
    }
}

fn ipc_number(property: &str) -> f64 {
    ipc_command(&[serde_json::json!("get_property"), serde_json::json!(property)])
        .ok()
        .and_then(|value| value.as_f64())
        .unwrap_or(0.0)
}

fn ipc_bool(property: &str) -> bool {
    ipc_command(&[serde_json::json!("get_property"), serde_json::json!(property)])
        .ok()
        .and_then(|value| value.as_bool())
        .unwrap_or(false)
}

#[derive(Clone, Copy, Default)]
pub(crate) struct MousePos {
    pub x: f64,
    pub y: f64,
    pub hover: bool,
}

pub(crate) fn mouse_pos_from_value(value: &serde_json::Value) -> MousePos {
    let number = |key: &str| {
        value
            .get(key)
            .and_then(|item| item.as_f64().or_else(|| item.as_i64().map(|n| n as f64)))
            .unwrap_or(0.0)
    };
    MousePos {
        x: number("x"),
        y: number("y"),
        hover: value.get("hover").and_then(|hover| hover.as_bool()).unwrap_or(false),
    }
}

pub(crate) fn mouse_hover_from_pos(value: &serde_json::Value) -> bool {
    mouse_pos_from_value(value).hover
}

pub(crate) fn sid_from_track_list(value: &serde_json::Value) -> Option<i64> {
    let tracks = value.as_array()?;
    let mut last_external = None;
    let mut last_sub = None;
    for track in tracks {
        if track.get("type").and_then(|item| item.as_str()) != Some("sub") {
            continue;
        }
        let Some(id) = track.get("id").and_then(|item| item.as_i64()) else {
            continue;
        };
        last_sub = Some(id);
        if track.get("external").and_then(|item| item.as_bool()) == Some(true) {
            last_external = Some(id);
        }
    }
    last_external.or(last_sub)
}

fn select_external_subtitle() {
    for _ in 0..25 {
        if let Ok(list) = ipc_command(&[
            serde_json::json!("get_property"),
            serde_json::json!("track-list"),
        ]) {
            if let Some(sid) = sid_from_track_list(&list) {
                let _ = ipc_command(&[
                    serde_json::json!("set_property"),
                    serde_json::json!("sid"),
                    serde_json::json!(sid),
                ]);
                return;
            }
        }
        std::thread::sleep(Duration::from_millis(80));
    }
}

fn ipc_mouse_pos() -> MousePos {
    ipc_command(&[
        serde_json::json!("get_property"),
        serde_json::json!("mouse-pos"),
    ])
    .ok()
    .map(|value| mouse_pos_from_value(&value))
    .unwrap_or_default()
}

#[tauri::command]
pub fn play_native(
    state: tauri::State<PlayerState>,
    url: String,
    title: String,
    start_position_ms: u64,
    user_agent: Option<String>,
    bounds: Option<EmbedBounds>,
    subtitle_base64: Option<String>,
    subtitle_file_name: Option<String>,
) -> Result<(), String> {
    let _ = bounds;
    if !is_supported_playback_url(&url) {
        return Err("unsupported playback url".into());
    }
    let Some(mpv) = resolve_mpv() else {
        return Err("未找到 mpv。请先安装 mpv 并确保在 PATH 中。".into());
    };
    let sub_path = match subtitle_base64.as_deref().filter(|value| !value.trim().is_empty()) {
        Some(encoded) => {
            let bytes = decode_subtitle_base64(encoded)?;
            let name = subtitle_file_name
                .as_deref()
                .filter(|value| !value.trim().is_empty())
                .unwrap_or("chi.srt");
            Some(write_subtitle_temp(&bytes, name)?)
        }
        None => None,
    };
    let agent = user_agent.as_deref().and_then(sanitized_user_agent);
    spawn_mpv(
        &mpv,
        &url,
        &title,
        start_position_ms,
        agent,
        sub_path.as_deref(),
        &state,
    )
}

#[tauri::command]
pub fn attach_native_subtitle(
    subtitle_base64: String,
    subtitle_file_name: Option<String>,
) -> Result<(), String> {
    let encoded = subtitle_base64.trim();
    if encoded.is_empty() {
        return Ok(());
    }
    let bytes = decode_subtitle_base64(encoded)?;
    let name = subtitle_file_name
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or("chi.srt");
    let path = write_subtitle_temp(&bytes, name)?;
    std::thread::spawn(move || {
        let _ = ipc_command(&[
            serde_json::json!("sub-add"),
            serde_json::json!(path.display().to_string()),
        ]);
        select_external_subtitle();
    });
    Ok(())
}

/// 旧版 Web 还会按内嵌位置摆画面；mpv 自己开窗后这里不再动它。
#[tauri::command]
pub fn layout_native(bounds: Option<EmbedBounds>) -> Result<(), String> {
    let _ = bounds;
    Ok(())
}

#[tauri::command]
pub fn stop_native(app: AppHandle, state: tauri::State<PlayerState>) -> Result<(), String> {
    stop_child(&state);
    restore_host_chrome(&app);
    Ok(())
}

#[tauri::command]
pub fn set_native_cursor_visible(app: AppHandle, visible: bool) -> Result<(), String> {
    host_window(&app)?
        .set_cursor_visible(visible)
        .map_err(|_| "无法切换鼠标指针。".to_string())
}

#[tauri::command]
pub async fn native_control(action: String, value: Option<f64>, mode: Option<String>) -> Result<(), String> {
    let commands = native_control_commands(&action, value, mode.as_deref())?;
    tauri::async_runtime::spawn_blocking(move || {
        for command in commands {
            ipc_command(&command)?;
        }
        Ok(())
    })
    .await
    .map_err(|_| "无法控制播放器。".to_string())?
}

/// 全屏归 mpv 自己的窗口管，Hub 窗口不再跟着进出全屏。
#[tauri::command]
pub fn toggle_native_window(fullscreen: Option<bool>) -> Result<(), String> {
    let command = fullscreen_command(fullscreen);
    std::thread::spawn(move || {
        let _ = ipc_command(&command);
    });
    Ok(())
}

#[tauri::command]
pub fn open_player_window(
    app: AppHandle,
    play_id: String,
    title: Option<String>,
    series_id: Option<String>,
) -> Result<(), String> {
    let url = player_page_url(&play_id, series_id.as_deref())?;
    let title = player_window_title(title.as_deref());
    if let Some(existing) = app.get_webview_window(PLAYER_LABEL) {
        existing
            .navigate(url)
            .map_err(|_| "无法打开播放窗口。".to_string())?;
        let _ = existing.set_title(&title);
        let _ = existing.show();
        let _ = existing.set_focus();
        return Ok(());
    }
    let window = WebviewWindowBuilder::new(&app, PLAYER_LABEL, WebviewUrl::External(url))
        .title(title)
        .inner_size(1280.0, 800.0)
        .min_inner_size(720.0, 480.0)
        .decorations(true)
        .skip_taskbar(false)
        .center()
        .build()
        .map_err(|_| "无法打开播放窗口。".to_string())?;
    let handle = app.clone();
    window.on_window_event(move |event| {
        if matches!(
            event,
            WindowEvent::CloseRequested { .. } | WindowEvent::Destroyed
        ) {
            if let Some(state) = handle.try_state::<PlayerState>() {
                teardown_player_window(&handle, &state);
            }
        }
    });
    Ok(())
}

#[tauri::command]
pub fn close_player_window(app: AppHandle, state: tauri::State<PlayerState>) -> Result<(), String> {
    teardown_player_window(&app, &state);
    if let Some(window) = app.get_webview_window(PLAYER_LABEL) {
        let _ = window.close();
    }
    Ok(())
}

#[tauri::command]
pub async fn native_status(state: tauri::State<'_, PlayerState>) -> Result<NativeStatus, String> {
    let running = state.is_running();
    let mut status = tauri::async_runtime::spawn_blocking(read_native_status)
        .await
        .unwrap_or_else(|_| idle_native_status());
    status.running = running;
    Ok(status)
}

fn idle_native_status() -> NativeStatus {
    NativeStatus {
        paused: true,
        time: 0.0,
        duration: 0.0,
        volume: 1.0,
        speed: 1.0,
        zoom: 1.0,
        cursor_hover: false,
        mouse_x: 0.0,
        mouse_y: 0.0,
        fullscreen: false,
        running: false,
        subtitles: None,
        skip: None,
    }
}

fn read_native_status() -> NativeStatus {
    let pos = ipc_mouse_pos();
    NativeStatus {
        paused: ipc_bool("pause"),
        time: ipc_number("time-pos"),
        duration: ipc_number("duration"),
        volume: ipc_number("volume") / 100.0,
        speed: ipc_number("speed").max(0.1),
        zoom: linear_zoom_from_mpv(),
        cursor_hover: pos.hover,
        mouse_x: pos.x,
        mouse_y: pos.y,
        fullscreen: ipc_bool("fullscreen"),
        running: false,
        subtitles: ipc_command(&[
            serde_json::json!("get_property"),
            serde_json::json!("sub-visibility"),
        ])
        .ok()
        .and_then(|value| value.as_bool()),
        skip: take_skip_command(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fullscreen_goes_to_mpv_not_the_hub_window() {
        assert_eq!(
            fullscreen_command(Some(true)),
            vec![
                serde_json::json!("set_property"),
                serde_json::json!("fullscreen"),
                serde_json::json!(true),
            ]
        );
        assert_eq!(
            fullscreen_command(None),
            vec![serde_json::json!("cycle"), serde_json::json!("fullscreen")]
        );
    }

    #[test]
    fn input_conf_leaves_mpv_defaults_alone() {
        let conf = input_conf_contents();
        assert!(conf.contains("MBTN_LEFT ignore"));
        assert!(!conf.contains("cycle pause"));
        assert!(!conf.contains("video-zoom"));
    }

    #[test]
    fn aspect_commands_fill_the_window_for_zoom() {
        let zoom = native_control_commands("aspect", None, Some("zoom")).expect("zoom");
        assert!(zoom.iter().any(|command| {
            command == &vec![
                serde_json::json!("set_property"),
                serde_json::json!("panscan"),
                serde_json::json!(1.0),
            ]
        }));
        let fill = native_control_commands("aspect", None, Some("fill")).expect("fill");
        assert!(fill.iter().any(|command| {
            command == &vec![
                serde_json::json!("set_property"),
                serde_json::json!("keepaspect"),
                serde_json::json!(false),
            ]
        }));
        let zoom_cmd = native_control_commands("zoom", Some(2.0), None).expect("zoom value");
        assert_eq!(
            zoom_cmd,
            vec![vec![
                serde_json::json!("set_property"),
                serde_json::json!("video-zoom"),
                serde_json::json!(1.0),
            ]]
        );
        assert!(native_control_commands("unknown", None, None).is_err());
        assert_eq!(
            native_control_commands("subtitles", Some(0.0), None).expect("hide"),
            vec![vec![
                serde_json::json!("set_property"),
                serde_json::json!("sub-visibility"),
                serde_json::json!(false),
            ]]
        );
        assert_eq!(
            native_control_commands("subtitles", Some(1.0), None).expect("show"),
            vec![vec![
                serde_json::json!("set_property"),
                serde_json::json!("sub-visibility"),
                serde_json::json!(true),
            ]]
        );
    }

    #[test]
    fn mouse_hover_reads_mpv_cursor_state() {
        let pos = mouse_pos_from_value(&serde_json::json!({"x": 12, "y": 8, "hover": true}));
        assert!(pos.hover);
        assert_eq!(pos.x, 12.0);
        assert_eq!(pos.y, 8.0);
        assert!(!mouse_hover_from_pos(&serde_json::json!({"x": 12, "y": 8, "hover": false})));
        assert!(!mouse_hover_from_pos(&serde_json::json!({})));
    }

    #[test]
    fn prefers_the_external_subtitle_track() {
        let tracks = serde_json::json!([
            {"id": 1, "type": "video"},
            {"id": 1, "type": "audio", "lang": "eng"},
            {"id": 1, "type": "sub", "lang": "eng", "external": false},
            {"id": 2, "type": "sub", "external": true, "title": "chi"}
        ]);
        assert_eq!(sid_from_track_list(&tracks), Some(2));
        assert_eq!(sid_from_track_list(&serde_json::json!([{"id": 1, "type": "audio"}])), None);
        assert_eq!(
            sid_from_track_list(&serde_json::json!([{"id": 1, "type": "sub", "external": false}])),
            Some(1)
        );
    }

    #[test]
    fn skip_command_file_is_consumed_once() {
        let path = skip_command_path();
        std::fs::write(&path, "next\n").expect("write skip");
        assert_eq!(take_skip_command().as_deref(), Some("next"));
        assert_eq!(take_skip_command(), None);
        std::fs::write(&path, "prev").expect("write skip");
        assert_eq!(take_skip_command().as_deref(), Some("prev"));
        std::fs::write(&path, "pause").expect("write skip");
        assert_eq!(take_skip_command(), None);
    }

    #[test]
    fn player_ids_stay_short_and_plain() {
        assert_eq!(sanitized_item_id("item-1").expect("id"), "item-1");
        assert_eq!(sanitized_item_id(" r_12 ").expect("shared"), "r_12");
        assert!(sanitized_item_id("").is_err());
        assert!(sanitized_item_id("https://cdn.example/x").is_err());
        assert!(sanitized_item_id("item/1").is_err());
    }

    #[test]
    fn player_page_stays_on_the_desktop_origin() {
        let url = player_page_url("item-1", Some("series-2")).expect("url");
        assert_eq!(
            url.as_str(),
            "https://media.himym.us.ci/?view=player&play=item-1&series=series-2"
        );
        assert!(player_page_url("https://evil.example", None).is_err());
        assert_eq!(player_window_title(Some("  范海辛\n续  ")), "范海辛 续");
        assert_eq!(player_window_title(None), "播放");
    }
}
