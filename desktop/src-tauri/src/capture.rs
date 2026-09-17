use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::sync::mpsc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use reqwest::blocking::Client;
use reqwest::header::{HeaderMap, HeaderValue, COOKIE, ORIGIN, REFERER, USER_AGENT};
use serde::{Deserialize, Serialize};
use tauri::webview::{NewWindowResponse, PageLoadEvent};
use tauri::{AppHandle, Manager, Url, WebviewUrl, WebviewWindow, WebviewWindowBuilder};

use crate::hls::{
    best_variant, classify_media_url, looks_like_m3u8, output_extension, parse_m3u8, HlsPlan,
    MediaKind, MAX_FILE_BYTES, MAX_PLAYLIST_BYTES,
};
use crate::is_supported_playback_url;

pub const CAPTURE_LABEL: &str = "capture";
const CAPTURE_HOOK: &str = include_str!("capture.js");
const CAPTURE_UA: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36";
const WAIT_MEDIA_FOR: Duration = Duration::from_secs(45);
const WAIT_QUIET_FOR: Duration = Duration::from_millis(2500);
const WAIT_POLL_EVERY: Duration = Duration::from_millis(400);
const LIST_JS: &str = r#"
(function () {
  try {
    if (typeof window.__mhCollectCapture === 'function') {
      return window.__mhCollectCapture();
    }
    return { page: location.href, items: [] };
  } catch (e) {
    return { page: '', items: [], error: String(e && e.message ? e.message : e) };
  }
})()
"#;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CaptureItem {
    pub url: String,
    pub kind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CaptureSnapshot {
    pub page: String,
    #[serde(default)]
    pub items: Vec<CaptureItem>,
}

#[derive(Debug, Clone, Serialize)]
pub struct CaptureDownload {
    pub kind: String,
    pub path: String,
    pub share_import_url: Option<String>,
}

fn parse_http_url(url: &str) -> Result<Url, String> {
    if !is_supported_playback_url(url) {
        return Err("只接受 http(s) 网页。".into());
    }
    Url::parse(url).map_err(|_| "网页地址无效。".to_string())
}

fn capture_profile_dir(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app
        .path()
        .app_data_dir()
        .map_err(|_| "找不到抓取配置目录。".to_string())?
        .join("capture-profile");
    fs::create_dir_all(&dir).map_err(|_| "无法创建抓取配置目录。".to_string())?;
    Ok(dir)
}

fn capture_output_dir(app: &AppHandle) -> Result<PathBuf, String> {
    let base = app
        .path()
        .download_dir()
        .or_else(|_| app.path().app_data_dir())
        .map_err(|_| "找不到下载目录。".to_string())?;
    let dir = base.join("MediaHubCapture");
    fs::create_dir_all(&dir).map_err(|_| "无法创建下载目录。".to_string())?;
    Ok(dir)
}

fn inject_hook(window: &WebviewWindow) {
    let _ = window.eval(CAPTURE_HOOK);
}

fn hide_capture(window: &WebviewWindow) {
    let _ = window.set_skip_taskbar(true);
    let _ = window.hide();
}

fn close_capture(app: &AppHandle) {
    if let Some(window) = app.get_webview_window(CAPTURE_LABEL) {
        let _ = window.close();
    }
}

fn open_capture(app: &AppHandle, url: &str) -> Result<(), String> {
    let parsed = parse_http_url(url)?;
    if let Some(existing) = app.get_webview_window(CAPTURE_LABEL) {
        hide_capture(&existing);
        existing
            .navigate(parsed)
            .map_err(|_| "无法打开抓取页。".to_string())?;
        inject_hook(&existing);
        return Ok(());
    }

    let profile = capture_profile_dir(app)?;
    let handle = app.clone();
    let builder = WebviewWindowBuilder::new(app, CAPTURE_LABEL, WebviewUrl::External(parsed))
        .title("抓取视频")
        .inner_size(1100.0, 800.0)
        .visible(false)
        .focused(false)
        .skip_taskbar(true)
        .user_agent(CAPTURE_UA)
        .data_directory(profile)
        .initialization_script_for_all_frames(CAPTURE_HOOK)
        .on_page_load(|window, payload| {
            hide_capture(&window);
            if payload.event() == PageLoadEvent::Finished {
                inject_hook(&window);
            }
        })
        .on_new_window(move |next, _features| {
            if is_supported_playback_url(next.as_str()) {
                if let Some(existing) = handle.get_webview_window(CAPTURE_LABEL) {
                    let _ = existing.navigate(next.clone());
                }
            }
            NewWindowResponse::Deny
        });
    builder
        .build()
        .map_err(|_| "无法打开抓取页。".to_string())?;
    Ok(())
}

