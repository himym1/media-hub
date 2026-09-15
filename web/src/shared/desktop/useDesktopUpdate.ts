import { useCallback, useEffect, useState } from 'react'
import { ApiError, getLatestDesktopRelease, type DesktopRelease } from '../api/mediaHub'
import {
  desktopAppPlatform,
  desktopAppVersion,
  desktopNativeInstallReady,
  desktopUpdateErrorMessage,
  downloadDesktopInstaller,
  installDesktopUpdate,
  isDesktopShell,
  isDesktopUpdateCancelled,
  rememberDismissedDesktopRelease,
  resolveDesktopUpdate,
} from './desktopUpdate'

export type DesktopUpdateState = {
  available: boolean
  currentVersion: string
  nativeInstall: boolean
  pendingRelaunch: boolean
  release: DesktopRelease | null
  prompt: DesktopRelease | null
  required: boolean
  installing: boolean
  checking: boolean
  error: string | null
  check: () => Promise<void>
  install: () => Promise<void>
  dismiss: () => void
}

export function useDesktopUpdate(): DesktopUpdateState {
  const available = isDesktopShell()
  const [currentVersion, setCurrentVersion] = useState('')
  const [nativeInstall, setNativeInstall] = useState(false)
  const [release, setRelease] = useState<DesktopRelease | null>(null)
  const [pendingRelaunch, setPendingRelaunch] = useState(false)
  const [required, setRequired] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [dismissedCode, setDismissedCode] = useState<number | null>(null)

  const check = useCallback(async () => {
    if (!available) return
    setChecking(true)
    setError(null)
    try {
      const platform = await desktopAppPlatform()
      if (!platform) {
        setRelease(null)
        setPendingRelaunch(false)
        setRequired(false)
        return
      }
      const version = await desktopAppVersion()
      setCurrentVersion(version ?? '')
      setNativeInstall(desktopNativeInstallReady(version, platform))
      const latest = await getLatestDesktopRelease(platform)
      const decision = resolveDesktopUpdate(version, latest)
      setRequired(decision.required)
      setPendingRelaunch(decision.pendingRelaunch)
      setRelease(decision.release)
    } catch (cause) {
      setRelease(null)
      setPendingRelaunch(false)
      setRequired(false)
      if (cause instanceof ApiError && (cause.status === 404 || cause.code === 'desktop_release_unavailable')) {
        setError(null)
      } else {
        setError(desktopUpdateErrorMessage(cause))
      }
    } finally {
      setChecking(false)
    }
  }, [available])

  useEffect(() => {
    if (!available) return
    void check()
  }, [available, check])

  const install = useCallback(async () => {
    if (!release || installing) return
    setInstalling(true)
    setError(null)
    try {
      if (nativeInstall) {
        await installDesktopUpdate(release)
      } else {
        await downloadDesktopInstaller(release)
        await check()
      }
    } catch (cause) {
      if (!isDesktopUpdateCancelled(cause)) {
        setError(desktopUpdateErrorMessage(cause))
      }
    } finally {
      setInstalling(false)
    }
  }, [check, installing, nativeInstall, release])

  const dismiss = useCallback(() => {
    if (!release || required || installing) return
    rememberDismissedDesktopRelease(release.versionCode)
    setDismissedCode(release.versionCode)
    if (pendingRelaunch) {
      setRelease(null)
      setPendingRelaunch(false)
    }
  }, [installing, pendingRelaunch, release, required])

  return {
    available,
    currentVersion,
    nativeInstall,
    pendingRelaunch,
    release,
    prompt: release && (required || release.versionCode !== dismissedCode) ? release : null,
    required,
    installing,
    checking,
    error,
    check,
    install,
    dismiss,
  }
}
