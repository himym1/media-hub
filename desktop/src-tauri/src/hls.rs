use url::Url;

pub const MAX_PLAYLIST_BYTES: usize = 8 * 1024 * 1024;
pub const MAX_FILE_BYTES: u64 = 8 * 1024 * 1024 * 1024;
pub const MAX_SEGMENTS: usize = 4000;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MediaKind {
    Hls,
    File,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum HlsPlan {
    Master { variants: Vec<HlsVariant> },
    Media { segments: Vec<String>, fmp4: bool },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HlsVariant {
    pub bandwidth: u64,
    pub uri: String,
}

pub fn classify_media_url(url: &str) -> Option<MediaKind> {
    let trimmed = url.trim();
    if trimmed.is_empty() || trimmed.len() > 16_384 {
        return None;
    }
    if trimmed.contains('\n') || trimmed.contains('\r') || trimmed.contains('\0') {
        return None;
    }
    if !(trimmed.starts_with("https://") || trimmed.starts_with("http://")) {
        return None;
    }
    let lower = trimmed.to_ascii_lowercase();
    if lower.contains(".m3u8") || lower.contains("m3u8") {
        return Some(MediaKind::Hls);
    }
    let path = lower.split(['?', '#']).next().unwrap_or(lower.as_str());
    if path.ends_with(".mp4")
        || path.ends_with(".m4v")
        || path.ends_with(".mkv")
        || path.ends_with(".webm")
        || path.ends_with(".mov")
    {
        return Some(MediaKind::File);
    }
    None
}

pub fn looks_like_m3u8(body: &[u8]) -> bool {
    let start = std::str::from_utf8(body.get(..32).unwrap_or(body)).unwrap_or("");
    start.trim_start().starts_with("#EXTM3U")
}

pub fn resolve_playlist_uri(base: &str, relative: &str) -> Result<String, String> {
    let relative = relative.trim();
    if relative.is_empty() {
        return Err("播放列表里有空地址。".into());
    }
    if relative.starts_with("https://") || relative.starts_with("http://") {
        return Ok(relative.to_string());
    }
    Url::parse(base)
        .map_err(|_| "播放列表地址无效。".to_string())?
        .join(relative)
        .map(|joined| joined.to_string())
        .map_err(|_| "无法解析分片地址。".to_string())
}

pub fn parse_m3u8(body: &str, playlist_url: &str) -> Result<HlsPlan, String> {
    if body.len() > MAX_PLAYLIST_BYTES {
        return Err("播放列表过大。".into());
    }
    if !body.trim_start().starts_with("#EXTM3U") {
        return Err("不是 HLS 播放列表。".into());
    }

    let mut variants: Vec<HlsVariant> = Vec::new();
    let mut segments: Vec<String> = Vec::new();
    let mut fmp4 = false;
    let mut pending_stream_inf: Option<u64> = None;
    let mut expect_segment = false;

    for raw in body.lines() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        if let Some(rest) = line.strip_prefix("#EXT-X-KEY:") {
            if key_is_encrypted(rest) {
                return Err("加密分片，跳过。".into());
            }
            continue;
        }
        if line.starts_with("#EXT-X-BYTERANGE")
            || (line.starts_with("#EXT-X-MAP:") && line.to_ascii_uppercase().contains("BYTERANGE="))
        {
            return Err("不支持按字节范围切的分片。".into());
        }
        if let Some(rest) = line.strip_prefix("#EXT-X-MAP:") {
            let uri = map_uri(rest).ok_or_else(|| "初始化片段地址无效。".to_string())?;
            segments.push(resolve_playlist_uri(playlist_url, &uri)?);
            fmp4 = true;
            continue;
        }
        if let Some(rest) = line.strip_prefix("#EXT-X-STREAM-INF:") {
            pending_stream_inf = Some(parse_bandwidth(rest));
            expect_segment = false;
            continue;
        }
        if line.starts_with("#EXTINF:") {
            expect_segment = true;
            pending_stream_inf = None;
            continue;
        }
        if line.starts_with('#') {
            continue;
        }
        if let Some(bandwidth) = pending_stream_inf.take() {
            variants.push(HlsVariant {
                bandwidth,
                uri: resolve_playlist_uri(playlist_url, line)?,
            });
            continue;
        }
        if expect_segment {
            if segments.len() >= MAX_SEGMENTS {
                return Err("分片太多，跳过。".into());
            }
            let uri = resolve_playlist_uri(playlist_url, line)?;
            let lower = uri.to_ascii_lowercase();
            if lower.contains(".m4s") || lower.contains(".mp4") {
                fmp4 = true;
            }
            segments.push(uri);
            expect_segment = false;
        }
    }

    if !variants.is_empty() {
        variants.sort_by_key(|variant| variant.bandwidth);
        return Ok(HlsPlan::Master { variants });
    }
    if segments.is_empty() {
        return Err("播放列表里没有分片。".into());
    }
    Ok(HlsPlan::Media { segments, fmp4 })
}

pub fn best_variant(variants: &[HlsVariant]) -> Option<&HlsVariant> {
    variants.iter().max_by_key(|variant| variant.bandwidth)
}

fn key_is_encrypted(attrs: &str) -> bool {
    let upper = attrs.to_ascii_uppercase();
    if upper.contains("METHOD=NONE") {
        return false;
    }
    upper.contains("METHOD=AES-128") || upper.contains("METHOD=SAMPLE-AES") || upper.contains("METHOD=SAMPLE-AES-CTR")
}

fn parse_bandwidth(attrs: &str) -> u64 {
    attrs
        .split(',')
        .find_map(|part| {
            let part = part.trim();
            part.strip_prefix("BANDWIDTH=")
                .or_else(|| part.strip_prefix("bandwidth="))
                .and_then(|value| value.trim().parse().ok())
        })
        .unwrap_or(0)
}

fn map_uri(attrs: &str) -> Option<String> {
    let marker = "URI=\"";
    let start = attrs.find(marker)? + marker.len();
    let rest = attrs.get(start..)?;
    let end = rest.find('"')?;
    Some(rest[..end].to_string())
}

pub fn output_extension(kind: &MediaKind, fmp4: bool) -> &'static str {
    match kind {
        MediaKind::File => "mp4",
        MediaKind::Hls if fmp4 => "mp4",
        MediaKind::Hls => "ts",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_hls_and_files() {
        assert_eq!(
            classify_media_url("https://cdn.example/play.m3u8?token=1"),
            Some(MediaKind::Hls)
        );
        assert_eq!(
            classify_media_url("https://cdn.example/clip.mp4"),
            Some(MediaKind::File)
        );
        assert_eq!(classify_media_url("https://cdn.example/page.html"), None);
        assert_eq!(classify_media_url("javascript:alert(1)"), None);
    }

    #[test]
    fn parses_media_playlist_relative_segments() {
        let body = "#EXTM3U\n#EXTINF:4,\nseg0.ts\n#EXTINF:4,\nseg1.ts\n";
        let plan = parse_m3u8(body, "https://cdn.example/hls/index.m3u8").expect("parse");
        assert_eq!(
            plan,
            HlsPlan::Media {
                segments: vec![
                    "https://cdn.example/hls/seg0.ts".into(),
                    "https://cdn.example/hls/seg1.ts".into(),
                ],
                fmp4: false,
            }
        );
    }

    #[test]
    fn master_playlist_keeps_bandwidth_order() {
        let body = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=5000000\nhigh.m3u8\n";
        let plan = parse_m3u8(body, "https://cdn.example/master.m3u8").expect("parse");
        match plan {
            HlsPlan::Master { variants } => {
                assert_eq!(best_variant(&variants).map(|v| v.uri.as_str()), Some("https://cdn.example/high.m3u8"));
            }
            _ => panic!("expected master"),
        }
    }

    #[test]
    fn skips_encrypted_playlists() {
        let body = "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"https://cdn.example/key\"\n#EXTINF:4,\nseg.ts\n";
        assert_eq!(parse_m3u8(body, "https://cdn.example/index.m3u8").unwrap_err(), "加密分片，跳过。");
    }

    #[test]
    fn prepends_fmp4_init() {
        let body = "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2,\nseg1.m4s\n";
        let plan = parse_m3u8(body, "https://cdn.example/a.m3u8").expect("parse");
        assert_eq!(
            plan,
            HlsPlan::Media {
                segments: vec![
                    "https://cdn.example/init.mp4".into(),
                    "https://cdn.example/seg1.m4s".into(),
                ],
                fmp4: true,
            }
        );
    }

    #[test]
    fn resolve_joins_parent_paths() {
        assert_eq!(
            resolve_playlist_uri("https://cdn.example/a/b/c.m3u8", "../d.ts").expect("join"),
            "https://cdn.example/a/d.ts"
        );
    }
}
