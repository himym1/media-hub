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
