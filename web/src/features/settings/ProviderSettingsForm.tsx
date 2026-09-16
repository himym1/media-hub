import { useEffect, useState } from 'react'
import { CalendarCheck, Film, KeyRound, MessageSquare, Save, Search, ServerCog, Waypoints } from 'lucide-react'
import type { ProviderSettings, ProviderSettingsUpdate, SecretUpdate } from '../../shared/api/mediaHub'

type Props = {
  settings: ProviderSettings
  isSaving: boolean
  isTesting: boolean
  error?: string
  testError?: string
  saved: boolean
  tested: boolean
  onSave: (input: ProviderSettingsUpdate) => void
  onTest: () => void
  onDirty: () => void
}

type Draft = ProviderSettingsUpdate

const machineFieldProps = {
  autoCapitalize: 'none' as const,
  autoComplete: 'off',
  spellCheck: false,
}

function secret(): SecretUpdate {
  return { value: '', clear: false }
}

const defaultCheckIn = { enabled: true, hour: 0, minute: 5, sources: ['framehdr', 'juying'] as Array<'framehdr' | 'juying'> }

function checkInTimeValue(hour: number, minute: number) {
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`
}

function createDraft(settings: ProviderSettings): Draft {
  const wecomSendMode = settings.wecom.sendMode || (settings.wecom.chatId ? 'appchat' : 'app')
  return {
    qmediaSync: { baseUrl: settings.qmediaSync.baseUrl, apiKey: secret() },
    emby: { baseUrl: settings.emby.baseUrl, apiKey: secret(), userId: settings.emby.userId, password: secret() },
    sharedEmby: {
      baseUrl: settings.sharedEmby?.baseUrl ?? '',
      username: settings.sharedEmby?.username ?? '',
      password: secret(),
      proxyUrl: settings.sharedEmby?.proxyUrl ?? '',
    },
    drive115: { clientId: settings.drive115.clientId },
    tmdb: { baseUrl: settings.tmdb.baseUrl, accessToken: secret() },
    assrt: { baseUrl: settings.assrt?.baseUrl ?? '', token: secret() },
    moviePilot: { baseUrl: settings.moviePilot?.baseUrl ?? '', apiToken: secret() },
    wecom: {
      baseUrl: settings.wecom.baseUrl,
      corpId: settings.wecom.corpId,
      secret: secret(),
      sendMode: wecomSendMode,
      agentId: settings.wecom.agentId,
      toUser: wecomSendMode === 'appchat' ? settings.wecom.toUser : (settings.wecom.toUser || '@all'),
      chatId: settings.wecom.chatId,
    },
    workflow: {
      ...structuredClone(settings.workflow),
      syncMode: 'builtin',
      strmBaseUrl: settings.workflow.strmBaseUrl || '',
      strmRootMount: settings.workflow.strmRootMount || '',
    },
    checkIn: settings.checkIn ?? defaultCheckIn,
    sources: settings.sources.map((source) => ({
      id: source.id,
      baseUrl: source.baseUrl,
      account: source.account,
      authMode: source.id === 'juying' ? (source.authMode || (source.account || source.token.configured ? 'developer' : 'web')) : '',
      token: secret(),
    })),
  }
}

function secretHint(configured: boolean) {
  return configured ? '已保存，留空保持不变' : '尚未保存'
}

export function ProviderSettingsForm({ settings, isSaving, isTesting, error, testError, saved, tested, onSave, onTest, onDirty }: Props) {
  const [draft, setDraft] = useState<Draft>(() => createDraft(settings))
  useEffect(() => setDraft(createDraft(settings)), [settings])

  const updateSecret = (provider: 'emby' | 'tmdb' | 'assrt' | 'moviePilot', field: 'apiKey' | 'accessToken' | 'token' | 'apiToken', value: SecretUpdate) => {
    setDraft((current) => ({ ...current, [provider]: { ...current[provider], [field]: value } } as Draft))
  }

  return (
    <form className="provider-settings" onChange={onDirty} onSubmit={(event) => { event.preventDefault(); onSave(draft) }}>
      <div className="section-heading"><div><h2>服务接入设置</h2></div><ServerCog size={20} /></div>

      <section className="settings-group" aria-labelledby="core-services-heading">
        <div className="settings-group-header">
          <h3 id="core-services-heading"><Film size={17} />核心媒体服务</h3>
          <p>配置影视信息刮削、网盘存储与媒体服务器。内置 STRM 使用现有 115 扫码会话写入播放地址。</p>
        </div>
        <div className="provider-settings-grid">
          <fieldset>
            <legend>TMDB</legend>
            <label><span>API 地址</span><input {...machineFieldProps} name="tmdb-base-url" onChange={(event) => setDraft((current) => ({ ...current, tmdb: { ...current.tmdb, baseUrl: event.target.value } }))} placeholder="https://api.themoviedb.org/3" type="url" value={draft.tmdb.baseUrl} /></label>
            <label><span>Read Access Token · {secretHint(settings.tmdb.accessToken.configured)}</span><input {...machineFieldProps} autoComplete="new-password" name="tmdb-access-token" onChange={(event) => updateSecret('tmdb', 'accessToken', { ...draft.tmdb.accessToken, value: event.target.value })} type="password" value={draft.tmdb.accessToken.value} /></label>
            {settings.tmdb.accessToken.configured ? <label className="inline-check"><input checked={draft.tmdb.accessToken.clear} name="tmdb-clear-token" onChange={(event) => updateSecret('tmdb', 'accessToken', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 Token</label> : null}
          </fieldset>

          <fieldset>
            <legend>Assrt</legend>
            <label><span>API 地址</span><input {...machineFieldProps} name="assrt-base-url" onChange={(event) => setDraft((current) => ({ ...current, assrt: { ...current.assrt, baseUrl: event.target.value } }))} placeholder="https://api.assrt.net" type="url" value={draft.assrt.baseUrl} /></label>
            <label><span>Token · {secretHint(settings.assrt?.token.configured ?? false)}</span><input {...machineFieldProps} autoComplete="new-password" name="assrt-token" onChange={(event) => updateSecret('assrt', 'token', { ...draft.assrt.token, value: event.target.value })} type="password" value={draft.assrt.token.value} /></label>
            {settings.assrt?.token.configured ? <label className="inline-check"><input checked={draft.assrt.token.clear} name="assrt-clear-token" onChange={(event) => updateSecret('assrt', 'token', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 Token</label> : null}
            <p className="settings-note">在 assrt.net 用户面板申请 32 位 Token。中文搜索优先走 Assrt，字幕写到 STRM 旁，不经 Media Hub 转发视频。</p>
            <p className="settings-note">字幕服务由 assrt.net 提供</p>
          </fieldset>

          <fieldset>
            <legend>MoviePilot</legend>
            <label><span>服务地址</span><input {...machineFieldProps} name="moviepilot-base-url" onChange={(event) => setDraft((current) => ({ ...current, moviePilot: { ...current.moviePilot, baseUrl: event.target.value } }))} placeholder="http://172.17.0.1:13001" type="url" value={draft.moviePilot.baseUrl} /></label>
            <label><span>API Token · {secretHint(settings.moviePilot?.apiToken.configured ?? false)}</span><input {...machineFieldProps} autoComplete="new-password" name="moviepilot-api-token" onChange={(event) => updateSecret('moviePilot', 'apiToken', { ...draft.moviePilot.apiToken, value: event.target.value })} type="password" value={draft.moviePilot.apiToken.value} /></label>
            {settings.moviePilot?.apiToken.configured ? <label className="inline-check"><input checked={draft.moviePilot.apiToken.clear} name="moviepilot-clear-token" onChange={(event) => updateSecret('moviePilot', 'apiToken', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 Token</label> : null}
            <p className="settings-note">搜索会列出 PT 结果。Hub 只把下载交给 MoviePilot，不保存 PT 站点 Cookie，也不能把种子转存到 115。</p>
          </fieldset>

          <fieldset>
            <legend>Emby</legend>
            <label><span>服务地址</span><input {...machineFieldProps} name="emby-base-url" onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, baseUrl: event.target.value } }))} type="url" value={draft.emby.baseUrl} /></label>
            <label><span>API Key · {secretHint(settings.emby.apiKey.configured)}</span><input {...machineFieldProps} autoComplete="new-password" name="emby-api-key" onChange={(event) => updateSecret('emby', 'apiKey', { ...draft.emby.apiKey, value: event.target.value })} type="password" value={draft.emby.apiKey.value} /></label>
            <label><span>用户 ID</span><input {...machineFieldProps} name="emby-user-id" onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, userId: event.target.value } }))} value={draft.emby.userId} /></label>
            <label><span>用户密码 · {secretHint(settings.emby.password.configured)}</span><input {...machineFieldProps} autoComplete="new-password" name="emby-password" onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, password: { ...current.emby.password, value: event.target.value } } }))} type="password" value={draft.emby.password.value} /></label>
            <p className="settings-note">Media Hub 从 NAS 访问 Emby，请填局域网地址。公网域名会在容器里回环，容易变成 502。</p>
            <p className="settings-note">从 Emby 删除媒体需要该用户的登录密码；仅 API Key 无法删除。</p>
            {settings.emby.apiKey.configured ? <label className="inline-check"><input checked={draft.emby.apiKey.clear} name="emby-clear-api-key" onChange={(event) => updateSecret('emby', 'apiKey', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 API Key</label> : null}
            {settings.emby.password.configured ? <label className="inline-check"><input checked={draft.emby.password.clear} name="emby-clear-password" onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, password: { value: '', clear: event.target.checked } } }))} type="checkbox" />清除已保存用户密码</label> : null}
          </fieldset>

          <fieldset>
            <legend>共享 Emby</legend>
            <label><span>服务地址</span><input {...machineFieldProps} name="shared-emby-base-url" onChange={(event) => setDraft((current) => ({ ...current, sharedEmby: { ...current.sharedEmby, baseUrl: event.target.value } }))} placeholder="https://emby.example.com" type="url" value={draft.sharedEmby.baseUrl} /></label>
            <label><span>用户名</span><input {...machineFieldProps} name="shared-emby-username" onChange={(event) => setDraft((current) => ({ ...current, sharedEmby: { ...current.sharedEmby, username: event.target.value } }))} value={draft.sharedEmby.username} /></label>
            <label><span>密码 · {secretHint(settings.sharedEmby?.password.configured ?? false)}</span><input {...machineFieldProps} autoComplete="new-password" name="shared-emby-password" onChange={(event) => setDraft((current) => ({ ...current, sharedEmby: { ...current.sharedEmby, password: { ...current.sharedEmby.password, value: event.target.value } } }))} type="password" value={draft.sharedEmby.password.value} /></label>
            <label><span>代理地址（可选）</span><input {...machineFieldProps} name="shared-emby-proxy-url" onChange={(event) => setDraft((current) => ({ ...current, sharedEmby: { ...current.sharedEmby, proxyUrl: event.target.value } }))} placeholder="http://127.0.0.1:7890" type="url" value={draft.sharedEmby.proxyUrl} /></label>
            <p className="settings-note">只读第二路目录，不替换 NAS 上的 Emby。播放由播放器直连远程 Emby，不经 Media Hub 转发视频。</p>
            {settings.sharedEmby?.password.configured ? <label className="inline-check"><input checked={draft.sharedEmby.password.clear} name="shared-emby-clear-password" onChange={(event) => setDraft((current) => ({ ...current, sharedEmby: { ...current.sharedEmby, password: { value: '', clear: event.target.checked } } }))} type="checkbox" />清除已保存密码</label> : null}
          </fieldset>

          <fieldset>
            <legend>115</legend>
            <p className="settings-note">用 115 App 在「概览」页扫码授权。不需要开放平台开发者账号或 Client ID。</p>
          </fieldset>
        </div>
      </section>

      <section className="settings-group" aria-labelledby="workflow-settings-heading">
        <div className="settings-group-header">
          <h3 id="workflow-settings-heading"><Waypoints size={17} />工作流目录映射</h3>
          <p>指定转存落盘的 115 目录 ID、STRM 写入路径与 Emby 库 ID。内置模式把 <code>/115/url/</code> 写到 Media Hub 同源地址，不经 Media Hub 转发视频字节。</p>
        </div>
        <fieldset className="workflow-settings">
          <div>
            <label><span>STRM 基址</span><input {...machineFieldProps} name="strm-base-url" onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, strmBaseUrl: event.target.value } }))} placeholder="https://media.himym.us.ci" type="url" value={draft.workflow.strmBaseUrl} /></label>
            <label><span>STRM 根挂载</span><input {...machineFieldProps} name="strm-root-mount" onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, strmRootMount: event.target.value } }))} placeholder="/media" value={draft.workflow.strmRootMount} /></label>
          </div>
          {(['movie', 'series'] as const).map((mediaType) => {
            const target = draft.workflow[mediaType]
            const label = mediaType === 'movie' ? '电影' : '剧集'
            return <div className="workflow-target" key={mediaType}><strong>{label}</strong>
              <label><span>115 目标目录 ID</span><input {...machineFieldProps} name={`${mediaType}-destination-id`} onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, destinationId: event.target.value } } }))} value={target.destinationId} /></label>
              <label><span>STRM 目标路径</span><input {...machineFieldProps} name={`${mediaType}-strm-target-path`} onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, qMediaSyncTargetPath: event.target.value } } }))} value={target.qMediaSyncTargetPath} /></label>
              <label><span>Emby 媒体库 ID</span><input {...machineFieldProps} name={`${mediaType}-emby-library-id`} onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, embyLibraryId: event.target.value } } }))} value={target.embyLibraryId} /></label>
            </div>
          })}
        </fieldset>
      </section>

      <section className="settings-group" aria-labelledby="notification-settings-heading">
        <div className="settings-group-header">
          <h3 id="notification-settings-heading"><MessageSquare size={17} />消息通知</h3>
          <p>转存完成或需人工核对时推送通知卡片。</p>
        </div>
        <fieldset className="notification-card">
          <legend>企业微信</legend>
          <div aria-label="企业微信发送方式" className="source-auth-mode" role="group">
            <button aria-pressed={draft.wecom.sendMode === 'app'} onClick={() => { onDirty(); setDraft((current) => ({ ...current, wecom: { ...current.wecom, sendMode: 'app', agentId: 0, toUser: '@all', chatId: '' } })) }} type="button">自建应用</button>
            <button aria-pressed={draft.wecom.sendMode === 'appchat'} onClick={() => { onDirty(); setDraft((current) => ({ ...current, wecom: { ...current.wecom, sendMode: 'appchat', agentId: 0, toUser: '', chatId: '' } })) }} type="button">AppChat</button>
          </div>
          <label><span>API 地址</span><input {...machineFieldProps} name="wecom-base-url" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, baseUrl: event.target.value } }))} placeholder="https://qyapi.weixin.qq.com" type="url" value={draft.wecom.baseUrl} /></label>
          <label><span>Corp ID</span><input {...machineFieldProps} name="wecom-corp-id" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, corpId: event.target.value } }))} value={draft.wecom.corpId} /></label>
          <label><span>Secret · {secretHint(settings.wecom.secret.configured)}</span><input {...machineFieldProps} autoComplete="new-password" name="wecom-secret" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, secret: { ...current.wecom.secret, value: event.target.value } } }))} type="password" value={draft.wecom.secret.value} /></label>
          {draft.wecom.sendMode === 'app' ? <>
            <label><span>Agent ID</span><input {...machineFieldProps} min="1" name="wecom-agent-id" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, agentId: Number(event.target.value) || 0 } }))} type="number" value={draft.wecom.agentId || ''} /></label>
            <label><span>接收人</span><input {...machineFieldProps} name="wecom-recipient" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, toUser: event.target.value } }))} placeholder="@all" value={draft.wecom.toUser} /></label>
          </> : <label><span>Chat ID</span><input {...machineFieldProps} name="wecom-chat-id" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, chatId: event.target.value } }))} value={draft.wecom.chatId} /></label>}
          {settings.wecom.secret.configured ? <label className="inline-check"><input checked={draft.wecom.secret.clear} name="wecom-clear-secret" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, secret: { value: '', clear: event.target.checked } } }))} type="checkbox" />清除已保存 Secret</label> : null}
          <div className="notification-actions">
            <button className="secondary-command" disabled={!settings.wecom.secret.configured || isSaving || isTesting} onClick={onTest} type="button">{isTesting ? '正在发送' : '测试通知发送'}</button>
            {tested ? <span className="form-success" role="status">测试通知已提交</span> : null}
            {testError ? <span className="form-error" role="alert">{testError}</span> : null}
          </div>
        </fieldset>
      </section>

      <section className="settings-group" aria-labelledby="sources-settings-heading">
        <div className="settings-group-header">
          <h3 id="sources-settings-heading"><Search size={17} />资源搜索源</h3>
          <p>全网检索影视资源的适配器。蜜柑和 Sidhub 使用内置匿名适配器。盘搜只收 115 分享，TG 频道配在盘搜服务里。</p>
        </div>
        <fieldset className="source-settings">
          {settings.sources.map((source, index) => {
            const item = draft.sources[index]
            const authMode = source.id === 'juying' ? (item.authMode || 'web') : ''
            const modeChanged = source.id === 'juying' && authMode !== source.authMode
            const credential = source.id === 'framehdr'
              ? { account: '用户名', secret: '密码' }
              : source.id === 'dian'
                ? { secret: 'Token' }
                : source.id === 'juying'
                  ? authMode === 'web' ? { account: '用户名', secret: '密码' } : { account: 'App ID', secret: 'API Key' }
                  : source.id === 'mikan' || source.id === 'sidhub'
                    ? null
                    : { secret: source.id === 'pansou' ? 'Bearer Token（可选）' : 'Token' }
            const setJuyingMode = (nextMode: 'web' | 'developer') => {
              if (authMode === nextMode) return
              onDirty()
              setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, authMode: nextMode, account: '', token: { value: '', clear: source.token.configured } } : value) }))
            }
            return <div className="source-setting-row" key={source.id}>
              <div className="source-setting-heading"><strong>{source.label}</strong>
                {source.id === 'juying' ? <div aria-label="聚影认证方式" className="source-auth-mode" role="group"><button aria-pressed={authMode === 'web'} onClick={() => setJuyingMode('web')} type="button">网页登录</button><button aria-pressed={authMode === 'developer'} onClick={() => setJuyingMode('developer')} type="button">开发者 API</button></div> : null}
              </div>
              <label><span>适配器地址</span><input {...machineFieldProps} name={`${source.id}-base-url`} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, baseUrl: event.target.value } : value) }))} placeholder={source.id === 'pansou' ? 'http://172.17.0.1:57081' : undefined} type="url" value={item.baseUrl} /></label>
              {credential?.account ? <label><span>{credential.account}</span><input {...machineFieldProps} name={`${source.id}-account`} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, account: event.target.value } : value) }))} value={item.account} /></label> : null}
              {credential ? <label><span>{credential.secret} · {modeChanged ? '切换模式后需重新填写' : secretHint(source.token.configured)}</span><input {...machineFieldProps} autoComplete="new-password" name={`${source.id}-secret`} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { value: event.target.value, clear: false } } : value) }))} type="password" value={item.token.value} /></label> : null}
              {credential && source.token.configured ? <label className="inline-check"><input checked={item.token.clear} name={`${source.id}-clear-secret`} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { value: '', clear: event.target.checked } } : value) }))} type="checkbox" />清除已保存 {credential.secret}</label> : <span />}
            </div>
          })}
        </fieldset>
      </section>

      <section className="settings-group" aria-labelledby="checkin-settings-heading">
        <div className="settings-group-header">
          <h3 id="checkin-settings-heading"><CalendarCheck size={17} />每日签到</h3>
          <p>按北京时间每天签一次。关闭自动签到后仍可在概览手动签到。</p>
        </div>
        <fieldset>
          <legend>签到计划</legend>
          <label className="inline-check">
            <input checked={draft.checkIn.enabled} name="checkin-enabled" onChange={(event) => setDraft((current) => ({ ...current, checkIn: { ...current.checkIn, enabled: event.target.checked } }))} type="checkbox" />
            自动签到
          </label>
          <label>
            <span>签到时间（北京时间）</span>
            <input
              disabled={!draft.checkIn.enabled}
              name="checkin-time"
              onChange={(event) => {
                const [hourText = '0', minuteText = '0'] = event.target.value.split(':')
                setDraft((current) => ({ ...current, checkIn: { ...current.checkIn, hour: Number(hourText) || 0, minute: Number(minuteText) || 0 } }))
              }}
              type="time"
              value={checkInTimeValue(draft.checkIn.hour, draft.checkIn.minute)}
            />
          </label>
          <div className="checkin-source-toggles" role="group" aria-label="自动签到来源">
            {([
              { id: 'framehdr', label: '帧影' },
              { id: 'juying', label: '聚影' },
            ] as const).map((source) => (
              <label className="inline-check" key={source.id}>
                <input
                  checked={draft.checkIn.sources.includes(source.id)}
                  disabled={!draft.checkIn.enabled}
                  name={`checkin-source-${source.id}`}
                  onChange={(event) => setDraft((current) => ({
                    ...current,
                    checkIn: {
                      ...current.checkIn,
                      sources: event.target.checked
                        ? [...current.checkIn.sources.filter((id) => id !== source.id), source.id]
                        : current.checkIn.sources.filter((id) => id !== source.id),
                    },
                  }))}
                  type="checkbox"
                />
                {source.label}
              </label>
            ))}
          </div>
        </fieldset>
      </section>

      <div className="settings-actions">
        {error ? <span className="form-error" role="alert">{error}</span> : null}
        {saved ? <span className="form-success" role="status">设置已加密保存并立即应用</span> : null}
        <button className="primary-action" disabled={isSaving} type="submit"><Save size={16} />{isSaving ? '正在保存' : '保存服务设置'}</button>
      </div>
      <div className="secret-policy"><KeyRound size={14} /><span>密钥不会回显；空输入保持原值，勾选清除后才会删除。</span></div>
    </form>
  )
}
