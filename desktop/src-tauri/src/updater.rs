use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::time::Duration;

use sha2::{Digest, Sha256};
use tauri::{AppHandle, Manager, Url, WebviewWindow};

const INSTALLER_PREFIX: &str = "/api/v1/client/desktop/releases/";
const SESSION_COOKIE: &str = "media_hub_session";

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum ArtifactKind {
    Windows,
    Darwin,
}

#[tauri::command]
pub fn desktop_app_version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}

#[tauri::command]
pub fn desktop_app_platform() -> String {
    if cfg!(target_os = "macos") {
        "darwin".into()
    } else if cfg!(windows) {
        "windows".into()
    } else {
        "linux".into()
    }
}

#[tauri::command]
pub async fn install_desktop_update(
    app: AppHandle,
    download_path: String,
    sha256: String,
    size_bytes: u64,
) -> Result<(), String> {
    if cfg!(not(any(windows, target_os = "macos"))) {
        return Err("当前系统没有桌面安装包。".into());
    }
    let sha256 = normalize_sha256(&sha256)?;
    if size_bytes == 0 {
        return Err("更新包大小无效。".into());
    }
    let (version_code, kind) = installer_artifact(&download_path)?;
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "找不到应用窗口。".to_string())?;
    let window_url = window.url().map_err(|_| "无法确定更新地址。".to_string())?;
    let download_url = installer_url(&window_url, version_code, kind)?;
    let cookie_window = window.clone();
    let cookie_url = download_url.clone();
    let cookie_header = tauri::async_runtime::spawn_blocking(move || {
        session_cookie_header(&cookie_window, cookie_url)
    })
    .await
    .map_err(|_| "无法读取登录会话。".to_string())??;
    let destination = installer_temp_path(version_code, kind);
    let expected = ExpectedRelease {
        sha256,
        size_bytes,
    };
    tauri::async_runtime::spawn_blocking(move || {
        download_and_verify(&download_url, &cookie_header, &destination, &expected)
    })
    .await
    .map_err(|_| "下载更新失败。".to_string())??;
    start_installer(&installer_temp_path(version_code, kind), kind)?;
    let app_for_exit = app.clone();
    std::thread::spawn(move || {
        std::thread::sleep(Duration::from_millis(500));
        app_for_exit.exit(0);
    });
    Ok(())
}

struct ExpectedRelease {
    sha256: String,
    size_bytes: u64,
}

#[cfg(test)]
pub(crate) fn version_code(version_name: &str) -> Option<u32> {
    let mut parts = version_name.trim().split('.');
    let major = parts.next()?.parse::<u32>().ok()?;
    let minor = parts.next()?.parse::<u32>().ok()?;
    let patch = parts
        .next()?
        .split(|byte: char| !byte.is_ascii_digit())
        .next()?
        .parse::<u32>()
        .ok()?;
    Some(major * 1_000_000 + minor * 1_000 + patch)
}

fn installer_artifact(download_path: &str) -> Result<(u32, ArtifactKind), String> {
    if download_path.contains('\n')
        || download_path.contains('\r')
        || download_path.contains('\0')
        || download_path.contains('?')
        || download_path.contains('#')
        || download_path.contains("..")
    {
        return Err("更新地址无效。".into());
    }
    let rest = download_path
        .strip_prefix(INSTALLER_PREFIX)
        .ok_or_else(|| "更新地址无效。".to_string())?;
    let (code, kind) = if let Some(code) = rest.strip_suffix("/installer") {
        (code, ArtifactKind::Windows)
    } else if let Some(code) = rest.strip_suffix("/dmg") {
        (code, ArtifactKind::Darwin)
    } else {
        return Err("更新地址无效。".into());
    };
    if code.is_empty() || !code.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err("更新地址无效。".into());
    }
    let version_code = code
        .parse::<u32>()
        .ok()
        .filter(|value| *value > 0)
        .ok_or_else(|| "更新地址无效。".to_string())?;
    Ok((version_code, kind))
}

pub(crate) fn trusted_download_file_name(url: &str) -> Option<String> {
    let parsed = Url::parse(url).ok()?;
    if parsed.scheme() != "https" && parsed.scheme() != "http" {
        return None;
    }
    if parsed.query().is_some() || parsed.fragment().is_some() {
        return None;
    }
    let (version_code, kind) = installer_artifact(parsed.path()).ok()?;
    Some(match kind {
        ArtifactKind::Windows => format!("media-hub-{version_code}.exe"),
        ArtifactKind::Darwin => format!("media-hub-{version_code}.dmg"),
    })
}

