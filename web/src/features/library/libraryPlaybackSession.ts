type SessionEvent = 'started' | 'progress' | 'stopped'

type ReportSession = (
  sessionId: string,
  event: SessionEvent,
  positionMs: number,
  paused: boolean,
) => Promise<unknown>

export function attachPlaybackSession(
  sessionId: string | undefined,
  video: HTMLVideoElement,
  report: ReportSession,
) {
  if (!sessionId) return () => {}
  let started = false
  const positionMs = () => Math.max(0, Math.floor((video.currentTime || 0) * 1000))
  const send = (event: SessionEvent, paused: boolean) => {
    void report(sessionId, event, positionMs(), paused)
  }
  const onPlaying = () => {
    if (!started) {
      started = true
      send('started', false)
      return
    }
    send('progress', false)
  }
  const onPause = () => {
    if (started) send('progress', true)
  }
  video.addEventListener('playing', onPlaying)
  video.addEventListener('pause', onPause)
  const timer = globalThis.setInterval(() => {
    if (started) send('progress', video.paused)
  }, 15_000)
  return () => {
    globalThis.clearInterval(timer)
    video.removeEventListener('playing', onPlaying)
    video.removeEventListener('pause', onPause)
    if (started) send('stopped', true)
  }
}
