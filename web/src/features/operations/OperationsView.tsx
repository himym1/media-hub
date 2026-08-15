import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Archive, CircleAlert, FolderOpen, HardDrive, RefreshCw, Upload } from 'lucide-react'
import {
  confirmArchivePlan,
  createArchivePlan,
  confirmDrive115Command,
  createDrive115Command,
  createLocalUpload,
  listArchivePlans,
  listDrive115Commands,
  listDrive115Files,
  listLocalUploadFiles,
  listLocalUploadRoots,
  listLocalUploads,
  type Drive115Command,
  previewArchive,
  retryArchivePlan,
  type ArchiveStep,
  retryDrive115Command,
  retryLocalUpload,
  type LocalUploadJob,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import { buildArchiveSteps } from './archivePlan'
import './OperationsView.css'


export function OperationsView() {
  return (
    <section className="operations-view" aria-label="运营工具">
      <header className="view-heading">
        <div>
          <span className="eyebrow">OPERATIONS</span>
          <h1>运营工具</h1>
        </div>
        <span className="service-state unconfigured">原生模式</span>
      </header>

      <Drive115Workspace />
      <LocalUploadWorkspace />
      <ArchiveWorkspace />

    </section>
  )
}

function Drive115Workspace() {
  const queryClient = useQueryClient()
  const [parentId, setParentId] = useState('0')
  const [operation, setOperation] = useState<Drive115Command['operation']>('create_folder')
  const [fileIds, setFileIds] = useState('')
  const [targetParentId, setTargetParentId] = useState('0')
  const [name, setName] = useState('')
  const files = useQuery({ queryKey: ['drive-115-files', parentId], queryFn: () => listDrive115Files(parentId), retry: false })
  const commands = useQuery({
    queryKey: ['drive-115-commands'],
    queryFn: listDrive115Commands,
    refetchInterval: (query) => query.state.data?.some((item) => ['queued', 'submitting'].includes(item.state)) ? 2_000 : false,
  })
  const create = useMutation({
    mutationFn: () => createDrive115Command(operation, buildDrive115Params(operation, fileIds, targetParentId, name)),
    onSuccess: () => {
      setName(''); setFileIds('')
      void queryClient.invalidateQueries({ queryKey: ['drive-115-commands'] })
      void queryClient.invalidateQueries({ queryKey: ['drive-115-files'] })
    },
  })
  const confirm = useMutation({
    mutationFn: (command: Drive115Command) => command.state === 'awaiting_confirmation' ? confirmDrive115Command(command) : retryDrive115Command(command),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['drive-115-commands'] }),
  })
  return <section className="drive-workspace" aria-label="115 原生文件运营">
    <div className="section-heading"><h3><HardDrive size={17} />115 文件</h3><IconButton label="刷新 115 文件" onClick={() => void files.refetch()}><RefreshCw size={15} /></IconButton></div>
    <label className="form-field"><span>当前目录 ID</span><input inputMode="numeric" pattern="[0-9]+" value={parentId} onChange={(event) => setParentId(event.target.value)} /></label>
    {files.isError ? <InlineError message="无法读取 115 目录" /> : <div className="drive-files">{files.data?.items.map((item) => <button key={item.id} disabled={item.kind !== 'folder'} onClick={() => item.kind === 'folder' && setParentId(item.id)} type="button"><FolderOpen size={15} /><span><strong>{item.name}</strong><small>{item.kind === 'folder' ? `目录 ${item.id}` : `${formatBytes(item.size)} · ${item.id}`}</small></span></button>)}{files.data?.items.length === 0 ? <p className="state-copy">目录为空</p> : null}</div>}
    <div className="drive-command-form">
      <label className="form-field"><span>文件操作</span><select value={operation} onChange={(event) => setOperation(event.target.value as Drive115Command['operation'])}><option value="create_folder">建目录</option><option value="move">移动</option><option value="rename">重命名</option><option value="delete">删除</option></select></label>
      {operation !== 'create_folder' && operation !== 'rename' ? <label className="form-field"><span>文件 ID（逗号分隔）</span><input value={fileIds} onChange={(event) => setFileIds(event.target.value)} /></label> : null}
      {operation === 'rename' ? <label className="form-field"><span>文件 ID</span><input value={fileIds} onChange={(event) => setFileIds(event.target.value)} /></label> : null}
      {operation === 'create_folder' || operation === 'move' ? <label className="form-field"><span>{operation === 'move' ? '目标目录 ID' : '父目录 ID'}</span><input inputMode="numeric" value={targetParentId} onChange={(event) => setTargetParentId(event.target.value)} /></label> : null}
      {operation === 'create_folder' || operation === 'rename' ? <label className="form-field"><span>名称</span><input maxLength={255} value={name} onChange={(event) => setName(event.target.value)} /></label> : null}
      <button className={operation === 'delete' ? 'danger-button' : 'primary-button'} disabled={create.isPending} onClick={() => create.mutate()} type="button">{create.isPending ? '正在持久化' : operation === 'delete' ? '创建待确认删除命令' : '创建命令'}</button>
      {create.isError ? <InlineError message={create.error.message} /> : null}
    </div>
    <div className="drive-command-list">{commands.data?.slice(0, 8).map((command) => <div key={command.id}><span><strong>{command.operation}</strong><small>{command.id}</small></span><span className={`task-state ${command.state}`}>{commandStateLabel(command.state)}</span>{command.state === 'awaiting_confirmation' || command.state === 'failed' || command.state === 'needs_attention' ? <button className="danger-button" disabled={confirm.isPending} onClick={() => confirm.mutate(command)} type="button">{command.state === 'awaiting_confirmation' ? '确认删除' : '确认重试'}</button> : null}</div>)}</div>
  </section>
}

