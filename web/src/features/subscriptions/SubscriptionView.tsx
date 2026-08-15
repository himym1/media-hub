import { useEffect, useRef, useState, type ChangeEvent, type Dispatch, type FormEvent, type SetStateAction } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, Clock3, Download, Pause, Play, Plus, RefreshCw, Save, Trash2, Upload } from 'lucide-react'
import {
  createSubscription,
  deleteSubscription,
  exportSubscriptions,
  importSubscriptions,
  listSubscriptionRuns,
  listSubscriptions,
  runSubscription,
  setSubscriptionEnabled,
  setSubscriptionsEnabled,
  updateSubscription,
  type Candidate,
  type Subscription,
  type SubscriptionInput,
  type SubscriptionPreferences,
  type SubscriptionRunState,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'

const emptyPreferences: SubscriptionPreferences = {
  resolutions: [],
  videoCodecs: [],
  dynamicRanges: [],
  audioContains: [],
  preferredSources: [],
  minSizeBytes: 0,
  maxSizeBytes: 0,
  allowUnknownSize: false,
  preferSmaller: false,
}

const runLabels: Record<SubscriptionRunState, string> = {
  queued: '等待执行',
  searching: '正在搜索',
  retry_wait: '等待重试',
  no_match: '没有匹配',
  duplicate: '已去重',
  enqueued: '已加入任务',
  completed: '已完成',
  failed: '失败',
  needs_attention: '需要确认',
}

type EditorState = {
  tmdbId: string
  title: string
  originalTitle: string
  year: string
  mediaType: 'movie' | 'series'
  season: string
  policy: 'once' | 'upgrade'
  enabled: boolean
  intervalMinutes: string
  sourceIds: string
  resolutions: string
  videoCodecs: string
  dynamicRanges: string
  audioContains: string
  preferredSources: string
  minSizeGiB: string
  maxSizeGiB: string
  allowUnknownSize: boolean
  preferSmaller: boolean
}

type SubscriptionViewProps = {
  draftCandidate: Candidate | null
  onDraftConsumed: () => void
}

export function SubscriptionView({ draftCandidate, onDraftConsumed }: SubscriptionViewProps) {
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [editor, setEditor] = useState<EditorState>(() => emptyEditor())
  const importInput = useRef<HTMLInputElement>(null)
  const subscriptions = useQuery({
    queryKey: ['subscriptions'],
    queryFn: listSubscriptions,
    refetchInterval: 15_000,
  })
  const selected = subscriptions.data?.subscriptions.find((item) => item.id === selectedId) ?? null
  const runs = useQuery({
    queryKey: ['subscription-runs', selectedId],
    queryFn: () => listSubscriptionRuns(selectedId ?? '', 50),
    enabled: Boolean(selectedId),
    refetchInterval: selectedId ? 10_000 : false,
  })

  useEffect(() => {
    if (!draftCandidate?.tmdbId) return
    setSelectedId(null)
    setEditor(editorFromCandidate(draftCandidate))
    onDraftConsumed()
  }, [draftCandidate, onDraftConsumed])

  useEffect(() => {
    if (selected) setEditor(editorFromSubscription(selected))
  }, [selected])

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] }),
      queryClient.invalidateQueries({ queryKey: ['subscription-runs'] }),
    ])
  }
  const save = useMutation({
    mutationFn: async (input: SubscriptionInput) => selected
      ? updateSubscription(selected.id, {
          title: input.title,
          originalTitle: input.originalTitle,
          year: input.year,
          policy: input.policy,
          enabled: input.enabled,
          intervalMinutes: input.intervalMinutes,
          sourceIds: input.sourceIds,
          preferences: input.preferences,
        })
      : createSubscription(input),
    onSuccess: async (item) => {
      setSelectedId(item.id)
      await refresh()
    },
  })
  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => setSubscriptionEnabled(id, enabled),
    onSuccess: refresh,
  })
  const runNow = useMutation({ mutationFn: runSubscription, onSuccess: refresh })
  const remove = useMutation({
    mutationFn: deleteSubscription,
    onSuccess: async () => {
      setSelectedId(null)
      setEditor(emptyEditor())
      await refresh()
    },
  })
  const batch = useMutation({
    mutationFn: (enabled: boolean) => setSubscriptionsEnabled(subscriptions.data?.subscriptions.map((item) => item.id) ?? [], enabled),
    onSuccess: refresh,
  })
  const importBackup = useMutation({
    mutationFn: async (file: File) => {
      if (file.size > 2 * 1024 * 1024) throw new Error('订阅备份超过 2 MiB')
      return importSubscriptions(JSON.parse(await file.text()))
    },
    onSuccess: refresh,
  })

  const downloadBackup = async () => {
    const backup = await exportSubscriptions()
    const url = URL.createObjectURL(new Blob([JSON.stringify(backup, null, 2)], { type: 'application/json' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `media-hub-subscriptions-${new Date().toISOString().slice(0, 10)}.json`
    anchor.click()
    URL.revokeObjectURL(url)
  }
  const readBackup = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) importBackup.mutate(file)
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const input = inputFromEditor(editor)
    if (input) save.mutate(input)
  }
  const createNew = () => {
    setSelectedId(null)
    setEditor(emptyEditor())
  }
  const mutationError = save.error ?? toggle.error ?? runNow.error ?? remove.error ?? batch.error ?? importBackup.error

  return (
    <section className="subscription-page">
      <header className="view-header">
        <div><p className="eyebrow">AUTOMATION</p><h1>订阅</h1><p>按身份和版本规则持续查找，符合条件后进入同一条可恢复工作流。</p></div>
        <div className="view-header-actions">
          <input ref={importInput} aria-label="选择订阅备份文件" className="sr-only" type="file" accept="application/json,.json" onChange={(event) => void readBackup(event)} />
          <IconButton label="导入订阅备份" onClick={() => importInput.current?.click()}><Upload size={16} /></IconButton>
          <IconButton label="导出订阅备份" onClick={() => void downloadBackup()}><Download size={16} /></IconButton>
          <button className="secondary-command" disabled={!subscriptions.data?.subscriptions.length || batch.isPending} onClick={() => batch.mutate(false)} type="button"><Pause size={16} />全部暂停</button>
          <button className="secondary-command" disabled={!subscriptions.data?.subscriptions.length || batch.isPending} onClick={() => batch.mutate(true)} type="button"><Play size={16} />全部启用</button>
          <button className="secondary-command" onClick={createNew} type="button"><Plus size={16} />新建订阅</button>
        </div>
      </header>

      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      <div className="subscription-layout">
        <div className="subscription-list" aria-label="订阅列表">
          {subscriptions.isLoading ? <div className="status-loading">正在读取订阅…</div> : null}
          {subscriptions.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>订阅读取失败</strong><span>{subscriptions.error.message}</span></div><button onClick={() => void subscriptions.refetch()} type="button">重试</button></div> : null}
          {subscriptions.data?.subscriptions.map((item) => (
            <button className={item.id === selectedId ? 'subscription-row selected' : 'subscription-row'} key={item.id} onClick={() => setSelectedId(item.id)} type="button">
              <span className={item.enabled ? 'subscription-state enabled' : 'subscription-state'} />
              <span><strong>{item.title}{item.season ? ` · S${item.season}` : ''}</strong><small>{item.mediaType === 'movie' ? '电影' : '剧集'} · TMDB {item.tmdbId}{item.lastEpisode ? ` · 已入库至 E${item.lastEpisode}` : ''}</small></span>
              <span className="subscription-next">{item.enabled ? formatNextRun(item.nextRunAt) : '已暂停'}</span>
            </button>
          ))}
          {!subscriptions.isLoading && subscriptions.data?.subscriptions.length === 0 ? <div className="empty-state"><Clock3 size={26} /><span>还没有订阅</span></div> : null}
        </div>

        <div className="subscription-editor">
          <form onSubmit={submit}>
            <div className="editor-heading"><div><p className="eyebrow">RULES</p><h2>{selected ? '编辑订阅' : '新建订阅'}</h2></div>{selected ? <span>{selected.enabled ? '运行中' : '已暂停'}</span> : null}</div>
            <div className="form-grid">
              <label><span>标题</span><input maxLength={300} required value={editor.title} onChange={(event) => setField(setEditor, 'title', event.target.value)} /></label>
              <label><span>原始标题</span><input maxLength={300} value={editor.originalTitle} onChange={(event) => setField(setEditor, 'originalTitle', event.target.value)} /></label>
              <label><span>TMDB ID</span><input disabled={Boolean(selected)} inputMode="numeric" required value={editor.tmdbId} onChange={(event) => setField(setEditor, 'tmdbId', event.target.value)} /></label>
              <label><span>年份</span><input max="2100" min="0" type="number" value={editor.year} onChange={(event) => setField(setEditor, 'year', event.target.value)} /></label>
              <label><span>类型</span><select disabled={Boolean(selected)} value={editor.mediaType} onChange={(event) => setField(setEditor, 'mediaType', event.target.value as 'movie' | 'series')}><option value="movie">电影</option><option value="series">剧集</option></select></label>
              <label><span>季号</span><input disabled={Boolean(selected) || editor.mediaType === 'movie'} max="100" min="0" type="number" value={editor.season} onChange={(event) => setField(setEditor, 'season', event.target.value)} /></label>
              <label><span>更新策略</span><select value={editor.policy} onChange={(event) => setField(setEditor, 'policy', event.target.value as 'once' | 'upgrade')}><option value="once">入库后停止重复转存</option><option value="upgrade">允许新的资源版本</option></select></label>
              <label><span>轮询间隔（分钟）</span><input max="10080" min="15" required type="number" value={editor.intervalMinutes} onChange={(event) => setField(setEditor, 'intervalMinutes', event.target.value)} /></label>
              <label className="span-two"><span>限定来源 ID（逗号分隔，留空为全部）</span><input value={editor.sourceIds} onChange={(event) => setField(setEditor, 'sourceIds', event.target.value)} /></label>
              <label><span>偏好来源顺序</span><input placeholder="framehdr, juying" value={editor.preferredSources} onChange={(event) => setField(setEditor, 'preferredSources', event.target.value)} /></label>
              <label><span>分辨率（优先顺序）</span><input placeholder="2160p, 1080p" value={editor.resolutions} onChange={(event) => setField(setEditor, 'resolutions', event.target.value)} /></label>
              <label><span>视频编码</span><input placeholder="HEVC, AVC" value={editor.videoCodecs} onChange={(event) => setField(setEditor, 'videoCodecs', event.target.value)} /></label>
              <label><span>动态范围</span><input placeholder="Dolby Vision, HDR10" value={editor.dynamicRanges} onChange={(event) => setField(setEditor, 'dynamicRanges', event.target.value)} /></label>
              <label><span>必须包含的音轨</span><input placeholder="Atmos, TrueHD" value={editor.audioContains} onChange={(event) => setField(setEditor, 'audioContains', event.target.value)} /></label>
              <label><span>最小体积（GiB）</span><input min="0" step="0.1" type="number" value={editor.minSizeGiB} onChange={(event) => setField(setEditor, 'minSizeGiB', event.target.value)} /></label>
              <label><span>最大体积（GiB）</span><input min="0" step="0.1" type="number" value={editor.maxSizeGiB} onChange={(event) => setField(setEditor, 'maxSizeGiB', event.target.value)} /></label>
            </div>
            <div className="check-row"><label><input checked={editor.enabled} onChange={(event) => setField(setEditor, 'enabled', event.target.checked)} type="checkbox" />启用自动运行</label><label><input checked={editor.allowUnknownSize} onChange={(event) => setField(setEditor, 'allowUnknownSize', event.target.checked)} type="checkbox" />允许未知体积</label><label><input checked={editor.preferSmaller} onChange={(event) => setField(setEditor, 'preferSmaller', event.target.checked)} type="checkbox" />同分时优先较小版本</label></div>
            <div className="editor-actions">
              {selected ? <><IconButton label={selected.enabled ? '暂停订阅' : '恢复订阅'} onClick={() => toggle.mutate({ id: selected.id, enabled: !selected.enabled })}>{selected.enabled ? <Pause size={16} /> : <Play size={16} />}</IconButton><IconButton label="立即运行" onClick={() => runNow.mutate(selected.id)}><RefreshCw size={16} /></IconButton><IconButton label="删除订阅" onClick={() => window.confirm('删除此订阅及运行历史？') && remove.mutate(selected.id)}><Trash2 size={16} /></IconButton></> : null}
              <button className="primary-action compact" disabled={save.isPending || !editor.tmdbId.trim() || !editor.title.trim()} type="submit"><Save size={16} />{save.isPending ? '保存中' : '保存订阅'}</button>
            </div>
          </form>

          {selected ? <div className="subscription-runs"><div className="section-heading"><div><p className="eyebrow">RUN HISTORY</p><h2>运行历史<span>{runs.data?.runs.length ?? 0}</span></h2></div></div>{runs.data?.runs.map((run) => <div className="subscription-run" key={run.id}><span className={`run-state ${run.state}`} /> <div><strong>{runLabels[run.state]}</strong><small>{run.message || '无补充信息'} · {formatTime(run.startedAt)}</small></div>{run.transferJobId ? <code>{run.transferJobId.slice(0, 8)}</code> : null}</div>)}{!runs.isLoading && runs.data?.runs.length === 0 ? <div className="empty-inline">还没有运行记录</div> : null}</div> : null}
        </div>
      </div>
    </section>
  )
}

