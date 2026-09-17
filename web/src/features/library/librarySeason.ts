/**
 * 剧集分季与分集展示辅助函数
 */

export function seasonDisplayName(seasonNumber: number): string {
  if (seasonNumber === 0) {
    return '特别篇'
  }
  return `第 ${seasonNumber} 季`
}

export function calculateEpisodeProgress(playbackPositionMs?: number, runtimeMinutes?: number): number {
  if (!playbackPositionMs || playbackPositionMs <= 0) {
    return 0
  }
  if (!runtimeMinutes || runtimeMinutes <= 0) {
    return 15 // 未知时长但有播放进度时显示基础进度条
  }
  const totalMs = runtimeMinutes * 60_000
  const percent = Math.round((playbackPositionMs / totalMs) * 100)
  return Math.min(100, Math.max(1, percent))
}

export function episodeDisplayNumber(_season?: number, episode?: number): string {
  if (episode && episode > 0) {
    return `第 ${episode} 集`
  }
  return '分集'
}