fn pick_auto_item(items: &[CaptureItem]) -> Option<&CaptureItem> {
    items
        .iter()
        .rev()
        .find(|item| item.kind == "hls")
        .or_else(|| items.iter().rev().find(|item| item.kind == "file"))
}

fn parse_snapshot(raw: &str) -> Result<CaptureSnapshot, String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() || trimmed == "null" {
        return Ok(CaptureSnapshot {
            page: String::new(),
            items: Vec::new(),
        });
    }
    let snapshot = serde_json::from_str::<CaptureSnapshot>(trimmed)
        .or_else(|_| {
            serde_json::from_str::<String>(trimmed)
                .and_then(|inner| serde_json::from_str::<CaptureSnapshot>(&inner))
        })
        .map_err(|_| "抓取结果无法读取。".to_string())?;
    Ok(sanitize_snapshot(snapshot))
}

fn sanitize_snapshot(mut snapshot: CaptureSnapshot) -> CaptureSnapshot {
    snapshot.items.retain(|item| classify_media_url(&item.url).is_some());
    for item in &mut snapshot.items {
        if let Some(kind) = classify_media_url(&item.url) {
            item.kind = match kind {
                MediaKind::Hls => "hls".into(),
                MediaKind::File => "file".into(),
            };
        }
    }
    snapshot.items.sort_by_key(|item| match item.kind.as_str() {
        "file" => 0,
        "hls" => 1,
        _ => 2,
    });
    snapshot.items.truncate(80);
    snapshot
}

fn cookie_header(window: &WebviewWindow, url: &str) -> String {
    let Ok(parsed) = Url::parse(url) else {
        return String::new();
    };
    let Ok(cookies) = window.cookies_for_url(parsed) else {
        return String::new();
    };
    cookies
        .iter()
        .map(|cookie| format!("{}={}", cookie.name(), cookie.value()))
        .collect::<Vec<_>>()
        .join("; ")
}

pub(crate) fn http_client() -> Result<Client, String> {
    Client::builder()
        .timeout(Duration::from_secs(60))
        .connect_timeout(Duration::from_secs(15))
        .redirect(reqwest::redirect::Policy::limited(8))
        .build()
        .map_err(|_| "无法开始下载。".to_string())
}

fn request_headers(referer: &str, cookie: &str) -> HeaderMap {
    let mut headers = HeaderMap::new();
    if let Ok(value) = HeaderValue::from_str(CAPTURE_UA) {
        headers.insert(USER_AGENT, value);
    }
    if !referer.is_empty() {
        if let Ok(value) = HeaderValue::from_str(referer) {
            headers.insert(REFERER, value);
        }
        if let Ok(url) = Url::parse(referer) {
            if let Some(host) = url.host_str() {
                let origin = format!("{}://{}", url.scheme(), host);
                if let Ok(value) = HeaderValue::from_str(&origin) {
                    headers.insert(ORIGIN, value);
                }
            }
        }
    }
    if !cookie.is_empty() {
        if let Ok(value) = HeaderValue::from_str(cookie) {
            headers.insert(COOKIE, value);
        }
    }
    headers
}

fn fetch_bytes(client: &Client, url: &str, referer: &str, cookie: &str, limit: usize) -> Result<Vec<u8>, String> {
    let response = client
        .get(url)
        .headers(request_headers(referer, cookie))
        .send()
        .map_err(|_| "下载失败。".to_string())?;
    if !response.status().is_success() {
        return Err(status_error(response.status().as_u16()));
    }
    let mut body = Vec::new();
    response
        .take(limit as u64 + 1)
        .read_to_end(&mut body)
        .map_err(|_| "下载失败。".to_string())?;
    if body.len() > limit {
        return Err("文件过大。".into());
    }
    Ok(body)
}

