export function isHlsStream(streamUrl: string) {
  try {
    return new URL(streamUrl).pathname.toLowerCase().endsWith('.m3u8')
  } catch {
    return false
  }
}
