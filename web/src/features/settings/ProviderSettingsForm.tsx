import { useEffect, useState } from 'react'
import { KeyRound, Save, ServerCog } from 'lucide-react'
import type { ProviderSettings, ProviderSettingsUpdate, SecretUpdate } from '../../shared/api/mediaHub'

type Props = {
  settings: ProviderSettings
  isSaving: boolean
  error?: string
  saved: boolean
  onSave: (input: ProviderSettingsUpdate) => void
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
    workflow: structuredClone(settings.workflow),
    sources: settings.sources.map((source) => ({ id: source.id, baseUrl: source.baseUrl, token: secret() })),
  }
}

function secretHint(configured: boolean) {
  return configured ? '已保存，留空保持不变' : '尚未保存'
}

export function ProviderSettingsForm({ settings, isSaving, error, saved, onSave, onDirty }: Props) {
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
          <legend>115 开放平台</legend>
          <label><span>Client ID</span><input onChange={(event) => setDraft((current) => ({ ...current, drive115: { clientId: event.target.value } }))} value={draft.drive115.clientId} /></label>
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
        {settings.sources.map((source, index) => {
          const item = draft.sources[index]
          return <div className="source-setting-row" key={source.id}><strong>{source.label}</strong>
            <label><span>适配器地址</span><input onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, baseUrl: event.target.value } : value) }))} type="url" value={item.baseUrl} /></label>
            <label><span>Bearer Token · {secretHint(source.token.configured)}</span><input autoComplete="off" onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { ...value.token, value: event.target.value } } : value) }))} type="password" value={item.token.value} /></label>
            {source.token.configured ? <label className="inline-check"><input checked={item.token.clear} onChange={(event) => setDraft((current) => ({ ...current, sources: current.sources.map((value, itemIndex) => itemIndex === index ? { ...value, token: { value: '', clear: event.target.checked } } : value) }))} type="checkbox" />清除 Token</label> : <span />}
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