fn append_url_to_file(
    client: &Client,
    url: &str,
    referer: &str,
    cookie: &str,
    file: &mut File,
    written: &mut u64,
) -> Result<(), String> {
    let response = client
        .get(url)
        .headers(request_headers(referer, cookie))
        .send()
        .map_err(|_| "下载失败。".to_string())?;
    if !response.status().is_success() {
        return Err(status_error(response.status().as_u16()));
    }
    if let Some(len) = response.content_length() {
        if *written + len > MAX_FILE_BYTES {
            return Err("文件过大。".into());
        }
    }
    let mut reader = response;
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let n = reader.read(&mut buffer).map_err(|_| "下载失败。".to_string())?;
        if n == 0 {
            break;
        }
        *written += n as u64;
        if *written > MAX_FILE_BYTES {
            return Err("文件过大。".into());
        }
        file.write_all(&buffer[..n]).map_err(|_| "无法写入下载文件。".to_string())?;
    }
    Ok(())
}

fn status_error(status: u16) -> String {
    if status == 403 || status == 401 {
        "抓到了地址但下载被拒绝，页面 Cookie 或 Referer 可能不够。".into()
    } else {
        "下载失败。".into()
    }
}

fn next_output_path(dir: &Path, kind: &MediaKind, fmp4: bool) -> PathBuf {
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    dir.join(format!(
        "media-hub-capture-{stamp}.{}",
        output_extension(kind, fmp4)
    ))
}

pub(crate) fn download_hls(
    client: &Client,
    url: &str,
    referer: &str,
    cookie: &str,
    dest_dir: &Path,
    depth: u8,
) -> Result<PathBuf, String> {
    if depth > 3 {
        return Err("播放列表嵌套过深。".into());
    }
    let body = fetch_bytes(client, url, referer, cookie, MAX_PLAYLIST_BYTES)?;
    let text = String::from_utf8(body).map_err(|_| "播放列表不是文本。".to_string())?;
    match parse_m3u8(&text, url)? {
        HlsPlan::Master { variants } => {
            let variant = best_variant(&variants).ok_or_else(|| "播放列表里没有清晰度。".to_string())?;
            download_hls(client, &variant.uri, referer, cookie, dest_dir, depth + 1)
        }
        HlsPlan::Media { segments, fmp4 } => {
            let path = next_output_path(dest_dir, &MediaKind::Hls, fmp4);
            let mut file = File::create(&path).map_err(|_| "无法创建下载文件。".to_string())?;
            let mut written = 0_u64;
            let result = (|| {
                for segment in segments {
                    append_url_to_file(client, &segment, referer, cookie, &mut file, &mut written)?;
                }
                file.flush().map_err(|_| "无法写入下载文件。".to_string())
            })();
            if result.is_err() {
                let _ = fs::remove_file(&path);
            }
            result?;
            Ok(path)
        }
    }
}

pub(crate) fn download_file(client: &Client, url: &str, referer: &str, cookie: &str, dest_dir: &Path) -> Result<PathBuf, String> {
    let path = next_output_path(dest_dir, &MediaKind::File, false);
    let mut file = File::create(&path).map_err(|_| "无法创建下载文件。".to_string())?;
    let mut written = 0_u64;
    let result = append_url_to_file(client, url, referer, cookie, &mut file, &mut written)
        .and_then(|_| file.flush().map_err(|_| "无法写入下载文件。".to_string()));
    if result.is_err() {
        let _ = fs::remove_file(&path);
    }
    result?;
    Ok(path)
}

fn sniff_kind(client: &Client, url: &str, referer: &str, cookie: &str) -> Result<MediaKind, String> {
    if let Some(kind) = classify_media_url(url) {
        return Ok(kind);
    }
    let body = fetch_bytes(client, url, referer, cookie, 64 * 1024)?;
    if looks_like_m3u8(&body) {
        return Ok(MediaKind::Hls);
    }
    Err("不是可下载的视频地址。".into())
}

fn is_under_dir(path: &Path, dir: &Path) -> bool {
    let Ok(path) = path.canonicalize() else {
        return false;
    };
    let Ok(dir) = dir.canonicalize() else {
        return false;
    };
    path.starts_with(dir)
}

fn reveal_path(path: &Path) -> Result<(), String> {
    #[cfg(windows)]
    {
        std::process::Command::new("explorer")
            .arg(format!("/select,{}", path.display()))
            .spawn()
            .map_err(|_| "无法打开文件夹。".to_string())?;
        return Ok(());
    }
    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .args(["-R", &path.display().to_string()])
            .spawn()
            .map_err(|_| "无法打开文件夹。".to_string())?;
        return Ok(());
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        let _ = path;
        Err("当前系统不能打开下载目录。".into())
    }
}

