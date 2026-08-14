import { useEffect, useState } from 'react'
import { KeyRound, Save, ServerCog } from 'lucide-react'
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

function secret(): SecretUpdate {
  return { value: '', clear: false }
}

function createDraft(settings: ProviderSettings): Draft {
  return {
    qmediaSync: { baseUrl: settings.qmediaSync.baseUrl, apiKey: secret() },
    emby: { baseUrl: settings.emby.baseUrl, apiKey: secret(), userId: settings.emby.userId },
    drive115: { clientId: settings.drive115.clientId },
    tmdb: { baseUrl: settings.tmdb.baseUrl, accessToken: secret() },
    wecom: {
      baseUrl: settings.wecom.baseUrl, corpId: settings.wecom.corpId, secret: secret(),
      sendMode: settings.wecom.sendMode || (settings.wecom.chatId ? 'appchat' : 'app'),
      agentId: settings.wecom.agentId, toUser: settings.wecom.toUser || '@all', chatId: settings.wecom.chatId,
    },
    workflow: structuredClone(settings.workflow),
    sources: settings.sources.map((source) => ({
      id: source.id, baseUrl: source.baseUrl, account: source.account,
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

  const updateSecret = (provider: 'qmediaSync' | 'emby' | 'tmdb', field: 'apiKey' | 'accessToken', value: SecretUpdate) => {
    setDraft((current) => ({ ...current, [provider]: { ...current[provider], [field]: value } } as Draft))
  }

  return (
    <form className="provider-settings" onChange={onDirty} onSubmit={(event) => { event.preventDefault(); onSave(draft) }}>
      <div className="section-heading"><div><p className="eyebrow">CONFIGURATION</p><h2>服务设置</h2></div><ServerCog size={20} /></div>

      <div className="provider-settings-grid">
        <fieldset>
          <legend>TMDB</legend>
          <label><span>API 地址</span><input onChange={(event) => setDraft((current) => ({ ...current, tmdb: { ...current.tmdb, baseUrl: event.target.value } }))} placeholder="https://api.themoviedb.org/3" type="url" value={draft.tmdb.baseUrl} /></label>
          <label><span>Read Access Token · {secretHint(settings.tmdb.accessToken.configured)}</span><input autoComplete="off" onChange={(event) => updateSecret('tmdb', 'accessToken', { ...draft.tmdb.accessToken, value: event.target.value })} type="password" value={draft.tmdb.accessToken.value} /></label>
          {settings.tmdb.accessToken.configured ? <label className="inline-check"><input checked={draft.tmdb.accessToken.clear} onChange={(event) => updateSecret('tmdb', 'accessToken', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 Token</label> : null}
        </fieldset>

        <fieldset>
          <legend>QMediaSync</legend>
          <label><span>服务地址</span><input onChange={(event) => setDraft((current) => ({ ...current, qmediaSync: { ...current.qmediaSync, baseUrl: event.target.value } }))} type="url" value={draft.qmediaSync.baseUrl} /></label>
          <label><span>API Key · {secretHint(settings.qmediaSync.apiKey.configured)}</span><input autoComplete="off" onChange={(event) => updateSecret('qmediaSync', 'apiKey', { ...draft.qmediaSync.apiKey, value: event.target.value })} type="password" value={draft.qmediaSync.apiKey.value} /></label>
          {settings.qmediaSync.apiKey.configured ? <label className="inline-check"><input checked={draft.qmediaSync.apiKey.clear} onChange={(event) => updateSecret('qmediaSync', 'apiKey', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 API Key</label> : null}
        </fieldset>

        <fieldset>
          <legend>Emby</legend>
          <label><span>服务地址</span><input onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, baseUrl: event.target.value } }))} type="url" value={draft.emby.baseUrl} /></label>
          <label><span>API Key · {secretHint(settings.emby.apiKey.configured)}</span><input autoComplete="off" onChange={(event) => updateSecret('emby', 'apiKey', { ...draft.emby.apiKey, value: event.target.value })} type="password" value={draft.emby.apiKey.value} /></label>
          <label><span>用户 ID</span><input onChange={(event) => setDraft((current) => ({ ...current, emby: { ...current.emby, userId: event.target.value } }))} value={draft.emby.userId} /></label>
          {settings.emby.apiKey.configured ? <label className="inline-check"><input checked={draft.emby.apiKey.clear} onChange={(event) => updateSecret('emby', 'apiKey', { value: '', clear: event.target.checked })} type="checkbox" />清除已保存 API Key</label> : null}
        </fieldset>

        <fieldset>
          <legend>115</legend>
          <p className="settings-note">用 115 App 在本页上方扫码授权。不需要开放平台开发者账号或 Client ID。</p>
        </fieldset>

        <fieldset>
          <legend>企业微信</legend>
          <div aria-label="企业微信发送方式" className="source-auth-mode" role="group"><button aria-pressed={draft.wecom.sendMode === 'app'} onClick={() => { onDirty(); setDraft((current) => ({ ...current, wecom: { ...current.wecom, sendMode: 'app', agentId: 0, toUser: '@all', chatId: '' } })) }} type="button">自建应用</button><button aria-pressed={draft.wecom.sendMode === 'appchat'} onClick={() => { onDirty(); setDraft((current) => ({ ...current, wecom: { ...current.wecom, sendMode: 'appchat', agentId: 0, toUser: '', chatId: '' } })) }} type="button">AppChat</button></div>
          <label><span>API 地址</span><input onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, baseUrl: event.target.value } }))} placeholder="https://qyapi.weixin.qq.com" type="url" value={draft.wecom.baseUrl} /></label>
          <label><span>Corp ID</span><input onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, corpId: event.target.value } }))} value={draft.wecom.corpId} /></label>
          <label><span>Secret · {secretHint(settings.wecom.secret.configured)}</span><input autoComplete="off" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, secret: { ...current.wecom.secret, value: event.target.value } } }))} type="password" value={draft.wecom.secret.value} /></label>
          {draft.wecom.sendMode === 'app' ? <>
            <label><span>Agent ID</span><input min="1" onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, agentId: Number(event.target.value) || 0 } }))} type="number" value={draft.wecom.agentId || ''} /></label>
            <label><span>接收人</span><input onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, toUser: event.target.value } }))} placeholder="@all" value={draft.wecom.toUser} /></label>
          </> : <label><span>Chat ID</span><input onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, chatId: event.target.value } }))} value={draft.wecom.chatId} /></label>}
          {settings.wecom.secret.configured ? <label className="inline-check"><input checked={draft.wecom.secret.clear} onChange={(event) => setDraft((current) => ({ ...current, wecom: { ...current.wecom, secret: { value: '', clear: event.target.checked } } }))} type="checkbox" />清除已保存 Secret</label> : null}
          <button className="secondary-command" disabled={!settings.wecom.secret.configured || isSaving || isTesting} onClick={onTest} type="button">{isTesting ? '正在发送' : '测试已保存配置'}</button>
          {tested ? <span className="form-success" role="status">测试通知已提交</span> : null}
          {testError ? <span className="form-error" role="alert">{testError}</span> : null}
        </fieldset>
      </div>

      <fieldset className="workflow-settings">
        <legend>工作流目标</legend>
        <label><span>QMediaSync Account ID</span><input min="0" onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, qMediaSyncAccountId: Number(event.target.value) || 0 } }))} type="number" value={draft.workflow.qMediaSyncAccountId} /></label>
        {(['movie', 'series'] as const).map((mediaType) => {
          const target = draft.workflow[mediaType]
          const label = mediaType === 'movie' ? '电影' : '剧集'
          return <div className="workflow-target" key={mediaType}><strong>{label}</strong>
            <label><span>115 目标目录 ID</span><input onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, destinationId: event.target.value } } }))} value={target.destinationId} /></label>
            <label><span>QMediaSync 目标路径</span><input onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, qMediaSyncTargetPath: event.target.value } } }))} value={target.qMediaSyncTargetPath} /></label>
            <label><span>Emby 媒体库 ID</span><input onChange={(event) => setDraft((current) => ({ ...current, workflow: { ...current.workflow, [mediaType]: { ...target, embyLibraryId: event.target.value } } }))} value={target.embyLibraryId} /></label>
          </div>
        })}
      </fieldset>

      <fieldset className="source-settings">
        <legend>原生资源源</legend>
        <p className="settings-note">蜜柑和 Sidhub 使用内置匿名适配器；帧影使用站点账号；聚影可选择网页登录或开发者 API。癫影当前仍使用合同适配器。</p>
        {settings.sources.map((source, index) => {
          const item = draft.sources[index]
          const authMode = source.id === 'juying' ? (item.authMode || 'web') : ''
          const modeChanged = source.id === 'juying' && authMode !== source.authMode
          const credential = source.id === 'framehdr'
            ? { account: '用户名', secret: '密码' }
            : source.id === 'dian'
              ? { secret: 'Bearer Token' }
              : source.id === 'juying'
                ? authMode === 'web' ? { account: '用户名', secret: '密码' } : { account: 'App ID', secret: 'API Key' }
                : source.id === 'mikan' || source.id === 'sidhub'
                  ? null
                  : { secret: 'Bearer Token' }
          const setJuyingMode = (nextMode: 'web' | 'developer') => {
            if (authMode === nextMode)
              return
            onDirty()
            setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, authMode: nextMode, account: '', token: { value: '', clear: source.token.configured } } : value) }))
          }
          return <div className="source-setting-row" key={source.id}><div className="source-setting-heading"><strong>{source.label}</strong>
            {source.id === 'juying' ? <div aria-label="聚影认证方式" className="source-auth-mode" role="group"><button aria-pressed={authMode === 'web'} onClick={() => setJuyingMode('web')} type="button">网页登录</button><button aria-pressed={authMode === 'developer'} onClick={() => setJuyingMode('developer')} type="button">开发者 API</button></div> : null}
            </div>
            <label><span>适配器地址</span><input onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, baseUrl: event.target.value } : value) }))} type="url" value={item.baseUrl} /></label>
            {credential?.account ? <label><span>{credential.account}</span><input autoComplete="off" onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, account: event.target.value } : value) }))} value={item.account} /></label> : null}
            {credential ? <label><span>{credential.secret} · {modeChanged ? '切换模式后需重新填写' : secretHint(source.token.configured)}</span><input autoComplete="new-password" onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { value: event.target.value, clear: false } } : value) }))} type="password" value={item.token.value} /></label> : null}
            {credential && source.token.configured ? <label className="inline-check"><input checked={item.token.clear} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { value: '', clear: event.target.checked } } : value) }))} type="checkbox" />清除已保存 {credential.secret}</label> : <span />}
          </div>
        })}
      </fieldset>

      <div className="settings-actions">
        {error ? <span className="form-error" role="alert">{error}</span> : null}
        {saved ? <span className="form-success" role="status">设置已加密保存并立即应用</span> : null}
        <button className="primary-action" disabled={isSaving} type="submit"><Save size={16} />{isSaving ? '正在保存' : '保存服务设置'}</button>
      </div>
      <div className="secret-policy"><KeyRound size={14} /><span>密钥不会回显；空输入保持原值，勾选清除后才会删除。</span></div>
    </form>
  )
}
