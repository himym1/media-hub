use std::io::{BufRead, BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, PhysicalPosition, PhysicalSize, WebviewWindow, Window};

use crate::{
    command_path_with_extras, decode_subtitle_base64, is_supported_playback_url, mpv_args,
    resolve_mpv, sanitized_user_agent, wait_child_started, write_subtitle_temp,
};

pub const SURFACE_LABEL: &str = "mpv-surface";

#[derive(Default)]
pub struct PlayerState {
    child: Mutex<Option<Child>>,
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

fn ensure_surface(app: &AppHandle) -> Result<Window, String> {
    if let Some(existing) = app.get_window(SURFACE_LABEL) {
        return Ok(existing);
    }
    let Some(main) = app.get_window("main") else {
        return Err("找不到应用窗口。".into());
    };
    tauri::window::WindowBuilder::new(app, SURFACE_LABEL)
        .title(" ")
        .decorations(false)
        .resizable(false)
        .skip_taskbar(true)
        .visible(false)
        .inner_size(8.0, 8.0)
        .position(-20_000.0, -20_000.0)
        .parent(&main)
        .map_err(|_| "无法准备播放画面。".to_string())?
        .build()
        .map_err(|_| "无法准备播放画面。".to_string())?;
    let surface = app
        .get_window(SURFACE_LABEL)
        .ok_or_else(|| "找不到内嵌播放窗口。".to_string())?;
    park_surface(&surface)?;
    Ok(surface)
}

fn main_window(app: &AppHandle) -> Result<WebviewWindow, String> {
    app.get_webview_window("main")
        .ok_or_else(|| "找不到应用窗口。".into())
}

fn surface_window(app: &AppHandle) -> Result<Window, String> {
    app.get_window(SURFACE_LABEL)
        .ok_or_else(|| "找不到内嵌播放窗口。".into())
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
        let _ = child.wait();
    }
    if !cfg!(windows) {
        let _ = std::fs::remove_file(ipc_path());
    }
    for name in ["media-hub-sub.srt", "media-hub-sub.ass", "media-hub-sub.vtt"] {
        let _ = std::fs::remove_file(std::env::temp_dir().join(name));
    }
}

fn spawn_embedded(
    mpv: &Path,
    url: &str,
    title: &str,
    start_position_ms: u64,
    user_agent: Option<&str>,
    wid: i64,
    sub_file: Option<&Path>,
    state: &PlayerState,
) -> Result<(), String> {
    stop_child(state);
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
        Some(wid),
        Some(&ipc),
        Some(&input),
        sub_file,
    ))
    .arg(url)
    .stdin(Stdio::null())
    .stdout(Stdio::null())
    .stderr(Stdio::null());
    if let Some(path) = command_path_with_extras() {
        cmd.env("PATH", path);
    }
    let mut child = cmd.spawn().map_err(|_| "无法启动播放器。".to_string())?;
    wait_child_started(&mut child, Duration::from_millis(1200))?;
    *state.child.lock().map_err(|_| "无法记录播放器。".to_string())? = Some(child);
    if sub_file.is_some() {
        select_external_subtitle();
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

fn ipc_command(command: &[serde_json::Value]) -> Result<serde_json::Value, String> {
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

pub(crate) fn mouse_hover_from_pos(value: &serde_json::Value) -> bool {
    value.get("hover").and_then(|hover| hover.as_bool()).unwrap_or(false)
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

fn ipc_mouse_hover() -> bool {
    ipc_command(&[
        serde_json::json!("get_property"),
        serde_json::json!("mouse-pos"),
    ])
    .ok()
    .is_some_and(|value| mouse_hover_from_pos(&value))
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
    let main = main_window(&app)?;
    let surface = ensure_surface(&app)?;
    apply_bounds(&main, &surface, &bounds)?;
    surface.show().map_err(|_| "无法打开播放画面。".to_string())?;
    // Keep the webview on top for mouse-move / double-click; mpv only paints.
    let _ = surface.set_ignore_cursor_events(true);
    let wid = surface_wid(&surface)?;
    spawn_embedded(
        &mpv,
        &url,
        &title,
        start_position_ms,
        user_agent.as_deref().and_then(sanitized_user_agent),
        wid,
        sub_path.as_deref(),
        &state,
    )?;
    let _ = main.set_focus();
    Ok(())
}

#[tauri::command]
pub fn layout_native(app: AppHandle, bounds: EmbedBounds) -> Result<(), String> {
    let Ok(surface) = surface_window(&app) else {
        return Ok(());
    };
    apply_bounds(&main_window(&app)?, &surface, &bounds)?;
    let _ = surface.set_ignore_cursor_events(true);
    Ok(())
}

#[tauri::command]
pub fn stop_native(app: AppHandle, state: tauri::State<PlayerState>) -> Result<(), String> {
    stop_child(&state);
    if let Ok(surface) = surface_window(&app) {
        let _ = park_surface(&surface);
    }
    Ok(())
}

#[tauri::command]
pub fn native_control(action: String, value: Option<f64>, mode: Option<String>) -> Result<(), String> {
    for command in native_control_commands(&action, value, mode.as_deref())? {
        ipc_command(&command)?;
    }
    Ok(())
}

#[tauri::command]
pub fn toggle_native_window(app: AppHandle) -> Result<(), String> {
    let window = main_window(&app)?;
    let maximized = window.is_maximized().unwrap_or(false);
    if maximized {
        window.unmaximize()
    } else {
        window.maximize()
    }
    .map_err(|_| "无法缩放窗口。".to_string())
}

#[tauri::command]
pub fn native_status() -> Result<NativeStatus, String> {
    Ok(NativeStatus {
        paused: ipc_bool("pause"),
        time: ipc_number("time-pos"),
        duration: ipc_number("duration"),
        volume: ipc_number("volume") / 100.0,
        speed: ipc_number("speed").max(0.1),
        zoom: linear_zoom_from_mpv(),
        cursor_hover: ipc_mouse_hover(),
    })
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
        assert!(mouse_hover_from_pos(&serde_json::json!({"x": 12, "y": 8, "hover": true})));
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
}