async fn collect_snapshot(app: &AppHandle) -> Result<CaptureSnapshot, String> {
    let window = app
        .get_webview_window(CAPTURE_LABEL)
        .ok_or_else(|| "还没有打开抓取页。".to_string())?;
    inject_hook(&window);
    let (tx, rx) = mpsc::channel();
    window
        .eval_with_callback(LIST_JS, move |value| {
            let _ = tx.send(value);
        })
        .map_err(|_| "无法读取抓取结果。".to_string())?;
    let raw = tauri::async_runtime::spawn_blocking(move || {
        rx.recv_timeout(Duration::from_secs(4))
            .map_err(|_| "读取抓取结果超时。".to_string())
    })
    .await
    .map_err(|_| "读取抓取结果失败。".to_string())??;
    parse_snapshot(&raw)
}

async fn wait_for_items(app: &AppHandle) -> Result<CaptureSnapshot, String> {
    let started = Instant::now();
    let mut last = CaptureSnapshot {
        page: String::new(),
        items: Vec::new(),
    };
    let mut changed_at = Instant::now();
    let mut seen = false;
    while started.elapsed() < WAIT_MEDIA_FOR {
        match collect_snapshot(app).await {
            Ok(snapshot) => {
                if snapshot.items != last.items {
                    last = snapshot;
                    changed_at = Instant::now();
                    seen = !last.items.is_empty();
                }
            }
            Err(error) if seen => return Err(error),
            Err(_) => {}
        }
        if seen && changed_at.elapsed() >= WAIT_QUIET_FOR {
            return Ok(last);
        }
        let _ = tauri::async_runtime::spawn_blocking(|| std::thread::sleep(WAIT_POLL_EVERY)).await;
    }
    if last.items.is_empty() {
        Err("页面里没有明文 m3u8/mp4。可能是登录墙、同意页，或分片加密。".into())
    } else {
        Ok(last)
    }
}

async fn download_selected(app: &AppHandle, url: &str) -> Result<CaptureDownload, String> {
    let media_url = url.trim().to_string();
    if classify_media_url(&media_url).is_none() && !media_url.starts_with("http") {
        return Err("不是可下载的视频地址。".into());
    }
    let window = app
        .get_webview_window(CAPTURE_LABEL)
        .ok_or_else(|| "还没有打开抓取页。".to_string())?;
    let referer = window
        .url()
        .ok()
        .map(|url| url.to_string())
        .unwrap_or_default();
    let dest_dir = capture_output_dir(app)?;
    let cookie_url = media_url.clone();
    let cookie_window = window.clone();
    let cookie = tauri::async_runtime::spawn_blocking(move || cookie_header(&cookie_window, &cookie_url))
        .await
        .unwrap_or_default();

    tauri::async_runtime::spawn_blocking(move || {
        let client = http_client()?;
        let kind = sniff_kind(&client, &media_url, &referer, &cookie)?;
        let path = match kind {
            MediaKind::Hls => download_hls(&client, &media_url, &referer, &cookie, &dest_dir, 0)?,
            MediaKind::File => download_file(&client, &media_url, &referer, &cookie, &dest_dir)?,
        };
        Ok(CaptureDownload {
            kind: match kind {
                MediaKind::Hls => "hls".into(),
                MediaKind::File => "file".into(),
            },
            path: path.to_string_lossy().into_owned(),
            share_import_url: match kind {
                MediaKind::File => Some(media_url),
                MediaKind::Hls => None,
            },
        })
    })
    .await
    .map_err(|_| "下载失败。".to_string())?
}

async fn capture_and_download(app: &AppHandle, url: &str) -> Result<CaptureDownload, String> {
    open_capture(app, url)?;
    let snapshot = match wait_for_items(app).await {
        Ok(snapshot) => snapshot,
        Err(error) => {
            close_capture(app);
            return Err(error);
        }
    };
    let item = match pick_auto_item(&snapshot.items) {
        Some(item) => item.clone(),
        None => {
            close_capture(app);
            return Err("页面里没有明文 m3u8/mp4。可能是登录墙、同意页，或分片加密。".into());
        }
    };
    match download_selected(app, &item.url).await {
        Ok(result) => Ok(result),
        Err(error) => {
            close_capture(app);
            Err(error)
        }
    }
}

