import { formatPlaybackClock } from './libraryPlayback'

export function handoffStatusText(playing: boolean, currentSeconds: number, durationSeconds: number) {
  const state = playing ? '正在 mpv 窗口播放' : '已暂停'
  if (durationSeconds > 0) {
    return `${state} · ${formatPlaybackClock(currentSeconds)} / ${formatPlaybackClock(durationSeconds)}`
  }
  return state
}

/// mpv 窗口被关掉才算播完；开播前后端还没记上进程，不能当成结束。
export function handoffEnded(seenRunning: boolean, running?: boolean) {
  return seenRunning && running === false
}

/// 接近片尾才自动播下一个；中途关掉 mpv 仍结束播放器。
export function playbackNearEnd(time: number, duration: number) {
  if (!Number.isFinite(time) || !Number.isFinite(duration) || duration < 2) return false
  return time >= duration - 2 || time / duration >= 0.92
}
