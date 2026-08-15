import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { CheckCircle2, CircleAlert, Clock3, ListTodo, RefreshCw, RotateCcw } from 'lucide-react'
import {
  getTransfer,
  listTransferNotifications,
  retryTransfer,
  retryTransferNotification,
  type TransferEvent,
  type TransferJob,
  type TransferNotification,
  type TransferState,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'

const stateLabel: Record<TransferState, string> = {
  queued: '排队中',
  transferring: '正在转存',
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
const runningStates = new Set<TransferState>(['queued', 'transferring', 'retry_wait', 'transferred', 'submitting_sync', 'syncing', 'refreshing_emby', 'indexing_emby', 'verifying_playback'])
const timeFormatter = new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })

type TransferQueueProps = {
  query: UseQueryResult<{ transfers: TransferJob[] }, Error>
}

export function TransferQueue({ query }: TransferQueueProps) {
  const queryClient = useQueryClient()
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const jobs = useMemo(() => query.data?.transfers ?? [], [query.data?.transfers])
  const activeJobs = jobs.filter((job) => runningStates.has(job.state) || job.state === 'needs_attention')
  const historyJobs = jobs.filter((job) => !activeJobs.includes(job))

  useEffect(() => {
    if (!selectedID && jobs[0]) setSelectedID(jobs[0].id)
    if (selectedID && !jobs.some((job) => job.id === selectedID)) setSelectedID(jobs[0]?.id ?? null)
  }, [jobs, selectedID])

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
  const notifications = useQuery({
    queryKey: ['transfer-notifications'],
    queryFn: () => listTransferNotifications(),
    refetchInterval: (state) => state.state.data?.notifications.some((item) => item.state === 'pending' || item.state === 'submitting') ? 4_000 : false,
  })
  const selectedNotification = notifications.data?.notifications.find((item) => item.jobId === selectedID && item.state === 'needs_attention')
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
        <div><p className="eyebrow">WORKFLOW</p><h1>任务</h1><p>查看当前进度，失败恢复和技术事件按需展开。</p></div>
        <IconButton label="刷新任务" onClick={() => void query.refetch()} subtle><RefreshCw size={17} /></IconButton>
      </header>

      {query.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>任务读取失败</strong><span>{query.error.message}</span></div><button onClick={() => void query.refetch()} type="button">重试</button></div> : null}
      {query.isLoading ? <div className="result-loading"><div /><div /><div /></div> : null}
      {!query.isLoading && jobs.length === 0 ? <div className="empty-state"><ListTodo size={28} /><span>还没有转存任务</span></div> : null}

      {jobs.length > 0 ? (
        <div className="task-layout">
          <div className="task-list" role="list">
            <TaskGroup label="进行中" jobs={activeJobs} selectedID={selectedID} onSelect={setSelectedID} />
            <TaskGroup label="历史记录" jobs={historyJobs} selectedID={selectedID} onSelect={setSelectedID} />
          </div>

          <aside className="task-detail" aria-label="任务详情">
            {detail.isLoading ? <div className="status-loading">正在读取…</div> : null}
            {detail.data ? <>
              <div className="task-detail-heading"><div><p className="eyebrow">DETAIL</p><h2>{detail.data.title}</h2></div><span className={`state-chip ${detail.data.state}`}>{stateLabel[detail.data.state]}</span></div>
              {detail.data.errorMessage ? <div className="task-error" role="alert"><CircleAlert size={17} /><span>{detail.data.errorMessage}</span></div> : null}
              <dl className="task-facts"><div><dt>类型</dt><dd>{detail.data.mediaType === 'movie' ? '电影' : '剧集'}</dd></div><div><dt>来源</dt><dd>{detail.data.source}</dd></div><div><dt>创建</dt><dd>{timeFormatter.format(new Date(detail.data.createdAt))}</dd></div></dl>
              <div className="task-progress-heading"><strong>最近进度</strong><span>{detail.data.events.length} 条记录</span></div>
              {renderEvents(recentEvents)}
              {detail.data.events.length > recentEvents.length ? <details className="event-history"><summary>查看全部技术记录</summary>{renderEvents(detail.data.events)}</details> : null}
              {selectedNotification ? <div className="notification-recovery"><CircleAlert size={17} /><div><strong>企业微信通知结果未知</strong><span>再次发送可能产生重复消息。</span></div><button disabled={retryNotification.isPending} onClick={() => retryNotification.mutate(selectedNotification)} type="button"><RotateCcw size={15} />{retryNotification.isPending ? '正在提交…' : '确认并重发'}</button></div> : null}
              {detail.data.retryable ? <button className="secondary-action" disabled={retry.isPending} onClick={() => retry.mutate(detail.data.id)} type="button"><RotateCcw size={16} />{retry.isPending ? '正在重试…' : '重试任务'}</button> : null}
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
      <span className="task-copy"><strong>{transferTitle(job)}</strong><small>{job.source} · {timeFormatter.format(new Date(job.updatedAt))}</small></span>
      <span className={`state-chip ${job.state}`}>{stateLabel[job.state]}</span>
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
