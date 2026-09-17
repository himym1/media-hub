import { expect, test } from '@playwright/test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const hookPath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../desktop/src-tauri/src/capture.js')

test('capture hook records plaintext hls and file urls', async ({ page }) => {
  await page.setContent('<!doctype html><video src="https://cdn.example/clip.mp4"></video>')
  await page.addScriptTag({ path: hookPath })
  await page.evaluate(async () => {
    await fetch('https://cdn.example/play.m3u8').catch(() => undefined)
  })
  const snapshot = await page.evaluate(() => {
    const collect = (window as Window & { __mhCollectCapture?: () => { items: Array<{ url: string; kind: string }> } }).__mhCollectCapture
    return collect ? collect() : { items: [] }
  })
  expect(snapshot.items.some((item) => item.kind === 'file' && item.url.includes('clip.mp4'))).toBe(true)
  expect(snapshot.items.some((item) => item.kind === 'hls' && item.url.includes('play.m3u8'))).toBe(true)
})
