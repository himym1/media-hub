export function isAssSubtitle(contentType: string, fileName: string) {
  const type = contentType.split(';')[0]?.trim().toLowerCase() ?? ''
  const name = fileName.toLowerCase()
  return type === 'text/x-ssa' || name.endsWith('.ass') || name.endsWith('.ssa')
}

export function srtToVtt(source: string) {
  const text = source.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n').trim()
  if (text.startsWith('WEBVTT')) return `${text}\n`
  const body = text
    .split('\n')
    .map((line) => line.replace(/(\d{1,2}:\d{2}:\d{2}),(\d{3})/g, '$1.$2'))
    .join('\n')
  return `WEBVTT\n\n${body}\n`
}

export function subtitleFileName(disposition: string | null) {
  if (!disposition) return 'subtitle.srt'
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (encoded?.[1]) return decodeURIComponent(encoded[1].trim())
  const quoted = disposition.match(/filename="([^"]+)"/i)
  if (quoted?.[1]) return quoted[1]
  const plain = disposition.match(/filename=([^;]+)/i)
  return plain?.[1]?.trim() || 'subtitle.srt'
}
