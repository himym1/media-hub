use std::io::{BufRead, BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Mutex};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::webview::WebviewWindowBuilder;
use tauri::{
    AppHandle, Manager, PhysicalPosition, PhysicalSize, Url, WebviewUrl, WebviewWindow, Window,
    WindowEvent,
};

use crate::{
    command_path_with_extras, decode_subtitle_base64, is_supported_playback_url, mpv_args,
    resolve_mpv, sanitized_user_agent, write_subtitle_temp,
};

pub const SURFACE_LABEL: &str = "mpv-surface";
pub const PLAYER_LABEL: &str = "player";

#[derive(Default)]
pub struct PlayerState {
    child: Mutex<Option<Child>>,
    windowed_fullscreen: AtomicBool,
    last_geometry: Mutex<Option<String>>,
}

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
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
}

fn parked_origin() -> PhysicalPosition<i32> {
    PhysicalPosition::new(-20_000, -20_000)
}

fn park_surface(surface: &Window) -> Result<(), String> {
    let _ = surface.hide();
    surface
        .set_position(parked_origin())
        .map_err(|_| "无法收起播放画面。".to_string())?;
    surface
        .set_size(PhysicalSize::new(8, 8))
        .map_err(|_| "无法收起播放画面。".to_string())?;
    Ok(())
}

fn close_surface(app: &AppHandle) {
    if let Some(surface) = app.get_window(SURFACE_LABEL) {
        let _ = surface.close();
    }
}

fn ensure_surface(app: &AppHandle) -> Result<Window, String> {
    if let Some(existing) = app.get_window(SURFACE_LABEL) {
        return Ok(existing);
    }
    let host = host_native_window(app)?;
    tauri::window::WindowBuilder::new(app, SURFACE_LABEL)
        .title(" ")
        .decorations(false)
        .resizable(false)
        .skip_taskbar(true)
        .visible(false)
        .inner_size(8.0, 8.0)
        .position(-20_000.0, -20_000.0)
        .parent(&host)
        .map_err(|_| "无法准备播放画面。".to_string())?
        .build()
        .map_err(|_| "无法准备播放画面。".to_string())?;
    let surface = app
        .get_window(SURFACE_LABEL)
        .ok_or_else(|| "找不到内嵌播放窗口。".to_string())?;
    park_surface(&surface)?;
    Ok(surface)
}

fn host_window(app: &AppHandle) -> Result<WebviewWindow, String> {
    app.get_webview_window(PLAYER_LABEL)
        .or_else(|| app.get_webview_window("main"))
        .ok_or_else(|| "找不到应用窗口。".into())
}

fn host_native_window(app: &AppHandle) -> Result<Window, String> {
    app.get_window(PLAYER_LABEL)
        .or_else(|| app.get_window("main"))
        .ok_or_else(|| "找不到应用窗口。".into())
}

fn main_window(app: &AppHandle) -> Result<WebviewWindow, String> {
    app.get_webview_window("main")
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
    if let Some(player) = app.get_webview_window(PLAYER_LABEL) {
        let _ = player.set_cursor_visible(true);
        let _ = player.set_fullscreen(false);
        return;
    }
    if let Ok(window) = main_window(app) {
        let _ = window.set_cursor_visible(true);
        let _ = window.set_fullscreen(false);
    }
}

fn teardown_player_window(app: &AppHandle, state: &PlayerState) {
    stop_child(state);
    close_surface(app);
    restore_host_chrome(app);
}

fn surface_window(app: &AppHandle) -> Result<Window, String> {
    app.get_window(SURFACE_LABEL)
        .ok_or_else(|| "找不到内嵌播放窗口。".into())
}

fn host_embed_supported() -> bool {
    cfg!(windows)
}

pub(crate) fn should_apply_windowed_layout(fullscreen: bool) -> bool {
    !fullscreen
}

pub(crate) fn windowed_fullscreen_command(enable: bool) -> Vec<serde_json::Value> {
    vec![
        serde_json::json!("set_property"),
        serde_json::json!("fullscreen"),
        serde_json::json!(enable),
    ]
}

pub(crate) fn window_geometry(scale: f64, origin_x: i32, origin_y: i32, bounds: &EmbedBounds) -> String {
    let scale = if scale > 0.0 { scale } else { 1.0 };
    let x = (origin_x as f64) / scale + bounds.x;
    let y = (origin_y as f64) / scale + bounds.y;
    format!(
        "{}x{}{:+}{:+}",
        bounds.width.max(8.0).round() as u32,
        bounds.height.max(8.0).round() as u32,
        x.round() as i32,
        y.round() as i32,
    )
}