pub(crate) fn user_download_dir() -> Option<PathBuf> {
    let home = std::env::var_os("USERPROFILE").or_else(|| std::env::var_os("HOME"))?;
    let downloads = PathBuf::from(home).join("Downloads");
    if downloads.is_dir() {
        Some(downloads)
    } else {
        Some(std::env::temp_dir())
    }
}

fn installer_url(window_url: &Url, version_code: u32, kind: ArtifactKind) -> Result<Url, String> {
    if window_url.scheme() != "https" && window_url.scheme() != "http" {
        return Err("更新地址无效。".into());
    }
    let suffix = match kind {
        ArtifactKind::Windows => "installer",
        ArtifactKind::Darwin => "dmg",
    };
    let mut url = window_url.clone();
    url.set_path(&format!("{INSTALLER_PREFIX}{version_code}/{suffix}"));
    url.set_query(None);
    url.set_fragment(None);
    Ok(url)
}

fn normalize_sha256(value: &str) -> Result<String, String> {
    let normalized = value.trim().to_ascii_lowercase();
    if normalized.len() != 64 || !normalized.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("更新包校验值无效。".into());
    }
    Ok(normalized)
}

fn session_cookie_header(window: &WebviewWindow, url: Url) -> Result<String, String> {
    let cookies = window
        .cookies_for_url(url)
        .or_else(|_| window.cookies())
        .map_err(|_| "无法读取登录会话。".to_string())?;
    if !cookies.iter().any(|cookie| cookie.name() == SESSION_COOKIE) {
        return Err("登录会话不可用，请重新登录后再更新。".into());
    }
    Ok(cookies
        .iter()
        .map(|cookie| format!("{}={}", cookie.name(), cookie.value()))
        .collect::<Vec<_>>()
        .join("; "))
}

fn installer_temp_path(version_code: u32, kind: ArtifactKind) -> PathBuf {
    let extension = match kind {
        ArtifactKind::Windows => "exe",
        ArtifactKind::Darwin => "dmg",
    };
    std::env::temp_dir().join(format!("media-hub-{version_code}.{extension}"))
}

fn download_and_verify(
    url: &Url,
    cookie_header: &str,
    destination: &Path,
    expected: &ExpectedRelease,
) -> Result<(), String> {
    let client = reqwest::blocking::Client::builder()
        .timeout(Duration::from_secs(600))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "无法创建下载连接。".to_string())?;
    let mut response = client
        .get(url.as_str())
        .header("Cookie", cookie_header)
        .header("Accept", "application/octet-stream")
        .header(
            "User-Agent",
            format!("Media-Hub-Desktop/{}", env!("CARGO_PKG_VERSION")),
        )
        .send()
        .map_err(|_| "无法下载桌面更新。".to_string())?;
    let status = response.status();
    if status == reqwest::StatusCode::UNAUTHORIZED {
        return Err("登录已过期，请重新登录后再更新。".into());
    }
    if status == reqwest::StatusCode::NOT_FOUND {
        return Err("当前没有可用的桌面更新。".into());
    }
    if !status.is_success() {
        return Err("无法下载桌面更新。".into());
    }
    if let Some(parent) = destination.parent() {
        fs::create_dir_all(parent).map_err(|_| "无法准备更新目录。".to_string())?;
    }
    let mut file = File::create(destination).map_err(|_| "无法保存更新包。".to_string())?;
    let mut hasher = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    let mut total = 0_u64;
    loop {
        let count = response
            .read(&mut buffer)
            .map_err(|_| "下载更新时中断。".to_string())?;
        if count == 0 {
            break;
        }
        file.write_all(&buffer[..count])
            .map_err(|_| "无法写入更新包。".to_string())?;
        hasher.update(&buffer[..count]);
        total += count as u64;
        if total > expected.size_bytes {
            drop(file);
            let _ = fs::remove_file(destination);
            return Err("更新包大小不匹配。".into());
        }
    }
    drop(file);
    let digest = format!("{:x}", hasher.finalize());
    if total != expected.size_bytes || digest != expected.sha256 {
        let _ = fs::remove_file(destination);
        return Err("更新包校验失败。".into());
    }
    unblock_downloaded_file(destination);
    Ok(())
}

pub(crate) fn unblock_downloaded_file(path: &Path) {
    let Some(value) = path.to_str() else {
        return;
    };
    if value.contains('\n') || value.contains('\r') || value.contains('\0') {
        return;
    }
    let _ = fs::remove_file(format!("{value}:Zone.Identifier"));
}

fn start_installer(path: &Path, kind: ArtifactKind) -> Result<(), String> {
    match kind {
        ArtifactKind::Windows => start_windows_installer(path),
        ArtifactKind::Darwin => start_darwin_installer(path),
    }
}