function emptyEditor(): EditorState {
  return {
    tmdbId: '', title: '', originalTitle: '', year: '', mediaType: 'movie', season: '0',
    policy: 'once', enabled: true, intervalMinutes: '60', sourceIds: '', resolutions: '',
    videoCodecs: '', dynamicRanges: '', audioContains: '', preferredSources: '',
    minSizeGiB: '', maxSizeGiB: '', allowUnknownSize: false, preferSmaller: false,
  }
}

function editorFromCandidate(candidate: Candidate): EditorState {
  return { ...emptyEditor(), tmdbId: candidate.tmdbId ?? '', title: candidate.title, year: String(candidate.year || ''), mediaType: candidate.mediaType, season: String(candidate.season ?? 0), sourceIds: candidate.sourceId }
}

function editorFromSubscription(item: Subscription): EditorState {
  return {
    tmdbId: item.tmdbId, title: item.title, originalTitle: item.originalTitle, year: String(item.year || ''),
    mediaType: item.mediaType, season: String(item.season), policy: item.policy, enabled: item.enabled,
    intervalMinutes: String(item.intervalMinutes), sourceIds: item.sourceIds.join(', '),
    resolutions: item.preferences.resolutions.join(', '), videoCodecs: item.preferences.videoCodecs.join(', '),
    dynamicRanges: item.preferences.dynamicRanges.join(', '), audioContains: item.preferences.audioContains.join(', '),
    preferredSources: item.preferences.preferredSources.join(', '),
    minSizeGiB: bytesToGiB(item.preferences.minSizeBytes), maxSizeGiB: bytesToGiB(item.preferences.maxSizeBytes),
    allowUnknownSize: item.preferences.allowUnknownSize, preferSmaller: item.preferences.preferSmaller,
  }
}