fn windowed_geometry(host: &WebviewWindow, bounds: &EmbedBounds) -> Result<String, String> {
    if bounds.width < 8.0 || bounds.height < 8.0 {
        return Err("播放区域太小。".into());
    }
    let scale = host.scale_factor().unwrap_or(1.0);
    let origin = host
        .inner_position()
        .map_err(|_| "无法定位播放窗口。".to_string())?;
    Ok(window_geometry(scale, origin.x, origin.y, bounds))
}

fn apply_bounds(main: &WebviewWindow, surface: &Window, bounds: &EmbedBounds) -> Result<(), String> {
    if bounds.width < 8.0 || bounds.height < 8.0 {
        return Err("播放区域太小。".into());
    }
    let scale = main.scale_factor().map_err(|_| "无法读取窗口。".to_string())?;
    let inner = main
        .inner_position()
        .map_err(|_| "无法读取窗口。".to_string())?;
    let x = inner.x as f64 + bounds.x * scale;
    let y = inner.y as f64 + bounds.y * scale;
    let width = (bounds.width * scale).max(8.0);
    let height = (bounds.height * scale).max(8.0);
    surface
        .set_position(PhysicalPosition::new(x.round() as i32, y.round() as i32))
        .map_err(|_| "无法放置播放画面。".to_string())?;
    surface
        .set_size(PhysicalSize::new(width.round() as u32, height.round() as u32))
        .map_err(|_| "无法放置播放画面。".to_string())?;
    Ok(())
}