#[cfg_attr(not(windows), allow(dead_code))]
fn windows_installer_path_is_safe(path: &Path) -> bool {
    path.to_str().is_some_and(|value| {
        Path::new(value).is_absolute()
            && !value.chars().any(|byte| matches!(byte, '"' | '\'' | '`' | '&' | '|' | '>' | '<' | '^' | '%'))
    })
}

#[cfg_attr(not(windows), allow(dead_code))]
fn windows_silent_update_command(installer: &str) -> Result<String, String> {
    if !windows_installer_path_is_safe(Path::new(installer)) {
        return Err("更新包路径无效。".into());
    }
    Ok(format!(
        "Start-Sleep -Seconds 5; Start-Process -FilePath '{installer}' -ArgumentList '/S','/UPDATE','/R' -Wait"
    ))
}

fn windows_update_script(installer: &str) -> Result<String, String> {
    if !windows_installer_path_is_safe(Path::new(installer)) {
        return Err("更新包路径无效。".into());
    }
    Ok(format!(
        "@echo off\r\ntimeout /t 5 /nobreak >nul\r\nstart \"\" /wait \"{installer}\" /S /UPDATE /R\r\n"
    ))
}

#[cfg_attr(not(windows), allow(dead_code))]
fn write_windows_update_script(installer: &Path) -> Result<PathBuf, String> {
    let installer_str = installer
        .to_str()
        .ok_or_else(|| "更新包路径无效。".to_string())?;
    let script = windows_update_script(installer_str)?;
    let script_path = installer.with_extension("cmd");
    if !windows_installer_path_is_safe(&script_path) {
        return Err("更新包路径无效。".into());
    }
    fs::write(&script_path, script).map_err(|_| "无法准备安装程序。".to_string())?;
    Ok(script_path)
}

fn start_windows_installer(path: &Path) -> Result<(), String> {
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        use std::process::Command;
        const CREATE_BREAKAWAY_FROM_JOB: u32 = 0x0100_0000;
        const CREATE_NEW_CONSOLE: u32 = 0x0000_0010;
        const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
        const DETACHED_PROCESS: u32 = 0x0000_0008;
        // CREATE_NEW_CONSOLE and DETACHED_PROCESS cannot be combined.
        let breakaway_console = CREATE_BREAKAWAY_FROM_JOB | CREATE_NEW_CONSOLE | CREATE_NEW_PROCESS_GROUP;
        let breakaway_detached = CREATE_BREAKAWAY_FROM_JOB | DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP;
        let script = write_windows_update_script(path)?;
        let script_arg = script
            .to_str()
            .ok_or_else(|| "更新包路径无效。".to_string())?;
        // `start` goes through ShellExecute so the waiter is not a WebView2 job child.
        if Command::new("cmd")
            .args(["/C", "start", "", "/MIN", script_arg])
            .creation_flags(breakaway_console)
            .spawn()
            .is_ok()
        {
            return Ok(());
        }
        let installer = path.to_str().ok_or_else(|| "更新包路径无效。".to_string())?;
        let command = windows_silent_update_command(installer)?;
        Command::new("powershell")
            .args(["-NoProfile", "-WindowStyle", "Hidden", "-Command", &command])
            .creation_flags(breakaway_detached)
            .spawn()
            .map_err(|_| "无法启动安装程序。".to_string())?;
        return Ok(());
    }
    #[cfg(not(windows))]
    {
        let _ = path;
        Err("这个更新包只能在 Windows 上安装。".into())
    }
}

