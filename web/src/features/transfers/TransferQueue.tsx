import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { Archive, ArchiveRestore, CheckCircle2, CircleAlert, Clock3, ListTodo, RefreshCw, RotateCcw, Trash2 } from 'lucide-react'
import {
  deleteTransfer,
  getTransfer,
  listTransferNotifications,
  listTransfers,
  retryTransfer,
  retryTransferNotification,
  setTransferArchived,
  type TransferEvent,
  type TransferJob,
  type TransferNotification,
  type TransferState,
} from '../../shared/api/mediaHub'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'

function jobMediaTypeLabel(mediaType: TransferJob['mediaType']) {
  if (mediaType === 'adult') return '成人'
  return mediaType === 'series' ? '剧集' : '电影'
}

function jobStateLabel(state: TransferState, source?: string) {
  if (source === 'moviepilot' && (state === 'transferring' || state === 'queued')) {
    return state === 'queued' ? '排队中' : '正在提交下载'
  }
  return stateLabel[state]
}

const stateLabel: Record<TransferState, string> = {
  queued: '排队中',
  transferring: '正在转存',
  downloading: '正在提交下载',
  retry_wait: '等待重试',
  transferred: '已转存',
  submitting_sync: '提交同步',
  syncing: '生成 STRM',
  refreshing_emby: '刷新 Emby',
  indexing_emby: '等待入库',
  verifying_playback: '验证播放',
  completed: '已完成',
  failed: '失败',
  needs_attention: '需要确认',
}
const runningStates = new Set<TransferState>(['queued', 'transferring', 'downloading', 'retry_wait', 'transferred', 'submitting_sync', 'syncing', 'refreshing_emby', 'indexing_emby', 'verifying_playback'])
const timeFormatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })

function taskIdFromLocation() {
  return new URLSearchParams(window.location.search).get('task')
}

function archivedFromLocation() {
  return new URLSearchParams(window.location.search).get('archive') === '1'
}

type TransferQueueProps = {
  query: UseQueryResult<{ transfers: TransferJob[] }, Error>
}