fn surface_wid(surface: &Window) -> Result<i64, String> {
    #[cfg(target_os = "macos")]
    {
        Ok(surface.ns_view().map_err(|_| "无法嵌入播放器。".to_string())? as i64)
    }
    #[cfg(windows)]
    {
        Ok(surface.hwnd().map_err(|_| "无法嵌入播放器。".to_string())?.0 as i64)
    }
    #[cfg(not(any(target_os = "macos", windows)))]
    {
        let _ = surface;
        Err("此系统暂不支持应用内播放。".into())
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
    concat!(
        "MBTN_LEFT cycle pause\n",
        "WHEEL_UP add video-zoom 0.1\n",
        "WHEEL_DOWN add video-zoom -0.1\n",
        "WHEEL_LEFT seek -10\n",
        "WHEEL_RIGHT seek 10\n",
    )
}

fn write_input_conf() -> Result<PathBuf, String> {
    let path = input_conf_path();
    std::fs::write(&path, input_conf_contents()).map_err(|_| "无法准备播放器。".to_string())?;
    Ok(path)
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
        "subtitles" => Ok(vec![vec![
            serde_json::json!("cycle"),
            serde_json::json!("sub-visibility"),
        ]]),
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
    for name in ["media-hub-sub.srt", "media-hub-sub.ass", "media-hub-sub.vtt"] {
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
    wid: Option<i64>,
    geometry: Option<&str>,
    sub_file: Option<&Path>,
    state: &PlayerState,
) -> Result<(), String> {
    stop_child(state);
    state.windowed_fullscreen.store(false, Ordering::SeqCst);
    if !cfg!(windows) {
        let _ = std::fs::remove_file(ipc_path());
    }
    let ipc = ipc_path();
    let input = write_input_conf()?;
    let mut cmd = Command::new(mpv);
    cmd.args(mpv_args(
        title,
        start_position_ms,
        user_agent,
        wid,
        Some(&ipc),
        Some(&input),
        sub_file,
        geometry,
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
                let _ = ipc_command(&[
                    serde_json::json!("set_property"),
                    serde_json::json!("sub-visibility"),
                    serde_json::json!(true),
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
    app: AppHandle,
    state: tauri::State<PlayerState>,
    url: String,
    title: String,
    start_position_ms: u64,
    user_agent: Option<String>,
    bounds: EmbedBounds,
    subtitle_base64: Option<String>,
    subtitle_file_name: Option<String>,
) -> Result<(), String> {
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
    let host = host_window(&app)?;
    let agent = user_agent.as_deref().and_then(sanitized_user_agent);
    if host_embed_supported() {
        let surface = ensure_surface(&app)?;
        apply_bounds(&host, &surface, &bounds)?;
        surface.show().map_err(|_| "无法打开播放画面。".to_string())?;
        let _ = surface.set_ignore_cursor_events(true);
        let wid = surface_wid(&surface)?;
        spawn_mpv(
            &mpv,
            &url,
            &title,
            start_position_ms,
            agent,
            Some(wid),
            None,
            sub_path.as_deref(),
            &state,
        )?;
        if app.get_webview_window(PLAYER_LABEL).is_none() {
            let _ = host.set_fullscreen(true);
        }
    } else {
        let geometry = windowed_geometry(&host, &bounds)?;
        spawn_mpv(
            &mpv,
            &url,
            &title,
            start_position_ms,
            agent,
            None,
            Some(&geometry),
            sub_path.as_deref(),
            &state,
        )?;
    }
    let _ = host.set_focus();
    Ok(())
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

#[tauri::command]
pub fn layout_native(
    app: AppHandle,
    state: tauri::State<PlayerState>,
    bounds: EmbedBounds,
) -> Result<(), String> {
    if host_embed_supported() {
        let Ok(surface) = surface_window(&app) else {
            return Ok(());
        };
        apply_bounds(&host_window(&app)?, &surface, &bounds)?;
        let _ = surface.set_ignore_cursor_events(true);
        return Ok(());
    }
    if !should_apply_windowed_layout(state.windowed_fullscreen.load(Ordering::SeqCst)) {
        return Ok(());
    }
    let host = host_window(&app)?;
    let geometry = windowed_geometry(&host, &bounds)?;
    if let Ok(mut last) = state.last_geometry.lock() {
        *last = Some(geometry.clone());
    }
    std::thread::spawn(move || {
        let _ = ipc_command(&[
            serde_json::json!("set_property"),
            serde_json::json!("geometry"),
            serde_json::json!(geometry),
        ]);
    });
    Ok(())
}

#[tauri::command]
pub fn stop_native(app: AppHandle, state: tauri::State<PlayerState>) -> Result<(), String> {
    state.windowed_fullscreen.store(false, Ordering::SeqCst);
    stop_child(&state);
    if let Ok(surface) = surface_window(&app) {
        let _ = park_surface(&surface);
    }
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

#[tauri::command]
pub fn toggle_native_window(app: AppHandle, state: tauri::State<PlayerState>) -> Result<(), String> {
    if host_embed_supported() {
        let window = host_window(&app)?;
        let fullscreen = window.is_fullscreen().unwrap_or(false);
        window
            .set_fullscreen(!fullscreen)
            .map_err(|_| "无法切换全屏。".to_string())?;
        return Ok(());
    }
    let next = !state.windowed_fullscreen.load(Ordering::SeqCst);
    state.windowed_fullscreen.store(next, Ordering::SeqCst);
    let geometry = state
        .last_geometry
        .lock()
        .ok()
        .and_then(|guard| guard.clone());
    std::thread::spawn(move || {
        let _ = ipc_command(&windowed_fullscreen_command(next));
        if !next {
            if let Some(geometry) = geometry {
                let _ = ipc_command(&[
                    serde_json::json!("set_property"),
                    serde_json::json!("geometry"),
                    serde_json::json!(geometry),
                ]);
            }
        }
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
pub async fn native_status() -> Result<NativeStatus, String> {
    Ok(tauri::async_runtime::spawn_blocking(read_native_status)
        .await
        .unwrap_or_else(|_| idle_native_status()))
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
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parked_surface_stays_offscreen() {
        let origin = parked_origin();
        assert!(origin.x <= -10_000);
        assert!(origin.y <= -10_000);
    }

    #[test]
    fn window_geometry_uses_logical_points() {
        let bounds = EmbedBounds {
            x: 10.0,
            y: 20.0,
            width: 800.0,
            height: 450.0,
        };
        assert_eq!(window_geometry(2.0, 200, 80, &bounds), "800x450+110+60");
        assert_eq!(window_geometry(1.0, 24, 48, &bounds), "800x450+34+68");
    }

    #[test]
    fn windowed_fullscreen_skips_layout_and_only_tells_mpv() {
        assert!(should_apply_windowed_layout(false));
        assert!(!should_apply_windowed_layout(true));
        assert_eq!(
            windowed_fullscreen_command(true),
            vec![
                serde_json::json!("set_property"),
                serde_json::json!("fullscreen"),
                serde_json::json!(true),
            ]
        );
    }

    #[test]
    fn input_conf_zooms_with_the_mouse_wheel() {
        let conf = input_conf_contents();
        assert!(conf.contains("MBTN_LEFT cycle pause"));
        assert!(conf.contains("WHEEL_UP add video-zoom 0.1"));
        assert!(conf.contains("WHEEL_DOWN add video-zoom -0.1"));
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
