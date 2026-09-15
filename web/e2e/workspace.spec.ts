import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page, type TestInfo } from '@playwright/test'
import { installApiFixtures } from './apiFixtures'

const runtimeErrors = new WeakMap<Page, string[]>()

test.beforeEach(async ({ page }) => {
  const errors: string[] = []
  runtimeErrors.set(page, errors)
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('console', (message) => {
    const text = message.text()
    if (message.type() === 'error' && text !== 'Failed to load resource: the server responded with a status of 401 (Unauthorized)') errors.push(text)
  })
})

test.afterEach(async ({ page }) => {
  expect(runtimeErrors.get(page) ?? [], 'browser runtime errors').toEqual([])
})

async function expectNoSeriousAccessibilityViolations(page: Page) {
  const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const violations = result.violations.filter((item) => item.impact === 'serious' || item.impact === 'critical')
  expect(violations, JSON.stringify(violations, null, 2)).toEqual([])
}

async function attachScreenshot(page: Page, testInfo: TestInfo, name: string) {
  await testInfo.attach(name, { body: await page.screenshot({ fullPage: true }), contentType: 'image/png' })
}

async function expectNoHorizontalOverflow(page: Page) {
  expect(await page.evaluate(() => {
    const main = document.getElementById('main-content')
    return {
      root: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      main: !main || main.scrollWidth <= main.clientWidth,
    }
  })).toEqual({ root: true, main: true })
}

test('login is keyboard-ready and accessible', async ({ page }, testInfo) => {
  await installApiFixtures(page, { authenticated: false })
  await page.goto('/')

  await expect(page.getByRole('heading', { name: '管理员登录' })).toBeVisible()
  const password = page.locator('#admin-password')
  await expect(password).not.toBeFocused()
  await page.keyboard.press('Tab')
  await expect(password).toBeFocused()
  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'login')
})

test('discovery preserves search state through browser history', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=discover')
  await expect(page.getByRole('heading', { name: '发现', level: 1 })).toBeVisible()
  if (testInfo.project.name === 'desktop') {
    await page.setViewportSize({ width: 1280, height: 800 })
    await expect(page.getByRole('button', { name: '全部类型' })).toBeVisible()
    await expectNoHorizontalOverflow(page)
  }

  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('#main-content')).toBeFocused()

  await page.getByLabel('搜索电影或电视剧').fill('验收影片')
  await page.getByRole('button', { name: '搜索', exact: true }).click()
  await expect(page).toHaveURL(/view=discover.*q=%E9%AA%8C%E6%94%B6%E5%BD%B1%E7%89%87/)
  await expect(page.getByRole('button', { name: /验收影片.*2160p/ })).toBeVisible()
  expect(page.url()).not.toContain('fixture-token')

  await page.getByRole('link', { name: '任务', exact: true }).first().click()
  await expect(page).toHaveURL(/view=transfers.*task=task-1/)
  await page.goBack()
  await expect(page.getByLabel('搜索电影或电视剧')).toHaveValue('验收影片')
  await expect(page.getByRole('button', { name: /验收影片.*2160p/ })).toBeVisible()

  await expectNoHorizontalOverflow(page)
  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'discovery')
})

test('task deep link survives initial list loading', async ({ page }) => {
  await installApiFixtures(page)
  await page.goto('/?view=transfers&task=task-1')
  await expect(page).toHaveURL(/view=transfers.*task=task-1/)
  await expect(page.locator('.task-row[aria-pressed="true"]')).toHaveCount(1)
  await expect(page.getByLabel('任务详情')).toBeVisible()
})

test('failed task delete uses in-page confirm instead of a native dialog', async ({ page }) => {
  let nativeDialog = false
  page.on('dialog', () => {
    nativeDialog = true
  })
  await installApiFixtures(page, { withFailedTransfer: true })
  await page.goto('/?view=transfers&task=task-failed')
  await page.getByRole('button', { name: '删除任务' }).click()
  await expect(page.getByText('删除这条失败任务记录？不会影响 115 / Emby 中的媒体。')).toBeVisible()
  await expect(nativeDialog, 'Tauri and some WebViews swallow window.confirm').toBe(false)
  await page.getByRole('button', { name: '确认删除' }).click()
  await expect(page.getByRole('button', { name: /失败影片/ })).toHaveCount(0)
  await expect(page.getByText('还没有转存任务')).toBeVisible()
})

