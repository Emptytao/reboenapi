import { useEffect, useMemo, useState } from 'react'
import type { VideoTaskRecord } from '../types'
import { refreshVideoTaskResult, removeVideoTaskRecord, useStore } from '../store'
import { useCloseOnEscape } from '../hooks/useCloseOnEscape'
import { usePreventBackgroundScroll } from '../hooks/usePreventBackgroundScroll'
import { copyTextToClipboard, getClipboardFailureMessage } from '../lib/clipboard'
import { TrashIcon } from './icons'

function gcd(a: number, b: number): number {
  let x = Math.abs(a)
  let y = Math.abs(b)
  while (y) {
    const next = x % y
    x = y
    y = next
  }
  return x || 1
}

function parseResolution(resolution?: string) {
  const match = resolution?.trim().match(/^(\d+)\s*[xX]\s*(\d+)$/)
  if (!match) return null
  const width = Number(match[1])
  const height = Number(match[2])
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null
  return { width, height }
}

function formatResolutionLabel(resolution?: string) {
  const parsed = parseResolution(resolution)
  return parsed ? `${parsed.width}×${parsed.height}` : ''
}

function formatRatioLabel(resolution?: string) {
  const parsed = parseResolution(resolution)
  if (!parsed) return ''
  const divisor = gcd(parsed.width, parsed.height)
  return `${Math.trunc(parsed.width / divisor)}:${Math.trunc(parsed.height / divisor)}`
}

function formatDuration(start: number, elapsed: number | null, running: boolean) {
  const seconds = Math.floor((running ? Date.now() - start : elapsed ?? 0) / 1000)
  const mm = String(Math.floor(seconds / 60)).padStart(2, '0')
  const ss = String(seconds % 60).padStart(2, '0')
  return `${mm}:${ss}`
}

function formatTime(ts: number | null) {
  if (!ts) return ''
  return new Date(ts).toLocaleString('zh-CN')
}

function getVideoDurationLabel(task: VideoTaskRecord) {
  if (task.params.durationSeconds != null) return `${task.params.durationSeconds}s`
  return '未设置'
}

function DetailParamBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 overflow-hidden rounded-lg bg-gray-50 px-3 py-2 dark:bg-white/[0.03]">
      <span className="text-gray-400 dark:text-gray-500">{label}</span>
      <br />
      <div className="mask-edge-r mt-0.5 overflow-x-auto whitespace-nowrap pr-2 hide-scrollbar">
        <span className="font-medium text-gray-700 dark:text-gray-300">{value || '未知'}</span>
      </div>
    </div>
  )
}

