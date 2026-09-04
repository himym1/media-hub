use tauri::webview::NewWindowResponse;
use tauri::{AppHandle, Manager, Url, WebviewUrl, WebviewWindowBuilder};

use crate::is_supported_playback_url;

pub const BROWSER_LABEL: &str = "browser";

fn window_title(title: Option<&str>) -> String {
    let trimmed = title.unwrap_or("").replace(['\n', '\r'], " ");
    let trimmed = trimmed.trim();
    if trimmed.is_empty() || trimmed.len() > 80 {
        "Media Hub".into()
    } else {
        trimmed.to_string()
    }
}

fn parse_http_url(url: &str) -> Result<Url, String> {
    if !is_supported_playback_url(url) {
        return Err("unsupported url".into());
    }
    Url::parse(url).map_err(|_| "unsupported url".into())
}

pub fn open_browser(app: &AppHandle, url: &str, title: Option<&str>) -> Result<(), String> {
    let parsed = parse_http_url(url)?;
    let title = window_title(title);
    if let Some(existing) = app.get_webview_window(BROWSER_LABEL) {
        existing
            .navigate(parsed)
            .map_err(|_| "无法在应用内打开页面。".to_string())?;
        let _ = existing.set_title(&title);
        let _ = existing.show();
        let _ = existing.set_focus();
        return Ok(());
    }
    let Some(main) = app.get_webview_window("main") else {
        return Err("找不到应用窗口。".into());
    };
    let handle = app.clone();
    let builder = WebviewWindowBuilder::new(app, BROWSER_LABEL, WebviewUrl::External(parsed))
        .title(title)
        .inner_size(1280.0, 800.0)
        .parent(&main)
        .map_err(|_| "无法在应用内打开页面。".to_string())?
        .on_document_title_changed(|window, document_title| {
            let next = window_title(Some(&document_title));
            let _ = window.set_title(&next);
        })
        .on_new_window(move |url, _features| {
            let _ = open_browser(&handle, url.as_str(), None);
            NewWindowResponse::Deny
        });
    builder
        .build()
        .map_err(|_| "无法在应用内打开页面。".to_string())?;
    Ok(())
}

#[tauri::command]
pub async fn open_in_app(app: AppHandle, url: String, title: Option<String>) -> Result<(), String> {
    open_browser(&app, &url, title.as_deref())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_non_http_browser_urls() {
        assert!(parse_http_url("javascript:alert(1)").is_err());
        assert!(parse_http_url("file:///tmp/x").is_err());
        assert!(parse_http_url("https://emby.example/item\nnext").is_err());
    }

    #[test]
    fn accepts_https_browser_urls() {
        assert!(parse_http_url("https://emby.example/web/index.html#!/item?id=1").is_ok());
    }

    #[test]
    fn titles_stay_short_and_plain() {
        assert_eq!(window_title(None), "Media Hub");
        assert_eq!(window_title(Some("  在 Emby 打开  ")), "在 Emby 打开");
        assert_eq!(window_title(Some("a\nb")), "a b");
    }
}