export function TransferQueue({ query }: TransferQueueProps) {
  const queryClient = useQueryClient()
  const [selectedID, setSelectedID] = useState<string | null>(taskIdFromLocation)
  const [showArchived, setShowArchived] = useState(archivedFromLocation)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [deletedIDs, setDeletedIDs] = useState<string[]>([])
  const [statusFilter, setStatusFilter] = useState<'all' | 'running' | 'completed' | 'issues'>('all')
  const archivedQuery = useQuery({
    queryKey: ['transfers', 'archived'],
    queryFn: () => listTransfers(100, true),
    enabled: showArchived,
  })
  const currentQuery = showArchived ? archivedQuery : query
  const jobs = useMemo(
    () => (currentQuery.data?.transfers ?? []).filter((job) => !deletedIDs.includes(job.id)),
    [currentQuery.data?.transfers, deletedIDs],
  )
  const activeJobs = jobs.filter((job) => runningStates.has(job.state) || job.state === 'needs_attention')
  const historyJobs = jobs.filter((job) => !activeJobs.includes(job))

  const displayJobs = useMemo(() => {
    if (showArchived) return jobs
    if (statusFilter === 'running') return jobs.filter((job) => runningStates.has(job.state))
    if (statusFilter === 'completed') return jobs.filter((job) => job.state === 'completed')
    if (statusFilter === 'issues') return jobs.filter((job) => job.state === 'failed' || job.state === 'needs_attention')
    return jobs
  }, [jobs, showArchived, statusFilter])
  const displayActiveJobs = useMemo(() => displayJobs.filter((job) => activeJobs.includes(job)), [displayJobs, activeJobs])
  const displayHistoryJobs = useMemo(() => displayJobs.filter((job) => historyJobs.includes(job)), [displayJobs, historyJobs])

  useEffect(() => {
    const restoreTask = () => {
      setSelectedID(taskIdFromLocation())
      setShowArchived(archivedFromLocation())
    }
    window.addEventListener('popstate', restoreTask)
    return () => window.removeEventListener('popstate', restoreTask)
  }, [])

  useEffect(() => {
    setConfirmingDelete(false)
  }, [selectedID])

  useEffect(() => {
    if (!currentQuery.data) return
    if (selectedID && jobs.some((job) => job.id === selectedID)) return
    const fallback = jobs[0]?.id ?? null
    setSelectedID(fallback)
    commitUrl({ task: fallback }, 'replace')
  }, [currentQuery.data, jobs, selectedID])

  const selectTask = (id: string) => {
    setSelectedID(id)
    commitUrl({ task: id })
  }

  const selectScope = (archived: boolean) => {
    setShowArchived(archived)
    setSelectedID(null)
    commitUrl({ archive: archived ? '1' : null, task: null })
  }

  const detail = useQuery({
    queryKey: ['transfer', selectedID],
    queryFn: () => getTransfer(selectedID!),
    enabled: Boolean(selectedID),
    refetchInterval: (state) => runningStates.has(state.state.data?.state ?? 'completed') ? 4_000 : false,
  })
  const retry = useMutation({
    mutationFn: retryTransfer,
    onSuccess: async (job) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['transfers'] }),
        queryClient.invalidateQueries({ queryKey: ['transfer', job.id] }),
      ])
    },
  })
  const archive = useMutation({
    mutationFn: ({ id, archived }: { id: string; archived: boolean }) => setTransferArchived(id, archived),
    onSuccess: async (job) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['transfers'] }),
        queryClient.invalidateQueries({ queryKey: ['transfer', job.id] }),
      ])
    },
  })
  const remove = useMutation({
    mutationFn: deleteTransfer,
    onSuccess: async (_result, id) => {
      setConfirmingDelete(false)
      setDeletedIDs((current) => current.includes(id) ? current : [...current, id])
      if (selectedID === id) {
        setSelectedID(null)
        commitUrl({ task: null })
      }
      queryClient.setQueriesData<{ transfers: TransferJob[] }>({ queryKey: ['transfers'] }, (current) => (
        current ? { transfers: current.transfers.filter((job) => job.id !== id) } : current
      ))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['transfers'] }),
        queryClient.invalidateQueries({ queryKey: ['transfer-notifications'] }),
      ])
    },
  })
  const notifications = useQuery({
    queryKey: ['transfer-notifications'],
    queryFn: () => listTransferNotifications(),
    enabled: !showArchived,
    refetchInterval: (state) => state.state.data?.notifications.some((item) => item.state === 'pending' || item.state === 'submitting') ? 4_000 : false,
  })
  const selectedNotification = showArchived ? undefined : notifications.data?.notifications.find((item) => item.jobId === selectedID && item.state === 'needs_attention')
  const retryNotification = useMutation({
    mutationFn: (item: TransferNotification) => retryTransferNotification(item),
    onSuccess: async (item) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['transfer-notifications'] }),
        queryClient.invalidateQueries({ queryKey: ['transfer', item.jobId] }),
      ])
    },
  })
  const recentEvents = detail.data?.events.slice(-4) ?? []

  return (
    <section className="workspace-view">
      <header className="view-header compact-view-header">
        <div><h1>任务</h1><p>查看当前进度，失败恢复和技术事件按需展开。</p></div>
        <div className="view-header-actions">
          <div className="task-scope-toggle" aria-label="任务范围" role="group">
            <button aria-pressed={!showArchived} onClick={() => selectScope(false)} type="button">当前</button>
            <button aria-pressed={showArchived} onClick={() => selectScope(true)} type="button">已归档</button>
          </div>
          <IconButton label="刷新任务" onClick={() => void currentQuery.refetch()} subtle><RefreshCw size={17} /></IconButton>
        </div>
      </header>

      {currentQuery.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>任务读取失败</strong><span>{currentQuery.error.message}</span></div><button onClick={() => void currentQuery.refetch()} type="button">重试</button></div> : null}
      {currentQuery.isLoading && jobs.length === 0 ? <div className="result-loading"><div /><div /><div /></div> : null}
      {!currentQuery.isLoading && jobs.length === 0 ? <div className="empty-state"><ListTodo size={28} /><span>{showArchived ? '还没有归档任务' : '还没有转存任务'}</span></div> : null}

      {jobs.length > 0 ? (
        <div className="task-layout">
          <div className="task-list">
            {!showArchived && (
              <div aria-label="按状态过滤任务" className="task-filter-bar" role="group">
                <button
                  aria-pressed={statusFilter === 'all'}
                  className={statusFilter === 'all' ? 'filter-chip active' : 'filter-chip'}
                  onClick={() => setStatusFilter('all')}
                  type="button"
                >
                  全部 ({jobs.length})
                </button>
                {activeJobs.length > 0 ? (
                  <button
                    aria-pressed={statusFilter === 'running'}
                    className={statusFilter === 'running' ? 'filter-chip active' : 'filter-chip'}
                    onClick={() => setStatusFilter('running')}
                    type="button"
                  >
                    进行中 ({activeJobs.length})
                  </button>
                ) : null}
                <button
                  aria-pressed={statusFilter === 'completed'}
                  className={statusFilter === 'completed' ? 'filter-chip active' : 'filter-chip'}
                  onClick={() => setStatusFilter('completed')}
                  type="button"
                >
                  已完成
                </button>
                {jobs.some((j) => j.state === 'failed' || j.state === 'needs_attention') ? (
                  <button
                    aria-pressed={statusFilter === 'issues'}
                    className={statusFilter === 'issues' ? 'filter-chip active' : 'filter-chip'}
                    onClick={() => setStatusFilter('issues')}
                    type="button"
                  >
                    需关注
                  </button>
                ) : null}
              </div>
            )}
            {showArchived ? <TaskGroup label="已归档" jobs={jobs} selectedID={selectedID} onSelect={selectTask} /> : <>
              <TaskGroup label="进行中" jobs={displayActiveJobs} selectedID={selectedID} onSelect={selectTask} />
              <TaskGroup label="历史记录" jobs={displayHistoryJobs} selectedID={selectedID} onSelect={selectTask} />
            </>}
          </div>

          <aside className="task-detail" aria-label="任务详情">
            {detail.isLoading && !detail.data ? <div className="status-loading">正在读取…</div> : null}
            {detail.data ? <>
              <div className="task-detail-heading"><div><h2>{detail.data.title}</h2></div><span className={`state-chip ${detail.data.state}`}>{jobStateLabel(detail.data.state, detail.data.source)}</span></div>
              <div aria-label="工作流阶段" className="task-pipeline">
                {getPipelineStages(detail.data.state, detail.data.source).map((stage, idx) => (
                  <div className={`pipeline-step ${stage.status}`} key={stage.id}>
                    <span className="step-circle">{stage.status === 'completed' ? '✓' : idx + 1}</span>
                    <span className="step-text">{stage.label}</span>
                    {idx < 3 ? <span aria-hidden="true" className="step-divider" /> : null}
                  </div>
                ))}
              </div>
              {detail.data.errorMessage ? <div className="task-error" role="alert"><CircleAlert size={17} /><span>{detail.data.errorMessage}</span></div> : null}
              <dl className="task-facts"><div><dt>类型</dt><dd>{jobMediaTypeLabel(detail.data.mediaType)}</dd></div><div><dt>来源</dt><dd>{detail.data.source === 'share' ? '115分享' : detail.data.source}</dd></div><div><dt>创建</dt><dd>{timeFormatter.format(new Date(detail.data.createdAt))}</dd></div></dl>
              <div className="task-progress-heading"><strong>最近进度</strong><span>{detail.data.events.length} 条记录</span></div>
              {renderEvents(recentEvents)}
              {detail.data.events.length > recentEvents.length ? <details className="event-history"><summary>查看全部技术记录</summary>{renderEvents(detail.data.events)}</details> : null}
              {selectedNotification ? <div className="notification-recovery"><CircleAlert size={17} /><div><strong>企业微信通知结果未知</strong><span>再次发送可能产生重复消息。</span></div><button disabled={retryNotification.isPending} onClick={() => retryNotification.mutate(selectedNotification)} type="button"><RotateCcw size={15} />{retryNotification.isPending ? '正在提交…' : '确认并重发'}</button></div> : null}
              {detail.data.retryable ? <button className="secondary-action" disabled={retry.isPending} onClick={() => retry.mutate(detail.data.id)} type="button"><RotateCcw size={16} />{retry.isPending ? '正在重试…' : '重试任务'}</button> : null}
              {archive.error ? <div className="task-error" role="alert"><CircleAlert size={17} /><span>{archive.error.message}</span></div> : null}
              {remove.error ? <div className="task-error" role="alert"><CircleAlert size={17} /><span>{remove.error.message}</span></div> : null}
              {detail.data.state === 'completed' ? (
                <div className="task-action-row">
                  <a className="secondary-action" href="?view=library">前往媒体库</a>
                  <button className="secondary-action" disabled={archive.isPending} onClick={() => archive.mutate({ id: detail.data.id, archived: !showArchived })} type="button">{showArchived ? <ArchiveRestore size={16} /> : <Archive size={16} />}{archive.isPending ? '正在处理…' : showArchived ? '恢复到任务列表' : '归档任务'}</button>
                </div>
              ) : null}
              {detail.data.state === 'failed' || detail.data.state === 'needs_attention' ? (
                confirmingDelete ? (
                  <div className="inline-delete-confirm">
                    <p>删除这条失败任务记录？不会影响 115 / Emby 中的媒体。</p>
                    <div className="inline-delete-confirm-actions">
                      <button className="danger-button" disabled={remove.isPending} onClick={() => remove.mutate(detail.data.id)} type="button">{remove.isPending ? '正在删除…' : '确认删除'}</button>
                      <button className="secondary-action" disabled={remove.isPending} onClick={() => setConfirmingDelete(false)} type="button">取消</button>
                    </div>
                  </div>
                ) : (
                  <button className="danger-button" disabled={remove.isPending} onClick={() => setConfirmingDelete(true)} type="button"><Trash2 size={16} />删除任务</button>
                )
              ) : null}
            </> : null}
          </aside>
        </div>
      ) : null}
    </section>
  )
}