function VideoPreviewModal({ task, onClose }: { task: VideoTaskRecord; onClose: () => void }) {
  const [previewError, setPreviewError] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const showToast = useStore((s) => s.showToast)
  const running = task.status === 'queued' || task.status === 'running'
  const duration = formatDuration(task.createdAt, task.elapsed, running)
  const ratioLabel = formatRatioLabel(task.params.resolution)
  const resolutionLabel = formatResolutionLabel(task.params.resolution)
  const sourceLabel = task.apiProfileName || task.apiProvider || '未知'

  useCloseOnEscape(true, onClose)
  usePreventBackgroundScroll(true)

  useEffect(() => {
    setPreviewError(false)
  }, [task.resultUrl])

  if (!task.resultUrl) return null

  const handleCopyPrompt = async () => {
    if (!task.prompt) return
    try {
      await copyTextToClipboard(task.prompt)
      showToast('提示词已复制', 'success')
    } catch (err) {
      showToast(getClipboardFailureMessage('复制提示词失败', err), 'error')
    }
  }

  const handleRefreshResult = async () => {
    if (!task.remoteTaskId || refreshing) return
    setRefreshing(true)
    try {
      await refreshVideoTaskResult(task.id)
    } finally {
      setRefreshing(false)
    }
  }

  const handleDelete = () => {
    onClose()
    void removeVideoTaskRecord(task)
  }

  return (
    <div
      data-no-drag-select
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={onClose}
    >
      <div className="absolute inset-0 bg-black/20 backdrop-blur-md animate-overlay-in dark:bg-black/40" />
      <div
        className="relative z-10 flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-3xl border border-white/50 bg-white/90 shadow-[0_8px_40px_rgb(0,0,0,0.12)] ring-1 ring-black/5 backdrop-blur-xl animate-modal-in md:flex-row dark:border-white/[0.08] dark:bg-gray-900/90 dark:shadow-[0_8px_40px_rgb(0,0,0,0.4)] dark:ring-white/10"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex h-14 items-center justify-end px-4 md:hidden">
          <button
            type="button"
            onClick={onClose}
            className="rounded-full p-1 text-gray-400 transition hover:bg-gray-100 dark:hover:bg-white/[0.06]"
            aria-label="关闭"
          >
            <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="relative flex h-64 w-full flex-shrink-0 items-center justify-center bg-gray-100 md:h-auto md:min-h-[16rem] md:w-1/2 dark:bg-black/20">
          {task.resultUrl && (
            <div data-selectable-text className="absolute left-4 top-[15px] z-20 flex items-center gap-1.5">
              {ratioLabel && (
                <span className="rounded bg-black/50 px-2 py-0.5 font-mono text-xs text-white backdrop-blur-sm">
                  {ratioLabel}
                </span>
              )}
              {resolutionLabel && (
                <span className="rounded bg-black/50 px-2 py-0.5 text-xs font-medium text-white/90 backdrop-blur-sm">
                  {resolutionLabel}
                </span>
              )}
              {!ratioLabel && !resolutionLabel && (
                <span className="flex items-center gap-1 rounded bg-black/50 px-2 py-0.5 font-mono text-xs text-white backdrop-blur-sm">
                  <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  {duration}
                </span>
              )}
            </div>
          )}
          {task.resultUrl && (
            <div className="absolute right-3 top-[15px] z-20 flex items-center gap-1.5">
              <a
                href={task.resultUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-center rounded bg-black/50 px-1.5 py-0.5 text-white backdrop-blur-sm transition hover:bg-black/70 focus:outline-none focus:ring-1 focus:ring-white/50"
                aria-label="打开视频"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 3h7m0 0v7m0-7L10 14M5 5h5M5 5v14h14v-5" />
                </svg>
              </a>
            </div>
          )}
          {task.resultUrl && !previewError ? (
            <video
              src={task.resultUrl}
              controls
              playsInline
              autoPlay
              className="max-h-[calc(100%-2rem)] max-w-[calc(100%-2rem)] object-contain"
              preload="auto"
              onError={() => setPreviewError(true)}
            />
          ) : (
            <div className="w-full max-w-md px-4 text-center">
              <svg className="mx-auto mb-2 h-10 w-10 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <p className="text-sm text-red-500">浏览器无法直接预览视频</p>
              <a
                href={task.resultUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-3 inline-flex rounded-full border border-blue-200/80 bg-white/80 px-3 py-1.5 text-sm text-blue-500 transition hover:bg-blue-50 dark:border-blue-400/20 dark:bg-white/[0.04] dark:hover:bg-blue-500/10"
              >
                打开视频
              </a>
            </div>
          )}
        </div>

        <div className="flex w-full flex-col overflow-y-auto overscroll-contain p-5 md:w-1/2">
          <button
            type="button"
            onClick={onClose}
            className="absolute right-3 top-3 z-10 hidden rounded-full p-1 text-gray-400 transition hover:bg-gray-100 md:block dark:hover:bg-white/[0.06]"
            aria-label="关闭"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>

          <div data-selectable-text className="flex-1">
            <div className="mb-2 flex items-center gap-1.5">
              <h3 className="text-xs font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
                输入内容
              </h3>
              {task.prompt && (
                <button
                  type="button"
                  onClick={handleCopyPrompt}
                  className="rounded p-1 text-gray-400 transition hover:bg-gray-100 dark:text-gray-500 dark:hover:bg-white/[0.06]"
                  title="复制提示词"
                >
                  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </button>
              )}
            </div>
            <p className="mb-4 whitespace-pre-wrap text-sm leading-relaxed text-gray-700 dark:text-gray-300">
              {task.prompt || '(无提示词)'}
            </p>

            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
              参数配置
            </h3>
            <div className="mb-2 min-w-0 overflow-hidden rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-white/[0.03]">
              <span className="text-gray-400 dark:text-gray-500">来源</span>
              <br />
              <div className="mask-edge-r mt-0.5 overflow-x-auto whitespace-nowrap pr-2 hide-scrollbar">
                <span className="font-medium text-gray-700 dark:text-gray-200">{sourceLabel}</span>
                <span className="text-gray-400 dark:text-gray-500"> · {task.templateId} · {task.apiModel || '未知'}</span>
              </div>
            </div>
            <div className="mb-4 grid min-w-0 grid-cols-2 gap-2 text-xs">
              <DetailParamBox label="尺寸" value={resolutionLabel || task.params.resolution || '未设置'} />
              <DetailParamBox label="时长" value={getVideoDurationLabel(task)} />
              <DetailParamBox label="模板" value={task.templateId} />
              <DetailParamBox label="状态" value={task.remoteStatus || task.status} />
              <DetailParamBox label="任务ID" value={task.remoteTaskId || '无'} />
              <DetailParamBox label="音频" value={task.params.metadata?.audio == null ? '未知' : String(task.params.metadata.audio)} />
            </div>

            <div className="mb-4 text-xs text-gray-400 dark:text-gray-500">
              <span>创建于 {formatTime(task.createdAt)}</span>
              {duration && <span> · 耗时 {duration}</span>}
            </div>
          </div>

          <div className="grid grid-cols-4 gap-2 border-t border-gray-100 pt-4 sm:flex dark:border-white/[0.08]">
            <button
              type="button"
              onClick={() => { void handleRefreshResult() }}
              disabled={!task.remoteTaskId || refreshing}
              className="col-span-2 flex items-center justify-center gap-1.5 rounded-xl bg-blue-50 px-3 py-2 text-sm font-medium text-blue-600 transition hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-40 sm:flex-1 dark:bg-blue-500/10 dark:text-blue-400 dark:hover:bg-blue-500/20"
            >
              <svg className={`h-4 w-4 flex-shrink-0 ${refreshing ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v6h6M20 20v-6h-6M5.64 17.66A8 8 0 0018.36 6.34M18.36 6.34A8 8 0 005.64 17.66" />
              </svg>
              重新获取
            </button>
            {task.resultUrl && (
              <a
                href={task.resultUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="col-span-2 flex items-center justify-center gap-1.5 rounded-xl bg-green-50 px-3 py-2 text-sm font-medium text-green-600 transition hover:bg-green-100 sm:flex-1 dark:bg-green-500/10 dark:text-green-400 dark:hover:bg-green-500/20"
              >
                <svg className="h-4 w-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 3h7m0 0v7m0-7L10 14M5 5h5M5 5v14h14v-5" />
                </svg>
                打开视频
              </a>
            )}
            <button
              type="button"
              onClick={handleDelete}
              className="col-span-3 flex items-center justify-center gap-1.5 rounded-xl bg-red-50 px-3 py-2 text-sm font-medium text-red-600 transition hover:bg-red-100 sm:flex-1 dark:bg-red-500/10 dark:text-red-400 dark:hover:bg-red-500/20"
            >
              <TrashIcon className="h-4 w-4 flex-shrink-0" />
              删除任务
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function VideoTaskCard({ task, onOpenPreview }: { task: VideoTaskRecord; onOpenPreview: (taskId: string) => void }) {
  const [now, setNow] = useState(Date.now())
  const [refreshing, setRefreshing] = useState(false)
  const [thumbError, setThumbError] = useState(false)
  const running = task.status === 'queued' || task.status === 'running'

  useEffect(() => {
    setThumbError(false)
  }, [task.resultUrl])

  useEffect(() => {
    if (!running) return
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    setNow(Date.now())
    return () => window.clearInterval(id)
  }, [running])

  const duration = formatDuration(task.createdAt, task.elapsed, running)
  const canRefreshResult = Boolean(task.remoteTaskId)
  const ratioLabel = formatRatioLabel(task.params.resolution)
  const resolutionLabel = formatResolutionLabel(task.params.resolution)

  const handleRefreshResult = async () => {
    if (!canRefreshResult || refreshing) return
    setRefreshing(true)
    try {
      await refreshVideoTaskResult(task.id)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div
      className={`relative overflow-hidden rounded-xl border bg-white touch-pan-y transition-[box-shadow,border-color,background-color,transform] hover:shadow-lg dark:bg-gray-900 dark:hover:bg-gray-800/80 ${
        running
          ? 'border-blue-400 generating'
          : task.status === 'error'
            ? 'border-red-200 dark:border-red-500/30'
            : 'border-gray-200 dark:border-white/[0.08] hover:border-gray-300 dark:hover:border-white/[0.18]'
      } ${task.resultUrl ? 'cursor-pointer' : 'cursor-default'}`}
      onClick={() => task.resultUrl && onOpenPreview(task.id)}
    >
      <div className="flex h-40">
        <div className="relative flex h-full w-40 min-w-[10rem] flex-shrink-0 items-center justify-center overflow-hidden bg-gray-100 dark:bg-black/20">
          {running && (
            <div className="flex flex-col items-center gap-2">
              <svg className="h-8 w-8 animate-spin text-blue-400" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              <span className="text-xs text-gray-400 dark:text-gray-500">生成中...</span>
            </div>
          )}
          {task.status === 'error' && (
            <div className="flex flex-col items-center gap-1 px-2">
              <svg className="h-7 w-7 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span className="text-center text-xs leading-tight text-red-400">失败</span>
            </div>
          )}
          {task.status === 'done' && task.resultUrl && !thumbError && (
            <video
              src={task.resultUrl}
              muted
              playsInline
              className="h-full w-full object-cover"
              preload="metadata"
              onLoadedMetadata={(e) => {
                const video = e.currentTarget
                if (video.readyState >= 1) {
                  try {
                    video.currentTime = 0.1
                  } catch {
                    // ignore
                  }
                }
              }}
              onError={() => setThumbError(true)}
            />
          )}
          {task.status === 'done' && (!task.resultUrl || thumbError) && (
            <svg className="h-8 w-8 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
          )}
          {task.status === 'done' && task.resultUrl && (
            <div className="absolute inset-0 flex items-center justify-center bg-black/10 opacity-0 transition-opacity hover:opacity-100">
              <span className="flex h-10 w-10 items-center justify-center rounded-full bg-black/45 text-white/90 backdrop-blur-sm">
                <svg className="ml-0.5 h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
              </span>
            </div>
          )}
          <div className="absolute left-1.5 top-1.5 flex items-center gap-1">
            {running || task.status !== 'done' || (!ratioLabel && !resolutionLabel) ? (
              <span className="flex items-center gap-1 rounded bg-black/50 px-1.5 py-0.5 font-mono text-[10px] text-white backdrop-blur-sm sm:text-xs">
                <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                {now && duration}
              </span>
            ) : (
              <>
                {ratioLabel && (
                  <span className="rounded bg-black/50 px-1.5 py-0.5 font-mono text-[10px] text-white backdrop-blur-sm sm:text-xs">
                    {ratioLabel}
                  </span>
                )}
                {resolutionLabel && (
                  <span className="rounded bg-black/50 px-1.5 py-0.5 text-[10px] font-medium text-white/90 backdrop-blur-sm sm:text-xs">
                    {resolutionLabel}
                  </span>
                )}
              </>
            )}
          </div>
        </div>

        <div className="flex min-w-0 flex-1 flex-col p-3">
          <div className="mb-2 min-h-0 flex-1 overflow-hidden">
            <p className="line-clamp-3 text-sm leading-relaxed text-gray-700 dark:text-gray-300">
              {task.prompt || '(无提示词)'}
            </p>
            {task.error && <p className="mt-2 line-clamp-2 text-xs text-red-500">{task.error}</p>}
          </div>
          <div className="mt-auto flex flex-col gap-1.5">
            <div data-tag-scroll-area className="mask-edge-r flex min-w-0 gap-1.5 overflow-x-auto whitespace-nowrap pr-2 pt-0.5 hide-scrollbar">
              <span className="flex flex-shrink-0 items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-white/[0.04] dark:text-gray-300">
                <svg className="h-3 w-3 flex-shrink-0 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 9l-3 3 3 3m4-6l3 3-3 3" />
                </svg>
                视频
              </span>
            </div>
            <div
              data-tag-scroll-area
              className="ml-auto flex max-w-full flex-shrink-0 items-center gap-1 overflow-x-auto pr-2 hide-scrollbar"
              onClick={(e) => e.stopPropagation()}
              onTouchStart={(e) => e.stopPropagation()}
              onTouchMove={(e) => e.stopPropagation()}
              onTouchEnd={(e) => e.stopPropagation()}
              onTouchCancel={(e) => e.stopPropagation()}
            >
              <button
                type="button"
                onClick={() => { void handleRefreshResult() }}
                disabled={!canRefreshResult || refreshing}
                className="rounded-md p-1.5 text-gray-400 transition hover:bg-blue-50 hover:text-blue-500 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-blue-950/30"
                aria-label="重新获取视频结果"
                title="重新获取结果"
              >
                <svg className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v6h6M20 20v-6h-6M5.64 17.66A8 8 0 0018.36 6.34M18.36 6.34A8 8 0 005.64 17.66" />
                </svg>
              </button>
              {task.resultUrl && (
                <a
                  href={task.resultUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="rounded-md p-1.5 text-gray-400 transition hover:bg-blue-50 hover:text-blue-500 dark:hover:bg-blue-950/30"
                  aria-label="打开结果视频"
                >
                  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 3h7m0 0v7m0-7L10 14M5 5h5M5 5v14h14v-5" />
                  </svg>
                </a>
              )}
              <button
                type="button"
                onClick={() => { void removeVideoTaskRecord(task) }}
                className="rounded-md p-1.5 text-gray-400 transition hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/30"
                aria-label="删除视频任务"
                title="删除任务"
              >
                <TrashIcon className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default function VideoWorkspace() {
  const videoTasks = useStore((s) => s.videoTasks)
  const [previewTaskId, setPreviewTaskId] = useState<string | null>(null)
  const sortedTasks = useMemo(() => [...videoTasks].sort((a, b) => b.createdAt - a.createdAt), [videoTasks])
  const previewTask = sortedTasks.find((task) => task.id === previewTaskId) ?? null

  useEffect(() => {
    if (previewTaskId && !previewTask) setPreviewTaskId(null)
  }, [previewTask, previewTaskId])

  return (
    <main data-home-main data-drag-select-surface className="pb-48">
      <div className="safe-area-x mx-auto max-w-7xl">
        {sortedTasks.length === 0 ? (
          <div className="py-20 text-center text-gray-400 dark:text-gray-500">
            <svg className="mx-auto mb-4 h-16 w-16 text-gray-200 dark:text-gray-700" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            <p className="text-sm">输入提示词开始生成视频</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 pb-10 sm:grid-cols-2 lg:grid-cols-3">
            {sortedTasks.map((task) => (
              <div key={task.id} className="task-card-wrapper" data-task-id={task.id}>
                <VideoTaskCard task={task} onOpenPreview={setPreviewTaskId} />
              </div>
            ))}
          </div>
        )}
      </div>
      {previewTask && (
        <VideoPreviewModal task={previewTask} onClose={() => setPreviewTaskId(null)} />
      )}
    </main>
  )
}
