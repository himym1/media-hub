import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, FolderInput } from 'lucide-react'
import { createShareImport } from '../../shared/api/mediaHub'
import { canSubmitShareImport } from './shareImport'

type Props = {
  onImported: () => void
}

export function ShareImportForm({ onImported }: Props) {
  const queryClient = useQueryClient()
  const [url, setUrl] = useState('')
  const [receiveCode, setReceiveCode] = useState('')
  const [title, setTitle] = useState('')
  const importShare = useMutation({
    mutationFn: () => createShareImport({
      url: url.trim(),
      receiveCode: receiveCode.trim() || undefined,
      title: title.trim() || undefined,
    }, crypto.randomUUID()),
    onSuccess: async () => {
      setUrl('')
      setReceiveCode('')
      setTitle('')
      await queryClient.invalidateQueries({ queryKey: ['transfers'] })
      onImported()
    },
  })

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (!canSubmitShareImport(url, receiveCode) || importShare.isPending) return
    importShare.mutate()
  }

  return (
    <section className="share-import" aria-labelledby="share-import-heading">
      <div className="share-import-copy">
        <h2 id="share-import-heading">导入视频</h2>
        <p>贴 115 分享、磁力或视频直链。115 离线拉到成人库，文件不经过 Media Hub。有番号更容易刮封面。</p>
      </div>
      <form className="share-import-form" onSubmit={handleSubmit}>
        <label>
          <span className="sr-only">导入链接</span>
          <input
            autoComplete="off"
            maxLength={8192}
            name="share-url"
            onChange={(event) => setUrl(event.target.value)}
            placeholder="115 分享、磁力或视频直链"
            type="text"
            value={url}
          />
        </label>
        <label>
          <span className="sr-only">提取码</span>
          <input
            autoComplete="off"
            maxLength={8}
            name="share-code"
            onChange={(event) => setReceiveCode(event.target.value)}
            placeholder="提取码"
            value={receiveCode}
          />
        </label>
        <label>
          <span className="sr-only">番号或标题</span>
          <input
            autoComplete="off"
            maxLength={300}
            name="share-title"
            onChange={(event) => setTitle(event.target.value)}
            placeholder="番号或标题，可选"
            value={title}
          />
        </label>
        <button disabled={!canSubmitShareImport(url, receiveCode) || importShare.isPending} type="submit">
          <FolderInput size={16} />
          {importShare.isPending ? '正在导入…' : '导入成人库'}
        </button>
      </form>
      {importShare.isError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{importShare.error.message}</span></div> : null}
      {importShare.isSuccess ? <p className="share-import-success" role="status">已加入转存队列</p> : null}
    </section>
  )
}
