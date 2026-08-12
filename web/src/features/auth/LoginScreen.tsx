import { useState, type FormEvent } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Eye, EyeOff, Film, LockKeyhole, LogIn } from 'lucide-react'
import {
  ApiError,
  getAuthConfiguration,
  login,
  type LoginResponse,
} from '../../shared/api/mediaHub'

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
      <header className="auth-brand">
        <span className="auth-brand-mark"><Film size={19} strokeWidth={2.4} /></span>
        <span><strong>MEDIA HUB</strong><small>HOME MEDIA CONTROL</small></span>
      </header>

      <section className="auth-panel" aria-labelledby="login-title">
        <span className="auth-lock"><LockKeyhole size={21} /></span>
        <p className="auth-eyebrow">PRIVATE CONTROL PLANE</p>
        <h1 id="login-title">管理员登录</h1>
        <form onSubmit={handleSubmit}>
          <label htmlFor="admin-password">密码</label>
          <div className="password-field">
            <input
              autoComplete="current-password"
              autoFocus
              disabled={unavailable}
              id="admin-password"
              onChange={(event) => setPassword(event.target.value)}
              type={showPassword ? 'text' : 'password'}
              value={password}
            />
            <button
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
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
            <LogIn size={17} />
            {loginMutation.isPending ? '正在登录' : '登录'}
          </button>
        </form>
        <div aria-live="polite" className="auth-message">
          {unavailable ? '管理员账户尚未初始化。' : null}
          {configuration.isError ? '无法读取服务器认证状态。' : null}
          {errorMessage ? errorMessage : null}
        </div>
      </section>

      <footer className="auth-footer"><span>MEDIA HUB / PRIVATE</span><span>API v1</span></footer>
    </main>
  )
}
