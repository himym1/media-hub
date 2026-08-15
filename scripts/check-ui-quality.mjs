import { readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const failures = []

function filesUnder(path, extension) {
  const result = []
  for (const entry of readdirSync(path)) {
    const full = join(path, entry)
    if (statSync(full).isDirectory()) result.push(...filesUnder(full, extension))
    else if (full.endsWith(extension)) result.push(full)
  }
  return result
}

function source(path) {
  return readFileSync(join(root, path), 'utf8')
}

function fail(path, message) {
  failures.push(`${path}: ${message}`)
}

const cssFiles = filesUnder(join(root, 'web/src'), '.css')
const cssSources = cssFiles.map((path) => [path, readFileSync(path, 'utf8')])
for (const [path, css] of cssSources) {
  for (const match of css.matchAll(/font-size:\s*(\d+(?:\.\d+)?)px/g)) {
    if (Number(match[1]) < 12) fail(relative(root, path), `font-size ${match[1]}px is below 12px`)
  }
  for (const match of css.matchAll(/transition:\s*([^;]+)/g)) {
    if (/^(?:all\b|\.?\d)/.test(match[1].trim())) fail(relative(root, path), `transition must name explicit properties: ${match[1].trim()}`)
  }
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = match[1].trim()
    if (!/(?:\bbutton\b|\binput\b|\bselect\b|\btextarea\b|\.icon-button|\.secondary-action|\.secondary-command|\.primary-action|\.primary-button|\.danger-button|\.health-summary|\.topbar-system-link|\.system-subnav\s+a)/.test(selector)) continue
    for (const height of match[2].matchAll(/min-height:\s*(\d+(?:\.\d+)?)px/g)) {
      if (Number(height[1]) < 44) fail(relative(root, path), `${selector} has a ${height[1]}px interactive minimum height`)
    }
  }
}

const allCss = cssSources.map(([, css]) => css).join('\n')
const definedVariables = new Set([...allCss.matchAll(/(--[a-z0-9-]+)\s*:/gi)].map((match) => match[1]))
const usedVariables = new Set([...allCss.matchAll(/var\((--[a-z0-9-]+)/gi)].map((match) => match[1]))
for (const variable of usedVariables) {
  if (!definedVariables.has(variable)) fail('web/src', `undefined CSS custom property ${variable}`)
}

const kotlinFiles = filesUnder(join(root, 'android/app/src/main/java'), '.kt')
for (const path of kotlinFiles) {
  const kotlin = readFileSync(path, 'utf8')
  for (const match of kotlin.matchAll(/fontSize\s*=\s*(\d+(?:\.\d+)?)\.sp/g)) {
    if (Number(match[1]) < 12) fail(relative(root, path), `fontSize ${match[1]}sp is below 12sp`)
  }
}

const workspace = source('web/src/features/search/SearchWorkspace.tsx')
if (!workspace.includes('className="skip-link"') || !workspace.includes('id="main-content"')) fail('web/src/features/search/SearchWorkspace.tsx', 'skip link and main target are required')
const workspaceCss = source('web/src/features/search/SearchWorkspace.css')
if (!/\.mobile-nav\s*\{[^}]*grid-template-columns:\s*repeat\(4,/s.test(workspaceCss)) fail('web/src/features/search/SearchWorkspace.css', 'mobile navigation must use four equal columns')

const login = source('web/src/features/auth/LoginScreen.tsx')
if (login.includes('autoFocus')) fail('web/src/features/auth/LoginScreen.tsx', 'mobile login must not autofocus')

const providerForm = source('web/src/features/settings/ProviderSettingsForm.tsx')
for (const input of providerForm.matchAll(/<input\b[\s\S]*?\/>/g)) {
  if (!/\bname=/.test(input[0])) fail('web/src/features/settings/ProviderSettingsForm.tsx', `provider input is missing name: ${input[0].slice(0, 80)}`)
}

const components = source('android/app/src/main/java/com/mediahub/android/core/designsystem/MediaHubComponents.kt')
if ((components.match(/heightIn\(min = 48\.dp\)/g) ?? []).length < 3) fail('android/app/src/main/java/com/mediahub/android/core/designsystem/MediaHubComponents.kt', 'buttons, segmented controls, and text fields require 48dp minimum height')

if (failures.length > 0) {
  console.error(`UI quality check failed (${failures.length})`)
  for (const failure of failures) console.error(`- ${failure}`)
  process.exit(1)
}

console.log(`UI quality check passed: ${cssFiles.length} CSS files and ${kotlinFiles.length} Kotlin files scanned`)