fn start_darwin_installer(path: &Path) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg(path)
            .spawn()
            .map_err(|_| "无法打开安装盘。".to_string())?;
        Ok(())
    }
    #[cfg(not(target_os = "macos"))]
    {
        let _ = path;
        Err("这个更新包只能在 macOS 上安装。".into())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_code_matches_android_formula() {
        assert_eq!(version_code("0.20.38"), Some(20_038));
        assert_eq!(version_code("1.2.3"), Some(1_002_003));
        assert_eq!(version_code("0.9.0-dev"), Some(9_000));
        assert_eq!(version_code("not-a-version"), None);
    }

    #[test]
    fn installer_path_accepts_current_release_only() {
        assert_eq!(
            installer_artifact("/api/v1/client/desktop/releases/20038/installer").unwrap(),
            (20_038, ArtifactKind::Windows)
        );
        assert_eq!(
            installer_artifact("/api/v1/client/desktop/releases/20038/dmg").unwrap(),
            (20_038, ArtifactKind::Darwin)
        );
        assert!(installer_artifact("/api/v1/client/android/releases/20038/apk").is_err());
        assert!(installer_artifact("/api/v1/client/desktop/releases/20038/installer?x=1").is_err());
        assert!(installer_artifact("/api/v1/client/desktop/releases/../20038/installer").is_err());
        assert!(installer_artifact("https://example/installer").is_err());
        assert!(installer_artifact("/api/v1/client/desktop/releases/0/installer").is_err());
    }

    #[test]
    fn installer_url_stays_on_window_origin() {
        let window = Url::parse("https://media.himym.us.ci/library").expect("url");
        let windows = installer_url(&window, 20_038, ArtifactKind::Windows).expect("installer url");
        assert_eq!(
            windows.as_str(),
            "https://media.himym.us.ci/api/v1/client/desktop/releases/20038/installer"
        );
        let darwin = installer_url(&window, 20_038, ArtifactKind::Darwin).expect("dmg url");
        assert_eq!(
            darwin.as_str(),
            "https://media.himym.us.ci/api/v1/client/desktop/releases/20038/dmg"
        );
    }

    #[test]
    fn reports_host_platform() {
        let platform = desktop_app_platform();
        if cfg!(target_os = "macos") {
            assert_eq!(platform, "darwin");
        } else if cfg!(windows) {
            assert_eq!(platform, "windows");
        } else {
            assert_eq!(platform, "linux");
        }
    }

    #[test]
    fn reports_package_version() {
        assert_eq!(desktop_app_version(), env!("CARGO_PKG_VERSION"));
    }

    #[test]
    fn sha256_must_be_64_hex() {
        assert!(normalize_sha256("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef").is_ok());
        assert!(normalize_sha256("short").is_err());
        assert!(normalize_sha256("g".repeat(64).as_str()).is_err());
    }

    #[test]
    fn windows_installer_path_rejects_shell_metacharacters() {
        assert!(!windows_installer_path_is_safe(Path::new("media-hub-20044.exe")));
        if cfg!(windows) {
            assert!(!windows_installer_path_is_safe(Path::new(r"C:\Temp\media-hub-20044.exe&calc")));
            assert!(!windows_installer_path_is_safe(Path::new(r"C:\Temp\media-hub-20044.exe'calc")));
            assert!(windows_installer_path_is_safe(Path::new(r"C:\Temp\media-hub-20044.exe")));
            let command = windows_silent_update_command(r"C:\Temp\media-hub-20044.exe").expect("cmd");
            let script = windows_update_script(r"C:\Temp\media-hub-20044.exe").expect("script");
            assert!(command.contains("/S"));
            assert!(command.contains("/UPDATE"));
            assert!(command.contains("/R"));
            assert!(script.contains("/S"));
            assert!(script.contains("/UPDATE"));
            assert!(script.contains("/R"));
            assert!(windows_silent_update_command(r"C:\Temp\media-hub-20044.exe'calc").is_err());
        } else {
            assert!(!windows_installer_path_is_safe(Path::new("/tmp/media-hub-20044.exe&calc")));
            assert!(windows_installer_path_is_safe(Path::new("/tmp/media-hub-20044.exe")));
            let command = windows_silent_update_command("/tmp/media-hub-20044.exe").expect("cmd");
            let script = windows_update_script("/tmp/media-hub-20044.exe").expect("script");
            assert!(command.contains("/S"));
            assert!(command.contains("/UPDATE"));
            assert!(command.contains("/R"));
            assert!(script.contains("start \"\" /wait \"/tmp/media-hub-20044.exe\" /S /UPDATE /R"));
            assert!(windows_update_script("/tmp/media-hub-20044.exe&calc").is_err());
        }
    }

    #[test]
    fn unblock_ignores_paths_with_control_characters() {
        unblock_downloaded_file(Path::new("media-hub-20048.exe\nZone.Identifier"));
    }

    #[test]
    fn trusted_download_names_stay_on_the_private_installer_path() {
        assert_eq!(
            trusted_download_file_name(
                "https://media.himym.us.ci/api/v1/client/desktop/releases/20044/installer"
            )
            .as_deref(),
            Some("media-hub-20044.exe")
        );
        assert_eq!(
            trusted_download_file_name("https://media.himym.us.ci/api/v1/client/desktop/releases/20044/dmg")
                .as_deref(),
            Some("media-hub-20044.dmg")
        );
        assert!(trusted_download_file_name(
            "https://media.himym.us.ci/api/v1/client/android/releases/20044/apk"
        )
        .is_none());
        assert!(trusted_download_file_name(
            "https://media.himym.us.ci/api/v1/client/desktop/releases/20044/installer?x=1"
        )
        .is_none());
    }
}