function LocalUploadWorkspace() {
  const queryClient = useQueryClient()
  const roots = useQuery({ queryKey: ['local-upload-roots'], queryFn: listLocalUploadRoots, retry: false })
  const [selectedRoot, setSelectedRoot] = useState('')
  const [path, setPath] = useState('')
  const [selectedFile, setSelectedFile] = useState('')
  const [destinationId, setDestinationId] = useState('')
  const rootId = selectedRoot || roots.data?.[0]?.id || ''
  const files = useQuery({ queryKey: ['local-upload-files', rootId, path], queryFn: () => listLocalUploadFiles(rootId, path), enabled: Boolean(rootId), retry: false })
  const uploads = useQuery({ queryKey: ['local-uploads'], queryFn: listLocalUploads, retry: false, refetchInterval: (query) => query.state.data?.some((item) => ['queued', 'hashing', 'submitting_init', 'uploading'].includes(item.state)) ? 2_000 : false })
  const create = useMutation({
    mutationFn: () => createLocalUpload(rootId, selectedFile, destinationId.trim()),
    onSuccess: () => { setSelectedFile(''); void queryClient.invalidateQueries({ queryKey: ['local-uploads'] }) },
  })
  const retry = useMutation({ mutationFn: retryLocalUpload, onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['local-uploads'] }) })
  if (roots.data?.length === 0) return null
  return <section className="local-upload-workspace" aria-label="本地上传">
    <div className="section-heading"><h3><Upload size={17} />本地上传</h3><IconButton label="刷新本地文件" onClick={() => void files.refetch()}><RefreshCw size={15} /></IconButton></div>
    <div className="local-upload-controls"><label className="form-field"><span>白名单根目录</span><select value={rootId} onChange={(event) => { setSelectedRoot(event.target.value); setPath(''); setSelectedFile('') }}>{roots.data?.map((root) => <option key={root.id}>{root.id}</option>)}</select></label><label className="form-field"><span>相对目录</span><input value={path} onChange={(event) => setPath(event.target.value)} /></label><button className="secondary-command" disabled={!path} onClick={() => setPath(path.split('/').slice(0, -1).join('/'))} type="button">返回上级</button></div>
    {files.isError ? <InlineError message="无法读取白名单目录" /> : <div className="local-files">{files.data?.map((item) => <button className={selectedFile === item.path ? 'active' : ''} key={item.path} onClick={() => item.directory ? (setPath(item.path), setSelectedFile('')) : setSelectedFile(item.path)} type="button"><FolderOpen size={15} /><span><strong>{item.name}</strong><small>{item.directory ? '目录' : formatBytes(item.size)}</small></span></button>)}</div>}
    <div className="local-upload-submit"><span>{selectedFile || '请选择一个文件'}</span><input aria-label="115 目标目录 ID" inputMode="numeric" placeholder="115 目标目录 ID" value={destinationId} onChange={(event) => setDestinationId(event.target.value)} /><button className="primary-button" disabled={!selectedFile || !/^\d+$/.test(destinationId) || create.isPending} onClick={() => create.mutate()} type="button"><Upload size={15} />{create.isPending ? '正在创建' : '创建上传任务'}</button></div>
    {create.isError ? <InlineError message={create.error.message} /> : null}
    <div className="local-upload-jobs">{uploads.data?.slice(0, 8).map((upload) => <div key={upload.id}><span><strong>{upload.path}</strong><small>{upload.bytesTotal ? `${Math.round(upload.bytesDone / upload.bytesTotal * 100)}% · ${formatBytes(upload.bytesTotal)}` : upload.id}</small></span><span className={`task-state ${upload.state}`}>{localUploadState(upload.state)}</span>{upload.state === 'failed' || upload.state === 'needs_attention' ? <button className="danger-button" disabled={retry.isPending} onClick={() => retry.mutate(upload)} type="button">确认重试</button> : null}</div>)}</div>
  </section>
}

function localUploadState(state: LocalUploadJob['state']) {
  return ({ queued: '等待中', hashing: '校验中', submitting_init: '初始化', uploading: '上传中', completed: '已完成', failed: '失败', needs_attention: '待核对' } as const)[state]
}