test('terminal task can be archived and restored without deletion', async ({ page }) => {
  await installApiFixtures(page)
  await page.goto('/?view=transfers&task=task-1')
  await page.getByRole('button', { name: '归档任务' }).click()
  await expect(page.getByText('还没有转存任务')).toBeVisible()

  await page.getByRole('button', { name: '已归档' }).click()
  await expect(page).toHaveURL(/archive=1/)
  await page.getByRole('link', { name: '媒体库', exact: true }).click()
  await expect(page).not.toHaveURL(/archive=1/)
  await page.goBack()
  await expect(page.getByRole('button', { name: '已归档' })).toHaveAttribute('aria-pressed', 'true')
  await page.reload()
  await expect(page.getByRole('button', { name: '已归档' })).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByRole('button', { name: /验收影片/ })).toBeVisible()
  await expect(page.getByRole('button', { name: '恢复到任务列表' })).toBeVisible()
  await page.getByRole('button', { name: '恢复到任务列表' }).click()
  await expect(page.getByText('还没有归档任务')).toBeVisible()

  await page.getByRole('button', { name: '当前', exact: true }).click()
  await expect(page).not.toHaveURL(/archive=1/)
  await expect(page.getByRole('button', { name: /验收影片/ })).toBeVisible()
})

test('library supports browsing item details and safe Emby actions', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=library')
  await expect(page.getByRole('button', { name: '电影', exact: true })).toHaveAttribute('aria-pressed', 'true')
  const libraryItem = page.getByRole('button', { name: /验收影片/ })
  await expect(libraryItem).not.toContainText('TMDB')
  await libraryItem.click()
  await expect(page.getByRole('heading', { name: '验收影片', level: 2 })).toBeVisible()
  await expect(page.getByText('已挂中文', { exact: true })).toBeVisible()
  await expect(page.getByText('用于验证媒体库详情。')).toBeVisible()
  await expect(page.locator('.library-detail').getByText('剧情', { exact: true })).toBeVisible()
  await expect(page.locator('.library-detail').getByText('科幻', { exact: true })).toBeVisible()
  await expect(page.getByText('Acceptance Movie', { exact: true })).toBeHidden()
  await expect(page.getByText('TMDB 编号', { exact: true })).toBeHidden()
  await expect(page.getByText('EMBY', { exact: true })).toHaveCount(0)
  await expect(page.getByText('DETAIL', { exact: true })).toHaveCount(0)
  await expect(page.getByText('Media Hub 不代理或删除媒体文件。', { exact: true })).toHaveCount(0)
  await page.getByText('更多信息', { exact: true }).click()
  await expect(page.getByText('Acceptance Movie', { exact: true })).toBeVisible()
  await expect(page.getByText('TMDB 编号', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: '打开 Emby 网页' })).toHaveCount(0)
  await page.getByRole('button', { name: '刷新元数据' }).click()
  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'library-detail')
  await page.getByRole('link', { name: '任务', exact: true }).click()
  await expect(page).not.toHaveURL(/media=item-1/)
  await page.goBack()
  await expect(page.getByRole('heading', { name: '验收影片', level: 2 })).toBeVisible()
  await expect(page.getByRole('button', { name: '返回媒体列表' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '验收影片', level: 2 })).toBeInViewport()
  await expect(page.getByRole('button', { name: /验收影片/ })).toBeHidden()
  await page.getByRole('button', { name: '返回媒体列表' }).click()
  await expect(page.getByRole('button', { name: /验收影片/ })).toBeVisible()
})

test('library previews and confirms Emby delete without touching 115 files', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=library')
  await page.getByRole('button', { name: /验收影片/ }).click()
  await page.getByRole('button', { name: '从 Emby 删除' }).click()
  await expect(page.getByText('将从 Emby 删除「验收影片」。NAS 上约 1 个 STRM 和同名字幕会被清掉，115 网盘文件不会删除。')).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'library-delete-preview')
  await page.getByRole('button', { name: '取消' }).click()
  await expect(page.getByRole('button', { name: '从 Emby 删除' })).toBeVisible()
  await page.getByRole('button', { name: '从 Emby 删除' }).click()
  await page.getByRole('button', { name: '确认删除' }).click()
  await expect(page.getByRole('heading', { name: '验收影片', level: 2 })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /验收影片/ })).toHaveCount(0)
  await expect(page).not.toHaveURL(/media=item-1/)
  await expect(page.getByText('此媒体库暂无可浏览内容')).toBeVisible()
})

