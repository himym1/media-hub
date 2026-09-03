type DecodableVideo = HTMLVideoElement & {
  webkitAudioDecodedByteCount?: number
  webkitVideoDecodedByteCount?: number
  mozHasAudio?: boolean
  audioTracks?: { length: number }
  captureStream?: () => MediaStream
}

/** True when video is advancing but the browser is not decoding any audio. */
export function looksLikeSilentDirectPlay(video: HTMLVideoElement) {
  const haveCurrentData = typeof HTMLMediaElement !== 'undefined' ? HTMLMediaElement.HAVE_CURRENT_DATA : 2
  if (video.paused || video.ended || video.readyState < haveCurrentData) return false
  if (video.muted || video.volume === 0) return false
  if (!Number.isFinite(video.currentTime) || video.currentTime < 2.5) return false

  const media = video as DecodableVideo
  const audioBytes = media.webkitAudioDecodedByteCount
  const videoBytes = media.webkitVideoDecodedByteCount
  if (typeof audioBytes === 'number' && typeof videoBytes === 'number') {
    return videoBytes > 0 && audioBytes === 0
  }
  if (typeof media.mozHasAudio === 'boolean') {
    return media.mozHasAudio === false
  }
  if (media.audioTracks && typeof media.audioTracks.length === 'number') {
    return media.audioTracks.length === 0
  }
  try {
    const capture = media.captureStream?.()
    if (capture) {
      const hasAudio = capture.getAudioTracks().length > 0
      const hasVideo = capture.getVideoTracks().length > 0
      if (hasVideo && !hasAudio) return true
    }
  } catch {
    // captureStream can throw when the element is not ready; ignore.
  }
  return false
}
