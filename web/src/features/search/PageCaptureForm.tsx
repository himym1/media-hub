import { useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { CircleAlert, FolderOpen, Radar } from 'lucide-react'
import { createShareImport, initCaptureUpload } from '../../shared/api/mediaHub'
import {
  canCapturePages,
  captureFileName,
  completeCaptureUploadWhenReady,
  isCapturablePageUrl,
  revealPageCapture,
  runPageCapture,
  uploadPageCapture,
  type CaptureDownload,
} from '../../shared/desktop/pageCapture'

type Props = {
  onImported: () => void
}

export function PageCaptureForm({ onImported }: Props) {
  const queryClient = useQueryClient()
  const [url, setUrl] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [download, setDownload] = useState<CaptureDownload | null>(null)
  const [imported, setImported] = useState(false)

  if (!canCapturePages()) return null

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (!isCapturablePageUrl(url) || busy) return
    void (async () => {
      setBusy(true)
      setError('')
      setDownload(null)
      setImported(false)
      try {
        const result = await runPageCapture(url.trim())
        setDownload(result)
        if (result.share_import_url) {
          await createShareImport({ url: result.share_import_url, title: result.title }, crypto.randomUUID())
        } else {
          const filename = captureFileName(result.path)
          const ticket = await initCaptureUpload({
            filename,
            size: result.size ?? 0,
            title: result.title,
          })
          await uploadPageCapture(result.path, ticket)
          await completeCaptureUploadWhenReady(
            { destinationId: ticket.destinationId, filename: ticket.filename, title: ticket.title },
            crypto.randomUUID(),
          )
        }
        await queryClient.invalidateQueries({ queryKey: ['transfers'] })
        setImported(true)
        onImported()
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : '抓取失败。')
      } finally {
        setBusy(false)
      }
    })()
  }

  return (
    <section className="share-import page-capture" aria-labelledby="page-capture-heading">
      <div className="share-import-copy">
        <h2 id="page-capture-heading">网页抓取</h2>
        <p>贴视频所在播放页的地址。桌面打开该页、嗅探明文地址后入库。加密分片或登录墙会失败。</p>
      </div>
      <form className="page-capture-form" onSubmit={handleSubmit}>
        <label>
          <span className="sr-only">网页地址</span>
          <input
            autoComplete="off"
            maxLength={8192}
            name="page-url"
            onChange={(event) => setUrl(event.target.value)}
            placeholder="贴播放页地址"
            type="text"
            value={url}
          />
        </label>
        <button disabled={!isCapturablePageUrl(url) || busy} type="submit">
          <Radar size={16} />
          {busy ? '正在抓取…' : '开始抓取'}
        </button>
      </form>
          {busy ? <p className="page-capture-empty" role="status">正在打开页面并等待明文地址…</p> : null}
      {error ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{error}</span></div> : null}
      {imported ? <p className="share-import-success" role="status">已加入转存队列</p> : null}
      {download ? (
        <p className="share-import-success page-capture-saved" role="status">
          已存到本机
          <button className="page-capture-reveal" onClick={() => void revealPageCapture(download.path)} type="button">
            <FolderOpen size={16} />
            打开所在文件夹
          </button>
        </p>
      ) : null}
    </section>
  )
}
