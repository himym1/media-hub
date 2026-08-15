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

  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('#main-content')).toBeFocused()

  await page.getByLabel('搜索电影或电视剧').fill('验收影片')
  await page.getByRole('button', { name: '搜索', exact: true }).click()
  await expect(page).toHaveURL(/view=discover.*q=%E9%AA%8C%E6%94%B6%E5%BD%B1%E7%89%87/)
  await expect(page.getByText('2160p', { exact: true })).toBeVisible()
  expect(page.url()).not.toContain('fixture-token')

  await page.getByRole('link', { name: '任务', exact: true }).first().click()
  await expect(page).toHaveURL(/view=transfers.*task=task-1/)
  await page.goBack()
  await expect(page.getByLabel('搜索电影或电视剧')).toHaveValue('验收影片')
  await expect(page.getByText('2160p', { exact: true })).toBeVisible()

  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
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

test('settings tabs support arrow keys and protect unsaved provider changes', async ({ page }, testInfo) => {
  await installApiFixtures(page)
  await page.goto('/?view=settings&settings=overview')

  const overviewTab = page.getByRole('tab', { name: '概览' })
  await overviewTab.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page).toHaveURL(/settings=providers/)
  await expect(page.getByRole('tab', { name: 'Provider' })).toBeFocused()

  const tmdbUrl = page.getByRole('textbox', { name: 'API 地址' }).first()
  await tmdbUrl.fill('https://tmdb.example/3')
  await expect(page.locator('.unsaved-indicator')).toBeVisible()

  page.once('dialog', async (dialog) => dialog.dismiss())
  await page.getByRole('tab', { name: '账户' }).click()
  await expect(page.getByRole('tab', { name: /Provider/ })).toHaveAttribute('aria-selected', 'true')

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
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    }
  }
})