function TaskGroup({ label, jobs, selectedID, onSelect }: { label: string; jobs: TransferJob[]; selectedID: string | null; onSelect: (id: string) => void }) {
  if (jobs.length === 0) return null
  return <section className="task-group"><div className="task-group-heading"><strong>{label}</strong><span>{jobs.length}</span></div>{jobs.map((job) => (
    <button aria-pressed={selectedID === job.id} className={selectedID === job.id ? 'task-row selected' : 'task-row'} key={job.id} onClick={() => onSelect(job.id)} type="button">
      <span className={`task-state-mark ${job.state}`} aria-hidden="true" />
      <span className="task-copy"><strong>{transferTitle(job)}</strong><small>{job.source} · {timeFormatter.format(new Date(job.updatedAt))}</small>{job.errorMessage ? <small className="task-row-error">{job.errorMessage}</small> : null}</span>
      <span className={`state-chip ${job.state}`}>{jobStateLabel(job.state, job.source)}</span>
    </button>
  ))}</section>
}

function renderEvents(events: TransferEvent[]) {
  return <ol className="event-list">{events.map((event) => <li key={event.id}>{event.state === 'completed' ? <CheckCircle2 size={16} /> : event.state === 'failed' || event.state === 'needs_attention' ? <CircleAlert size={16} /> : <Clock3 size={16} />}<div><strong>{stateLabel[event.state]}</strong><span>{event.message || '状态已更新'}</span><time>{timeFormatter.format(new Date(event.createdAt))}</time></div></li>)}</ol>
}

