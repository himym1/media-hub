#[tauri::command]
fn play_native(url: String, title: String, start_position_ms: u64) -> Result<(), String> {
    if !(url.starts_with("https://") || url.starts_with("http://")) {
        return Err("unsupported playback url".into());
    }
    let mut cmd = std::process::Command::new("mpv");
    cmd.arg("--force-window=yes")
        .arg("--keep-open=no")
        .arg(format!("--title={}", title.replace(['\n', '\r'], " ")))
        .arg("--no-terminal");
    if start_position_ms > 0 {
        cmd.arg(format!("--start={:.3}", start_position_ms as f64 / 1000.0));
    }
    cmd.arg(&url);
    cmd.spawn()
        .map(|_| ())
        .map_err(|_| "未找到 mpv。请先安装 mpv 并确保在 PATH 中。".to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![play_native])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
