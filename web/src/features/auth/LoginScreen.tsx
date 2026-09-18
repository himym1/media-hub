import { useState, type FormEvent } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Eye, EyeOff, Film, KeyRound, LockKeyhole, LogIn, ShieldCheck } from 'lucide-react'
import {
  ApiError,
  getAuthConfiguration,
  login,
  type LoginResponse,
} from '../../shared/api/mediaHub'
import { isDesktopShell } from '../../shared/desktop/desktopShell'

type LoginScreenProps = {
  onAuthenticated: (response: LoginResponse) => void
  serviceError: string | null
}

export function LoginScreen({ onAuthenticated, serviceError }: LoginScreenProps) {
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const configuration = useQuery({
    queryKey: ['auth', 'configuration'],
    queryFn: getAuthConfiguration,
    retry: false,
  })
  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: onAuthenticated,
  })

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!password || loginMutation.isPending || configuration.data?.configured !== true) return
    loginMutation.mutate(password)
  }

  const errorMessage = loginMutation.error instanceof ApiError
    ? loginMutation.error.message
    : serviceError
  const unavailable = configuration.isSuccess && !configuration.data.configured

  return (
    <main className="auth-shell">
      {isDesktopShell() ? <div aria-hidden="true" className="desktop-drag-region" data-tauri-drag-region /> : null}
      <div aria-hidden="true" className="auth-ambient-glow auth-ambient-1" />
      <div aria-hidden="true" className="auth-ambient-glow auth-ambient-2" />
      <div aria-hidden="true" className="auth-ambient-grid" />

      <div className="auth-center-wrap">
        <header className="auth-hero-brand">
          <div className="auth-hero-mark">
            <Film size={30} strokeWidth={2.2} />
          </div>
          <h1 className="auth-hero-title">Media Hub</h1>
          <p className="auth-hero-subtitle">私有流媒体与自动化控制中心</p>
        </header>

        <section aria-labelledby="login-title" className="auth-panel">
          <div className="auth-panel-header">
            <span className="auth-lock-chip">
              <LockKeyhole size={14} />
              管理员认证
            </span>
            <span className="auth-status-dot" title="系统服务在线" />
          </div>

          <form onSubmit={handleSubmit}>
            <label htmlFor="admin-password">访问密码</label>
            <div className="password-field">
              <span aria-hidden="true" className="password-icon">
                <KeyRound size={16} />
              </span>
              <input
                autoComplete="current-password"
                autoFocus
                disabled={unavailable}
                id="admin-password"
                name="admin-password"
                onChange={(event) => setPassword(event.target.value)}
                placeholder="请输入管理员密码…"
                type={showPassword ? 'text' : 'password'}
                value={password}
              />
              <button
                aria-label={showPassword ? '隐藏密码' : '显示密码'}
                className="password-toggle-btn"
                onClick={() => setShowPassword((visible) => !visible)}
                type="button"
              >
                {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>

            <button
              className="login-button"
              disabled={!password || loginMutation.isPending || unavailable}
              type="submit"
            >
              <LogIn size={16} />
              {loginMutation.isPending ? '正在验证…' : '进入控制台'}
            </button>
          </form>

          {errorMessage || unavailable || configuration.isError ? (
            <div aria-live="polite" className="auth-message-box" role="alert">
              {unavailable ? '管理员账户尚未初始化。' : null}
              {configuration.isError ? '无法读取服务器认证状态，请检查网络连接。' : null}
              {errorMessage ? errorMessage : null}
            </div>
          ) : null}

          <div className="auth-features-row">
            <span><ShieldCheck size={13} /> 115 直连播放</span>
            <span>·</span>
            <span>Emby 媒体同步</span>
            <span>·</span>
            <span>本地加密存储</span>
          </div>
        </section>

        <footer className="auth-footer">
          <span>Media Hub Control Plane</span>
          <span>v0.20.69 · API v1</span>
        </footer>
      </div>
    </main>
  )
}