#[tauri::command]
pub async fn open_page_capture(app: AppHandle, url: String) -> Result<(), String> {
    let _ = capture_and_download(&app, url.trim()).await?;
    Ok(())
}

#[tauri::command]
pub async fn list_page_capture(app: AppHandle) -> Result<CaptureSnapshot, String> {
    collect_snapshot(&app).await
}

#[tauri::command]
pub async fn download_page_capture(app: AppHandle, url: String) -> Result<CaptureDownload, String> {
    download_selected(&app, &url).await
}

#[tauri::command]
pub async fn run_page_capture(app: AppHandle, url: String) -> Result<CaptureDownload, String> {
    let result = capture_and_download(&app, url.trim()).await?;
    close_capture(&app);
    Ok(result)
}

#[tauri::command]
pub async fn reveal_page_capture(app: AppHandle, path: String) -> Result<(), String> {
    let dest_dir = capture_output_dir(&app)?;
    let file = PathBuf::from(path);
    if !file.is_file() || !is_under_dir(&file, &dest_dir) {
        return Err("找不到下载文件。".into());
    }
    reveal_path(&file)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn snapshot_keeps_only_media_urls() {
        let snapshot = sanitize_snapshot(CaptureSnapshot {
            page: "https://site.example/watch".into(),
            items: vec![
                CaptureItem {
                    url: "https://cdn.example/a.m3u8".into(),
                    kind: "hls".into(),
                },
                CaptureItem {
                    url: "javascript:alert(1)".into(),
                    kind: "file".into(),
                },
                CaptureItem {
                    url: "https://cdn.example/a.mp4".into(),
                    kind: "hls".into(),
                },
            ],
        });
        assert_eq!(snapshot.items.len(), 2);
        assert_eq!(snapshot.items[0].kind, "file");
        assert_eq!(snapshot.items[1].kind, "hls");
    }

    #[test]
    fn parse_snapshot_unwraps_stringified_json() {
        let raw = r#""{\"page\":\"https://site.example\",\"items\":[{\"url\":\"https://cdn.example/a.mp4\",\"kind\":\"file\"}]}""#;
        let snapshot = parse_snapshot(raw).expect("parse");
        assert_eq!(snapshot.items.len(), 1);
        assert_eq!(snapshot.items[0].kind, "file");
    }

    #[test]
    fn rejects_non_http_capture_pages() {
        assert!(parse_http_url("javascript:alert(1)").is_err());
        assert!(parse_http_url("file:///tmp/x").is_err());
        assert!(parse_http_url("https://site.example/watch").is_ok());
    }

    #[test]
    fn auto_pick_prefers_latest_hls_then_file() {
        let items = vec![
            CaptureItem {
                url: "https://cdn.example/ad.mp4".into(),
                kind: "file".into(),
            },
            CaptureItem {
                url: "https://cdn.example/master.m3u8".into(),
                kind: "hls".into(),
            },
            CaptureItem {
                url: "https://cdn.example/media.m3u8".into(),
                kind: "hls".into(),
            },
        ];
        assert_eq!(
            pick_auto_item(&items).map(|item| item.url.as_str()),
            Some("https://cdn.example/media.m3u8")
        );
        let files_only = vec![
            CaptureItem {
                url: "https://cdn.example/preview.mp4".into(),
                kind: "file".into(),
            },
            CaptureItem {
                url: "https://cdn.example/clip.mp4".into(),
                kind: "file".into(),
            },
        ];
        assert_eq!(
            pick_auto_item(&files_only).map(|item| item.url.as_str()),
            Some("https://cdn.example/clip.mp4")
        );
        assert!(pick_auto_item(&[]).is_none());
    }

    fn spawn_capture_fixture() -> (String, std::thread::JoinHandle<()>) {
        use std::io::{Read, Write};
        use std::net::TcpListener;

        let listener = TcpListener::bind("127.0.0.1:0").expect("bind");
        let port = listener.local_addr().expect("addr").port();
        let handle = std::thread::spawn(move || {
            for incoming in listener.incoming().take(24) {
                let Ok(mut stream) = incoming else { continue };
                let _ = stream.set_read_timeout(Some(Duration::from_secs(2)));
                let mut data = Vec::new();
                let mut buf = [0_u8; 1024];
                loop {
                    match stream.read(&mut buf) {
                        Ok(0) => break,
                        Ok(n) => {
                            data.extend_from_slice(&buf[..n]);
                            if data.windows(4).any(|window| window == b"\r\n\r\n") {
                                break;
                            }
                        }
                        Err(_) => break,
                    }
                }
                let req = String::from_utf8_lossy(&data);
                let path = req
                    .lines()
                    .next()
                    .unwrap_or_default()
                    .split_whitespace()
                    .nth(1)
                    .unwrap_or("/");
                let (status, content_type, body): (&str, &str, Vec<u8>) = match path {
                    "/master.m3u8" => (
                        "200 OK",
                        "application/vnd.apple.mpegurl",
                        b"#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000\nmedia.m3u8\n".to_vec(),
                    ),
                    "/media.m3u8" => (
                        "200 OK",
                        "application/vnd.apple.mpegurl",
                        b"#EXTM3U\n#EXTINF:1,\nseg0.ts\n#EXTINF:1,\nseg1.ts\n".to_vec(),
                    ),
                    "/locked.m3u8" => (
                        "200 OK",
                        "application/vnd.apple.mpegurl",
                        b"#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXTINF:1,\nseg0.ts\n".to_vec(),
                    ),
                    "/seg0.ts" => ("200 OK", "video/mp2t", b"SEG0".to_vec()),
                    "/seg1.ts" => ("200 OK", "video/mp2t", b"SEG1".to_vec()),
                    "/clip.mp4" => ("200 OK", "video/mp4", b"ftypmp4".to_vec()),
                    "/denied.mp4" => ("403 Forbidden", "text/plain", b"no".to_vec()),
                    _ => ("404 Not Found", "text/plain", b"missing".to_vec()),
                };
                let header = format!(
                    "HTTP/1.1 {status}\r\nContent-Length: {}\r\nContent-Type: {content_type}\r\nConnection: close\r\n\r\n",
                    body.len()
                );
                let _ = stream.write_all(header.as_bytes());
                let _ = stream.write_all(&body);
            }
        });
        (format!("http://127.0.0.1:{port}"), handle)
    }

    #[test]
    fn downloads_master_hls_and_concats_segments() {
        let (base, _server) = spawn_capture_fixture();
        let dir = std::env::temp_dir().join(format!("media-hub-capture-test-{}", std::process::id()));
        fs::create_dir_all(&dir).expect("dir");
        let client = http_client().expect("client");
        let path = download_hls(&client, &format!("{base}/master.m3u8"), &base, "", &dir, 0).expect("hls");
        assert_eq!(fs::read(&path).expect("read"), b"SEG0SEG1");
        let _ = fs::remove_file(&path);
        let _ = fs::remove_dir(&dir);
    }

    #[test]
    fn skips_encrypted_hls_without_writing_output() {
        let (base, _server) = spawn_capture_fixture();
        let dir = std::env::temp_dir().join(format!("media-hub-capture-locked-{}", std::process::id()));
        fs::create_dir_all(&dir).expect("dir");
        let client = http_client().expect("client");
        let err = download_hls(&client, &format!("{base}/locked.m3u8"), &base, "", &dir, 0).unwrap_err();
        assert_eq!(err, "加密分片，跳过。");
        assert!(fs::read_dir(&dir).expect("list").next().is_none());
        let _ = fs::remove_dir(&dir);
    }

    #[test]
    fn downloads_direct_file_and_explains_forbidden() {
        let (base, _server) = spawn_capture_fixture();
        let dir = std::env::temp_dir().join(format!("media-hub-capture-file-{}", std::process::id()));
        fs::create_dir_all(&dir).expect("dir");
        let client = http_client().expect("client");
        let path = download_file(&client, &format!("{base}/clip.mp4"), &base, "", &dir).expect("file");
        assert_eq!(fs::read(&path).expect("read"), b"ftypmp4");
        let err = download_file(&client, &format!("{base}/denied.mp4"), &base, "", &dir).unwrap_err();
        assert!(err.contains("Cookie") || err.contains("Referer"));
        let _ = fs::remove_file(&path);
        let _ = fs::remove_dir_all(&dir);
    }
}
