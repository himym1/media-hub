---
name: media-hub-ui-quality
description: Audit and verify Media Hub Web and Android UI changes, accessibility, responsive layout, touch targets, URL state, and Compose semantics before release.
---

# Media Hub UI Quality

Use this skill for any Web or Android UI implementation, review, redesign, or release-readiness check in this repository.

## Required Checks

1. Run the deterministic static gate:

```bash
pnpm --dir web quality
```

2. Verify Web logic and production output:

```bash
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build
pnpm --dir web test:e2e
```

The Playwright suite must run both `desktop` and `mobile` projects. It must keep axe serious/critical violations at zero, preserve keyboard navigation, enforce scoped URL state, protect unsaved Provider edits, reject root horizontal overflow, and retain four mobile primary destinations.

3. Verify Android code and UI-test APKs:

```bash
./android/gradlew -p android :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest :app:lintDebug
```

Run instrumented Compose tests on an API 35 emulator when one is available:

```bash
./android/gradlew -p android :app:connectedDebugAndroidTest
```

4. Inspect current Web screenshots at `1440x900` and `390x844` with the visual-hierarchy rubric: entry point, eye flow, weight distribution, and emphasis. Check Android screenshots at normal and 200% font scale when an emulator is available.

## Non-Negotiable Contracts

- Web and Android have exactly four primary destinations: Discover, Tasks, Subscriptions, Library.
- Operations and service/account settings remain under a separate system entry.
- User-facing text is at least `12px` on Web and `12sp` on Android.
- Web touch controls are at least `44px`; Android interactive controls are at least `48dp`.
- Provider edits warn before navigation or unload when unsaved.
- Stable view state belongs in the URL. Never place signed transfer tokens, credentials, media paths, or provider secrets in the URL, logs, screenshots, or fixtures.
- Do not suppress axe failures or disable Android assertions to make a gate pass. Fix the underlying UI.
- UI checks never deploy or write production state.

## Evidence To Report

Report commands and pass/fail counts, Playwright viewport coverage, axe violation count, Android JVM tests versus `NO-SOURCE`, instrumented-test result, screenshot evidence, and any device/environment gap. A clean build without browser or Compose evidence is not a complete UI acceptance.