test('library opens Emby from the web instead of an in-page player', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=library')
  await page.getByRole('button', { name: /验收影片/ }).click()
  await expect(page.getByRole('button', { name: '播放', exact: true })).toHaveCount(0)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const embyLink = page.getByRole('link', { name: '在 Emby 打开' })
  await expect(embyLink).toHaveAttribute('href', 'https://emby.example/web/index.html#!/item?id=item-1')
  await expect(embyLink).toHaveAttribute('target', '_blank')
  await expect(page).not.toHaveURL(/play=item-1/)
  await page.goto('/?view=library&library=movie&media=item-1&play=item-1')
  await expect(page.getByRole('heading', { name: '验收影片', level: 2 })).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page).not.toHaveURL(/play=item-1/)
  await attachScreenshot(page, testInfo, 'library-emby-open')
})

test('player view stays out of the web library chrome', async ({ page }) => {
  await installApiFixtures(page)
  await page.goto('/?view=player&play=item-1')
  await expect(page.getByRole('heading', { name: '验收影片' })).toBeVisible()
  await expect(page.getByText('请在桌面应用中播放，或在 Emby 打开。')).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByRole('navigation', { name: '主导航' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: '在 Emby 打开' })).toHaveAttribute('href', 'https://emby.example/web/index.html#!/item?id=item-1')
})

test('library series opens Emby per episode and keeps a page-level play button off the detail', async ({ page }) => {
  await installApiFixtures(page)
  await page.goto('/?view=library')
  await page.getByRole('button', { name: '剧集', exact: true }).click()
  await page.getByRole('button', { name: /验收剧集/ }).click()
  await expect(page.getByRole('heading', { name: '验收剧集', level: 2 })).toBeVisible()
  await expect(page.getByRole('button', { name: '播放', exact: true })).toHaveCount(0)
  await expect(page.getByRole('link', { name: '打开 Emby 网页' })).toHaveCount(0)
  const episode = page.getByRole('link', { name: '在 Emby 打开 第 1 集 · 连接' })
  await expect(episode).toHaveAttribute('href', 'https://emby.example/web/index.html#!/item?id=episode-1')
  await expect(episode).toHaveAttribute('target', '_blank')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page).not.toHaveURL(/play=episode-1/)
})

test('subscription editor keeps advanced rules collapsed behind useful presets', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=subscriptions')
  await expect(page.getByRole('combobox', { name: '质量预设' })).toHaveValue('standard')
  await expect(page.getByText('视频编码', { exact: true })).toBeHidden()
  await expect(page.getByLabel('juying')).toBeHidden()
  await page.getByText('资源来源', { exact: true }).click()
  await expect(page.getByLabel('juying')).toBeVisible()
  await page.getByText('高级规则与媒体身份').click()
  await page.getByRole('combobox', { name: '质量预设' }).selectOption('custom')
  await expect(page.getByText('视频编码', { exact: true })).toBeVisible()
  await attachScreenshot(page, testInfo, 'subscription-editor')
})

test('subscription polling does not overwrite an in-progress edit', async ({ page }) => {
  await installApiFixtures(page, { withSubscription: true })
  await page.goto('/?view=subscriptions')
  await page.getByRole('button', { name: /验收影片.*TMDB 100/ }).click()
  await page.getByLabel('标题', { exact: true }).fill('尚未保存的标题')

  const refreshed = page.waitForResponse((response) => response.request().method() === 'GET' && new URL(response.url()).pathname === '/api/v1/subscriptions')
  await page.getByRole('button', { name: '立即运行' }).click()
  await refreshed
  await expect(page.getByLabel('标题', { exact: true })).toHaveValue('尚未保存的标题')
})