function transferTitle(job: TransferJob) {
  if (!job.season) return job.title
  if (!job.episodeStart) return `${job.title} · S${job.season}`
  const episodes = job.episodeStart === job.episodeEnd ? `${job.episodeStart}` : `${job.episodeStart}-${job.episodeEnd}`
  return `${job.title} · S${job.season}E${episodes}`
}

type PipelineStage = {
  id: string
  label: string
  status: 'completed' | 'active' | 'failed' | 'pending'
}

function getPipelineStages(state: TransferState, source?: string): PipelineStage[] {
  if (source === 'moviepilot') {
    const submitting = state === 'queued' || state === 'transferring' || state === 'retry_wait'
    return [
      { id: 'd1', label: '提交下载', status: state === 'completed' ? 'completed' : submitting ? 'active' : state === 'failed' || state === 'needs_attention' ? 'failed' : 'pending' },
      { id: 'd2', label: 'MoviePilot', status: state === 'completed' ? 'completed' : 'pending' },
    ]
  }
  let s1: PipelineStage['status'] = 'pending'
  let s2: PipelineStage['status'] = 'pending'
  let s3: PipelineStage['status'] = 'pending'
  let s4: PipelineStage['status'] = 'pending'

  switch (state) {
    case 'queued':
    case 'transferring':
    case 'downloading':
    case 'retry_wait':
      s1 = 'active'
      break
    case 'transferred':
      s1 = 'completed'
      s2 = 'active'
      break
    case 'submitting_sync':
    case 'syncing':
      s1 = 'completed'
      s2 = 'active'
      break
    case 'refreshing_emby':
    case 'indexing_emby':
      s1 = 'completed'
      s2 = 'completed'
      s3 = 'active'
      break
    case 'verifying_playback':
      s1 = 'completed'
      s2 = 'completed'
      s3 = 'completed'
      s4 = 'active'
      break
    case 'completed':
      s1 = 'completed'
      s2 = 'completed'
      s3 = 'completed'
      s4 = 'completed'
      break
    case 'failed':
    case 'needs_attention':
      s1 = 'completed'
      s2 = 'failed'
      break
  }

  return [
    { id: 't1', label: '115 转存', status: s1 },
    { id: 't2', label: 'STRM 生成', status: s2 },
    { id: 't3', label: 'Emby 刮削', status: s3 },
    { id: 't4', label: '可用验证', status: s4 },
  ]
}
