import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, Download, FolderOpen, Radar } from 'lucide-react'
import { createShareImport } from '../../shared/api/mediaHub'
import {
  canCapturePages,
  captureItemLabel,
  captureKindLabel,
  downloadPageCapture,
  isCapturablePageUrl,
  listPageCapture,
  openPageCapture,
  revealPageCapture,
  type CaptureDownload,
  type CaptureItem,
} from '../../shared/desktop/pageCapture'

type Props = {
  onImported: () => void
}

export function PageCaptureForm({ onImported }: Props) {
  const queryClient = useQueryClient()
  const [url, setUrl] = useState('')
  const [opened, setOpened] = useState(false)
  const [items, setItems] = useState<CaptureItem[]>([])
  const [listError, setListError] = useState('')
  const [download, setDownload] = useState<CaptureDownload | null>(null)
  const [busyUrl, setBusyUrl] = useState('')

  const openCapture = useMutation({
    mutationFn: () => openPageCapture(url.trim()),
    onSuccess: () => {
      setOpened(true)
      setListError('')
      setDownload(null)
    },
  })

  const importFile = useMutation({
    mutationFn: (mediaUrl: string) => createShareImport({ url: mediaUrl }, crypto.randomUUID()),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['transfers'] })
      onImported()
    },
  })

  useEffect(() => {
    if (!opened) return undefined
    let cancelled = false
    const tick = async () => {
      try {
        const snapshot = await listPageCapture()
        if (!cancelled) {
          setItems(snapshot.items)
          setListError('')
        }
      } catch (cause) {
        if (!cancelled) {
          setListError(cause instanceof Error ? cause.message : '无法读取抓取结果。')
        }
      }
    }
    void tick()
    const timer = window.setInterval(() => { void tick() }, 1500)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [opened])

  if (!canCapturePages()) return null

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (!isCapturablePageUrl(url) || openCapture.isPending) return
    openCapture.mutate()
  }

  const handleDownload = async (item: CaptureItem) => {
    setBusyUrl(item.url)
    setDownload(null)
    try {
      setDownload(await downloadPageCapture(item.url))
    } catch (cause) {
      setListError(cause instanceof Error ? cause.message : '下载失败。')
    } finally {
      setBusyUrl('')
    }
  }

  return (
    <section className="share-import page-capture" aria-labelledby="page-capture-heading">
      <div className="share-import-copy">
        <h2 id="page-capture-heading">网页抓取</h2>
        <p>桌面壳单独开页，拦明文 m3u8 / mp4。加密分片会跳过。直链可送进成人库；HLS 拼到本机下载目录，115 离线吃不下播放列表。</p>
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
        <button disabled={!isCapturablePageUrl(url) || openCapture.isPending} type="submit">
          <Radar size={16} />
          {openCapture.isPending ? '正在打开…' : '打开并抓取'}
        </button>
      </form>
      {openCapture.isError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{openCapture.error.message}</span></div> : null}
      {opened ? (
        <div className="page-capture-results">
          {items.length === 0 && !listError ? <p className="page-capture-empty">窗口里播一下视频，明文地址会出现在这里。</p> : null}
          {items.map((item) => (
            <div className="page-capture-row" key={item.url}>
              <div>
                <strong>{captureKindLabel(item.kind)}</strong>
                <span title={item.url}>{captureItemLabel(item)}</span>
              </div>
              <div className="page-capture-row-actions">
                <button disabled={busyUrl === item.url} onClick={() => void handleDownload(item)} type="button">
                  <Download size={16} />
                  {busyUrl === item.url ? '正在下载…' : '本机下载'}
                </button>
                {item.kind === 'file' ? (
                  <button disabled={importFile.isPending} onClick={() => importFile.mutate(item.url)} type="button">
                    送进成人库
                  </button>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      ) : null}
      {listError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{listError}</span></div> : null}
      {importFile.isError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{importFile.error.message}</span></div> : null}
      {importFile.isSuccess ? <p className="share-import-success" role="status">直链已加入转存队列</p> : null}
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