function inputFromEditor(editor: EditorState): SubscriptionInput | null {
  const tmdbId = editor.tmdbId.trim()
  const title = editor.title.trim()
  const year = Number(editor.year || 0)
  const season = editor.mediaType === 'movie' ? 0 : Number(editor.season || 0)
  const intervalMinutes = Number(editor.intervalMinutes)
  if (!/^\d+$/.test(tmdbId) || !title || !Number.isInteger(year) || !Number.isInteger(season) || !Number.isInteger(intervalMinutes)) return null
  return {
    tmdbId, title, originalTitle: editor.originalTitle.trim(), year, mediaType: editor.mediaType,
    season, policy: editor.policy, enabled: editor.enabled, intervalMinutes,
    sourceIds: splitValues(editor.sourceIds),
    preferences: {
      ...emptyPreferences,
      resolutions: splitValues(editor.resolutions), videoCodecs: splitValues(editor.videoCodecs),
      dynamicRanges: splitValues(editor.dynamicRanges), audioContains: splitValues(editor.audioContains),
      preferredSources: splitValues(editor.preferredSources), minSizeBytes: gibToBytes(editor.minSizeGiB),
      maxSizeBytes: gibToBytes(editor.maxSizeGiB), allowUnknownSize: editor.allowUnknownSize,
      preferSmaller: editor.preferSmaller,
    },
  }
}

function splitValues(value: string) {
  return [...new Set(value.split(',').map((item) => item.trim()).filter(Boolean))]
}

function gibToBytes(value: string) {
  const number = Number(value || 0)
  return Number.isFinite(number) && number >= 0 ? Math.round(number * 1024 * 1024 * 1024) : 0
}

function bytesToGiB(value: number) {
  return value > 0 ? (value / 1024 / 1024 / 1024).toFixed(1) : ''
}

function setField<Key extends keyof EditorState>(setter: Dispatch<SetStateAction<EditorState>>, key: Key, value: EditorState[Key]) {
  setter((current) => ({ ...current, [key]: value }))
}

function formatNextRun(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) || date.getTime() === 0 ? '等待恢复' : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}
