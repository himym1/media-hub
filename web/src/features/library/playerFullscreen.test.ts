import { describe, expect, it, vi } from 'vitest'
import {
  documentFullscreenElement,
  exitDocumentFullscreen,
  requestDocumentFullscreen,
  shouldClosePlayerOnEscape,
  toggleDocumentFullscreen,
} from './playerFullscreen'

describe('playerFullscreen', () => {
  it('reads standard and webkit fullscreen elements', () => {
    const element = {} as Element
    expect(documentFullscreenElement({ fullscreenElement: null } as unknown as Document)).toBeNull()
    expect(documentFullscreenElement({ fullscreenElement: element } as unknown as Document)).toBe(element)
    expect(documentFullscreenElement({
      fullscreenElement: null,
      webkitFullscreenElement: element,
    } as unknown as Document)).toBe(element)
  })

  it('prefers the prefixed request when the standard API is missing', async () => {
    const webkitRequestFullscreen = vi.fn(async () => undefined)
    const element = { webkitRequestFullscreen } as unknown as Element
    await requestDocumentFullscreen(element)
    expect(webkitRequestFullscreen).toHaveBeenCalled()
  })

  it('exits through the prefixed document API', async () => {
    const webkitExitFullscreen = vi.fn(async () => undefined)
    await exitDocumentFullscreen({ webkitExitFullscreen } as unknown as Document)
    expect(webkitExitFullscreen).toHaveBeenCalled()
  })

  it('toggles out of an existing fullscreen element', async () => {
    const exitFullscreen = vi.fn(async () => undefined)
    const requestFullscreen = vi.fn(async () => undefined)
    const element = { requestFullscreen } as unknown as Element
    await toggleDocumentFullscreen(element, {
      fullscreenElement: element,
      exitFullscreen,
    } as unknown as Document)
    expect(exitFullscreen).toHaveBeenCalled()
    expect(requestFullscreen).not.toHaveBeenCalled()
  })

  it('keeps Esc on the player until presentation is already windowed', () => {
    expect(shouldClosePlayerOnEscape(true, false)).toBe(false)
    expect(shouldClosePlayerOnEscape(false, true)).toBe(false)
    expect(shouldClosePlayerOnEscape(false, false)).toBe(true)
  })
})
