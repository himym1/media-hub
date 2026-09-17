use std::path::Path;
use std::time::Duration;

use reqwest::blocking::multipart;
use reqwest::header::{HeaderMap, HeaderValue, USER_AGENT};
use serde::Deserialize;
use url::Url;

const OSS_UA: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 115Browser/27.0.3.7";
const MAX_UPLOAD_BYTES: u64 = 5 * 1024 * 1024 * 1024;

#[derive(Debug, Clone, Deserialize)]
pub struct OssTicket {
    pub host: String,
    pub object: String,
    pub accessid: String,
    pub policy: String,
    pub signature: String,
    pub callback: String,
    pub target: String,
    pub filename: String,
}

pub fn sanitize_oss_post_url(raw: &str) -> Result<String, String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Err("上传地址无效。".into());
    }
    let with_scheme = if trimmed.contains("://") {
        trimmed.to_string()
    } else {
        format!("https://{}", trimmed.trim_start_matches("//"))
    };
    let mut parsed = Url::parse(&with_scheme).map_err(|_| "上传地址无效。".to_string())?;
    if parsed.scheme() == "http" {
        parsed.set_scheme("https").map_err(|_| "上传地址无效。".to_string())?;
    }
    if parsed.scheme() != "https" || parsed.username() != "" || parsed.password().is_some() {
        return Err("上传地址无效。".into());
    }
    if parsed.query().is_some() || parsed.fragment().is_some() {
        return Err("上传地址无效。".into());
    }
    let path = parsed.path();
    if path != "" && path != "/" {
        return Err("上传地址无效。".into());
    }
    let host = parsed.host_str().unwrap_or_default().to_ascii_lowercase();
    if host.is_empty()
        || !(host.ends_with(".aliyuncs.com") || host.ends_with(".aliyuncs.com.cn"))
    {
        return Err("上传地址无效。".into());
    }
    Ok(format!("https://{host}"))
}

pub fn post_file(path: &Path, ticket: &OssTicket) -> Result<(), String> {
    let host = sanitize_oss_post_url(&ticket.host)?;
    if ticket.object.trim().is_empty()
        || ticket.accessid.trim().is_empty()
        || ticket.policy.trim().is_empty()
        || ticket.signature.trim().is_empty()
        || ticket.callback.trim().is_empty()
        || ticket.target.trim().is_empty()
        || ticket.filename.trim().is_empty()
    {
        return Err("上传凭证不完整。".into());
    }
    let size = std::fs::metadata(path).map_err(|_| "找不到下载文件。".to_string())?.len();
    if size < 1 || size > MAX_UPLOAD_BYTES {
        return Err("文件过大。".into());
    }
    let part = multipart::Part::file(path)
        .map_err(|_| "找不到下载文件。".to_string())?
        .file_name(ticket.filename.clone())
        .mime_str("application/octet-stream")
        .map_err(|_| "无法读取下载文件。".to_string())?;
    let form = multipart::Form::new()
        .text("success_action_status", "200")
        .text("name", ticket.filename.clone())
        .text("target", ticket.target.clone())
        .text("key", ticket.object.clone())
        .text("policy", ticket.policy.clone())
        .text("OSSAccessKeyId", ticket.accessid.clone())
        .text("callback", ticket.callback.clone())
        .text("signature", ticket.signature.clone())
        .part("file", part);
    let mut headers = HeaderMap::new();
    if let Ok(value) = HeaderValue::from_str(OSS_UA) {
        headers.insert(USER_AGENT, value);
    }
    let client = reqwest::blocking::Client::builder()
        .timeout(Duration::from_secs(30 * 60))
        .connect_timeout(Duration::from_secs(20))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "无法开始上传。".to_string())?;
    let response = client
        .post(host)
        .headers(headers)
        .multipart(form)
        .send()
        .map_err(|_| "上传失败。".to_string())?;
    let status = response.status().as_u16();
    if (200..300).contains(&status) {
        return Ok(());
    }
    Err("上传失败。".into())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn oss_host_must_be_https_aliyun() {
        assert_eq!(
            sanitize_oss_post_url("http://bucket.oss-cn-shenzhen.aliyuncs.com").unwrap(),
            "https://bucket.oss-cn-shenzhen.aliyuncs.com"
        );
        assert!(sanitize_oss_post_url("https://evil.example").is_err());
        assert!(sanitize_oss_post_url("https://bucket.oss-cn-shenzhen.aliyuncs.com/evil").is_err());
    }
}
