export const playbackRates = [0.5, 0.75, 1, 1.25, 1.5, 2]
export const playbackSkipSeconds = 10
export const pictureZoomMin = 0.5
export const pictureZoomMax = 3
export const pictureZoomStep = 0.1

export const playerAspectModes = [
  { id: 'fit', label: '自适应', description: '保持比例，完整显示' },
  { id: 'zoom', label: '铺满', description: '裁切边缘，填满屏幕' },
  { id: 'fixed-width', label: '宽度优先', description: '宽度铺满，高度按比例' },
  { id: 'fixed-height', label: '高度优先', description: '高度铺满，宽度按比例' },
  { id: 'fill', label: '拉伸', description: '拉伸填满，可能变形' },
] as const

export type PlayerAspectId = (typeof playerAspectModes)[number]['id']

export function isPlayerAspectId(value: string): value is PlayerAspectId {
  return playerAspectModes.some((mode) => mode.id === value)
}

export function clampPictureZoom(value: number) {
  if (!Number.isFinite(value)) return 1
  const stepped = Math.round(value / pictureZoomStep) * pictureZoomStep
  return Math.min(pictureZoomMax, Math.max(pictureZoomMin, Number(stepped.toFixed(1))))
}

export function formatPictureZoom(value: number) {
  return `${Math.round(clampPictureZoom(value) * 100)}%`
}

export function stepPictureZoom(current: number, direction: 1 | -1) {
  return clampPictureZoom(current + direction * pictureZoomStep)
}

export function nextPlayerAspect(current: PlayerAspectId): PlayerAspectId {
  const index = playerAspectModes.findIndex((mode) => mode.id === current)
  const next = index < 0 ? 0 : (index + 1) % playerAspectModes.length
  return playerAspectModes[next].id
}

export function playerAspectClassName(id: PlayerAspectId) {
  return `is-aspect-${id}`
}

export function linearZoomFromLog(logZoom: number) {
  if (!Number.isFinite(logZoom)) return 1
  return clampPictureZoom(2 ** logZoom)
}

export function logZoomFromLinear(value: number) {
  return Math.log2(clampPictureZoom(value))
}

export function shouldAutoHidePlayerChrome(
  _nativeActive: boolean,
  playing: boolean,
  pinned: boolean,
  hasError: boolean,
  nativePointerReady = true,
) {
  return playing && !pinned && !hasError && nativePointerReady
}

export type NativePointer = {
  x: number
  y: number
  hover: boolean
}

export function nativePointerFromStatus(status: {
  mouseX?: number
  mouseY?: number
  cursorHover?: boolean
}): NativePointer | null {
  if (typeof status.mouseX !== 'number' || typeof status.mouseY !== 'number') return null
  return {
    x: status.mouseX,
    y: status.mouseY,
    hover: Boolean(status.cursorHover),
  }
}

export function shouldRevealChromeFromNativePointer(previous: NativePointer | null, next: NativePointer) {
  if (!next.hover) return false
  if (!previous) return true
  return !previous.hover || previous.x !== next.x || previous.y !== next.y
}

export function shouldArmNativeChromeHide(previous: NativePointer | null, next: NativePointer) {
  return Boolean(previous && next.hover && (previous.x !== next.x || previous.y !== next.y))
}

export function playerChromeInsets(
  visible: boolean,
  toolbar?: { getBoundingClientRect: () => { height: number } } | null,
  chrome?: { getBoundingClientRect: () => { height: number } } | null,
) {
  if (!visible) return { top: 0, bottom: 0 }
  return {
    top: Math.max(56, Math.round(toolbar?.getBoundingClientRect().height ?? 72)),
    bottom: Math.max(96, Math.round(chrome?.getBoundingClientRect().height ?? 148)),
  }
}

export function nativeEmbedRect(
  hole: { getBoundingClientRect: () => { left: number; top: number; width: number; height: number } },
  insets: { top: number; bottom: number },
) {
  const rect = hole.getBoundingClientRect()
  const top = Math.max(0, insets.top)
  const bottom = Math.max(0, insets.bottom)
  return {
    x: rect.left,
    y: rect.top + top,
    width: Math.max(8, rect.width),
    height: Math.max(8, rect.height - top - bottom),
  }
}

export const playerClickDelayMs = 280

export type PlayerClickTimer = { current: number | null }

export function schedulePlayerClick(timer: PlayerClickTimer, onClick: () => void, delayMs = playerClickDelayMs) {
  if (timer.current != null) window.clearTimeout(timer.current)
  timer.current = window.setTimeout(() => {
    timer.current = null
    onClick()
  }, delayMs)
}

export function cancelScheduledPlayerClick(timer: PlayerClickTimer) {
  if (timer.current == null) return
  window.clearTimeout(timer.current)
  timer.current = null
}