test('settings tabs support arrow keys and protect unsaved provider changes', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=settings&settings=overview')

  const overviewTab = page.getByRole('tab', { name: '概览' })
  await overviewTab.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page).toHaveURL(/settings=providers/)
  await expect(page.getByRole('tab', { name: '服务接入' })).toBeFocused()

  await expect(page.getByLabel('STRM 基址')).toBeVisible()
  await expect(page.getByLabel('STRM 根挂载')).toBeVisible()
  await expect(page.getByLabel('STRM 目标路径').first()).toBeVisible()

  const tmdbUrl = page.getByRole('textbox', { name: 'API 地址' }).first()
  await tmdbUrl.fill('https://tmdb.example/3')
  await expect(page.locator('.unsaved-indicator')).toBeVisible()

  page.once('dialog', async (dialog) => dialog.dismiss())
  await page.getByRole('tab', { name: '账户' }).click()
  await expect(page.getByRole('tab', { name: /服务接入/ })).toHaveAttribute('aria-selected', 'true')

  page.once('dialog', async (dialog) => dialog.dismiss())
  await page.goBack()
  await expect(page).toHaveURL(/settings=providers/)
  await expect(page.locator('.unsaved-indicator')).toBeVisible()

  page.once('dialog', async (dialog) => dialog.dismiss())
  await page.locator('a.nav-item:visible').filter({ hasText: '发现' }).click()
  await expect(page).toHaveURL(/settings=providers/)

  if (testInfo.project.name === 'desktop') {
    page.once('dialog', async (dialog) => dialog.dismiss())
    await page.locator('button.profile-row:visible').click()
    await expect(page).toHaveURL(/settings=providers/)
  }

  page.once('dialog', async (dialog) => dialog.accept())
  await page.goBack()
  await expect(page).toHaveURL(/settings=overview/)

  await page.getByRole('tab', { name: '账户' }).click()
  await expect(page).toHaveURL(/settings=account/)
  await expect(page.getByRole('heading', { name: '服务与设置' })).toBeVisible()

  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'settings-account')
})

test('mobile keeps four primary destinations and usable touch targets', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile', 'mobile-only contract')
  await installApiFixtures(page)
  await page.goto('/?view=discover')

  const mobileNav = page.getByRole('navigation', { name: '移动端主导航' })
  await expect(mobileNav.getByRole('link')).toHaveCount(4)
  const navBounds = await mobileNav.getByRole('link').evaluateAll((links) => links.map((link) => {
    const rect = link.getBoundingClientRect()
    return { left: rect.left, right: rect.right, width: rect.width }
  }))
  const viewportWidth = await page.evaluate(() => window.innerWidth)
  expect(navBounds[0].left).toBeLessThanOrEqual(9)
  expect(navBounds.at(-1)!.right).toBeGreaterThanOrEqual(viewportWidth - 9)
  expect(Math.max(...navBounds.map((item) => item.width)) - Math.min(...navBounds.map((item) => item.width))).toBeLessThan(1)
  await expect(mobileNav.getByRole('link', { name: '系统设置' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: '系统设置' })).toBeVisible()

  const undersizedButtons = await page.locator('button:visible').evaluateAll((buttons) => buttons
    .map((button) => ({ label: button.getAttribute('aria-label') || button.textContent?.trim(), width: button.getBoundingClientRect().width, height: button.getBoundingClientRect().height }))
    .filter((button) => button.width < 43.5 || button.height < 43.5))
  expect(undersizedButtons).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await expectNoHorizontalOverflow(page)

  await expectNoSeriousAccessibilityViolations(page)
  await attachScreenshot(page, testInfo, 'mobile-discovery')
})

test('all workspace destinations meet the accessibility gate', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  const destinations = [
    ['transfers', '任务'],
    ['subscriptions', '订阅'],
    ['library', '媒体库'],
    ['operations', '运营工具'],
    ['settings&settings=overview', '服务与设置'],
  ] as const

  for (const [route, heading] of destinations) {
    await page.goto(`/?view=${route}`)
    await expect(page.getByRole('heading', { name: heading, level: 1 })).toBeVisible()
    await expectNoSeriousAccessibilityViolations(page)
    if (testInfo.project.name === 'mobile') {
      const undersizedButtons = await page.locator('button:visible').evaluateAll((buttons) => buttons
        .map((button) => ({ label: button.getAttribute('aria-label') || button.textContent?.trim(), width: button.getBoundingClientRect().width, height: button.getBoundingClientRect().height }))
        .filter((button) => button.width < 43.5 || button.height < 43.5))
      expect(undersizedButtons, `${heading} contains undersized controls`).toEqual([])
      await expectNoHorizontalOverflow(page)
    }
  }
})