function buildDrive115Params(operation: Drive115Command['operation'], fileIds: string, targetParentId: string, name: string): Record<string, unknown> {
  const ids = fileIds.split(',').map((value) => value.trim()).filter(Boolean)
  if (operation === 'create_folder') return { parentId: targetParentId.trim(), name: name.trim() }
  if (operation === 'move') return { fileIds: ids, targetParentId: targetParentId.trim() }
  if (operation === 'rename') return { fileId: ids[0] ?? '', name: name.trim() }
  return { fileIds: ids }
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(1)} GB`
}


function commandStateLabel(state: string) {
  return ({ awaiting_confirmation: '待确认', queued: '等待中', submitting: '提交中', completed: '已完成', failed: '失败', needs_attention: '待核对' } as Record<string, string>)[state] ?? state
}


function InlineError({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className="inline-error" role="alert"><CircleAlert size={16} /><div><strong>操作失败</strong><span>{message}</span></div>{onRetry && <button onClick={onRetry}>重试</button>}</div>
}

function ArchiveWorkspace() {
  const queryClient = useQueryClient()
  const [parentId, setParentId] = useState('0')
  const [targetParentId, setTargetParentId] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [names, setNames] = useState<Record<string, string>>({})
  const preview = useMutation({
    mutationFn: () => previewArchive(parentId),
    onSuccess: (value) => {
      setSelected(new Set())
      setNames(Object.fromEntries(value.suggestions.map((item) => [item.fileId, item.suggestedName])))
    },
  })
  const plans = useQuery({ queryKey: ['archive-plans'], queryFn: listArchivePlans, refetchInterval: 3_000 })
  const refreshPlans = () => queryClient.invalidateQueries({ queryKey: ['archive-plans'] })
  const create = useMutation({ mutationFn: (steps: ArchiveStep[]) => createArchivePlan(steps), onSuccess: refreshPlans })
  const confirm = useMutation({ mutationFn: confirmArchivePlan, onSuccess: refreshPlans })
  const retry = useMutation({ mutationFn: retryArchivePlan, onSuccess: refreshPlans })
  const suggestions = preview.data?.suggestions ?? []
  const steps = buildArchiveSteps(suggestions, selected, names, targetParentId)
  return (
    <section className="drive-workspace" aria-labelledby="archive-heading">
      <div className="operation-heading"><div className="operation-icon"><Archive size={20} /></div><div><h2 id="archive-heading">原生归档整理</h2><p>预览不会修改文件；只有选中的显式步骤在计划 ID 二次确认后才会顺序执行。</p></div></div>
      <div className="drive-command-grid">
        <label><span>来源目录 ID</span><input value={parentId} onChange={(event) => setParentId(event.target.value)} /></label>
        <label><span>移动目标目录 ID（可选）</span><input value={targetParentId} onChange={(event) => setTargetParentId(event.target.value)} /></label>
        <button type="button" className="primary-command" disabled={preview.isPending || !parentId.trim()} onClick={() => preview.mutate()}>预览建议</button>
      </div>
      {preview.error && <p className="operation-error">{String(preview.error)}</p>}
      {suggestions.length > 0 && <div className="archive-suggestions">
        {suggestions.map((item) => <label className="archive-suggestion" key={item.fileId}>
          <input
            type="checkbox"
            checked={selected.has(item.fileId)}
            onChange={(event) => setSelected((current) => {
              const next = new Set(current)
              if (event.target.checked) next.add(item.fileId)
              else next.delete(item.fileId)
              return next
            })}
          />
          <span><strong>{item.currentName}</strong><small>{item.kind === 'directory' ? '目录' : '文件'} · 需人工复核</small></span>
          <input aria-label={`${item.currentName} 的新名称`} value={names[item.fileId] ?? item.suggestedName} onChange={(event) => setNames((current) => ({ ...current, [item.fileId]: event.target.value }))} />
        </label>)}
        <button type="button" className="primary-command" disabled={steps.length === 0 || create.isPending} onClick={() => create.mutate(steps)}>保存待确认计划（{steps.length} 步）</button>
      </div>}
      <div className="drive-command-history"><h3>归档计划</h3>
        {(plans.data ?? []).map((plan) => <article className="drive-command-row" key={plan.id}><div><strong>{plan.state}</strong><small>{plan.stepIndex} / {plan.stepTotal} 步 · {plan.id}</small>{plan.errorMessage && <small>{plan.errorMessage}</small>}</div>{plan.state === 'awaiting_confirmation' && <button type="button" className="danger-command" onClick={() => confirm.mutate(plan.id)}>确认执行计划 ID</button>}{(plan.state === 'failed' || plan.state === 'needs_attention') && <button type="button" className="danger-command" onClick={() => retry.mutate(plan.id)}>核对后继续</button>}</article>)}
      </div>
    </section>
  )
}
