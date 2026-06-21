import type {
  ApiProfile,
  StoredMediaFile,
  VideoTemplateId,
  VideoInputCapability,
  VideoModelCapability,
  VideoTaskStatus,
} from '../types'
import { buildApiUrl, readClientDevProxyConfig, shouldUseApiProxy } from './devProxy'
import { getApiErrorMessage } from './imageApiShared'
import { getDefaultVideoTemplateId, getVideoTemplate } from './videoCapabilities'

export interface RefreshVideoModelsResult {
  models: string[]
}

export interface SubmitVideoTaskOptions {
  profile: ApiProfile
  templateId: VideoTemplateId
  modelId: string
  prompt: string
  capability: VideoModelCapability
  durationSeconds: number | null
  resolution: string
  imageFiles: StoredMediaFile[]
  videoFiles: StoredMediaFile[]
  audioFiles: StoredMediaFile[]
  imageUrl?: string
}

export interface VideoTaskSnapshot {
  remoteTaskId: string | null
  remoteStatus: string | null
  resultUrl: string | null
  status: VideoTaskStatus
  error: string | null
  rawResponsePayload?: string
}

const SUCCESS_REMOTE_STATUSES = new Set(['success', 'succeeded', 'completed', 'done', 'finished', 'ok'])
const FAILURE_REMOTE_STATUSES = new Set(['failed', 'failure', 'error', 'cancelled', 'canceled', 'rejected', 'expired'])

function createHeaders(profile: ApiProfile): Record<string, string> {
  return { Authorization: `Bearer ${profile.apiKey}` }
}

function createAbortController(profile: ApiProfile) {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), Math.max(1, profile.timeout) * 1000)
  return { controller, timeoutId }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function getByPath(source: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((current, key) => {
    if (!key) return current
    if (!current || typeof current !== 'object') return undefined
    return (current as Record<string, unknown>)[key]
  }, source)
}

function getFirstString(source: unknown, paths: string[]): string | null {
  for (const path of paths) {
    const value = getByPath(source, path)
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return null
}

function parseRemoteStatus(status: string | null, resultUrl: string | null, error: string | null): VideoTaskStatus {
  const normalized = status?.trim().toLowerCase() ?? ''
  if (normalized && FAILURE_REMOTE_STATUSES.has(normalized)) return 'error'
  if (error) return 'error'
  if (resultUrl) return 'done'
  if (normalized && SUCCESS_REMOTE_STATUSES.has(normalized)) return 'done'
  return normalized === 'queued' ? 'queued' : 'running'
}

function serializeRawPayload(payload: unknown) {
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
}

async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer())
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(binary)
}

function normalizeVideoTaskPayload(payload: unknown): VideoTaskSnapshot {
  const remoteTaskId = getFirstString(payload, [
    'id',
    'video_id',
    'task_id',
    'data.id',
    'data.video_id',
    'data.task_id',
    'data.data.id',
    'data.data.video_id',
    'data.data.task_id',
  ])
  const remoteStatus = getFirstString(payload, ['status', 'data.status', 'data.data.status'])
  const resultUrl = getFirstString(payload, [
    'url',
    'video_url',
    'result_url',
    'data.url',
    'data.video_url',
    'data.result_url',
    'data.data.url',
    'data.data.video_url',
    'data.data.result_url',
  ])
  const error = getFirstString(payload, [
    'error.message',
    'error',
    'message',
    'fail_reason',
    'data.error.message',
    'data.error',
    'data.message',
    'data.fail_reason',
    'data.data.error.message',
    'data.data.error',
    'data.data.message',
    'data.data.fail_reason',
  ])
  const status = parseRemoteStatus(remoteStatus, resultUrl, error)
  const rawResponsePayload = serializeRawPayload(payload)

  if (!remoteTaskId && !resultUrl) {
    return {
      remoteTaskId: null,
      remoteStatus,
      resultUrl,
      status: 'error',
      error: error ?? '无法从视频接口响应中解析任务 ID 或结果 URL',
      rawResponsePayload,
    }
  }

  return {
    remoteTaskId,
    remoteStatus,
    resultUrl,
    status,
    error: status === 'error' ? error ?? '视频任务失败' : null,
    rawResponsePayload,
  }
}

function normalizeModelsPayload(payload: unknown): string[] {
  if (!isRecord(payload) || !Array.isArray(payload.data)) {
    throw new Error('当前配置返回的 /v1/models 结果不受支持')
  }
  const models = payload.data
    .map((item) => isRecord(item) && typeof item.id === 'string' ? item.id.trim() : '')
    .filter(Boolean)
  if (models.length === 0) throw new Error('当前配置返回的 /v1/models 中没有可用模型')
  return Array.from(new Set(models)).sort((a, b) => a.localeCompare(b))
}

function assertCount(kindLabel: string, count: number, capability: VideoInputCapability) {
  if (count < capability.min) throw new Error(`${kindLabel}数量不足，至少需要 ${capability.min} 个`)
  if (count > capability.max) throw new Error(`${kindLabel}数量超出限制，最多支持 ${capability.max} 个`)
}

function assertTransport(kindLabel: string, files: StoredMediaFile[], capability: VideoInputCapability) {
  if (files.length > 0 && capability.transport !== 'multipart-file' && capability.transport !== 'base64-json') {
    throw new Error(`${kindLabel}仅支持 ${capability.transport === 'url' ? 'URL' : 'metadata'} 方式传入，当前模型不支持本地上传`)
  }
}

function parseResolution(resolution: string) {
  const trimmed = resolution.trim()
  if (!trimmed) return null
  const match = trimmed.match(/^(\d+)\s*[xX]\s*(\d+)$/)
  if (!match) throw new Error('分辨率格式无效，需为 1280x720 这类宽x高格式')
  return { width: Number(match[1]), height: Number(match[2]) }
}

function getGreatestCommonDivisor(a: number, b: number): number {
  let x = Math.abs(a)
  let y = Math.abs(b)
  while (y) {
    const next = x % y
    x = y
    y = next
  }
  return x || 1
}

function formatAspectRatio(width: number, height: number) {
  const divisor = getGreatestCommonDivisor(width, height)
  return `${Math.trunc(width / divisor)}:${Math.trunc(height / divisor)}`
}

function buildMetadataWithResolution(options: SubmitVideoTaskOptions, resolution: ReturnType<typeof parseResolution>) {
  return {
    ...(options.capability.metadataDefaults ?? {}),
    ...(resolution ? { aspect_ratio: formatAspectRatio(resolution.width, resolution.height) } : {}),
  }
}

function assertCapability(options: SubmitVideoTaskOptions) {
  const { modelId, capability, durationSeconds, resolution, imageFiles, videoFiles, audioFiles } = options
  if (!modelId.trim()) throw new Error('请选择视频模型')
  if (capability.durationOptions.length > 0 && durationSeconds != null && !capability.durationOptions.includes(durationSeconds)) {
    throw new Error('当前模型不支持所选时长')
  }
  if (capability.resolutionOptions.length > 0 && resolution.trim() && !capability.resolutionOptions.includes(resolution.trim())) {
    throw new Error('当前模型不支持所选分辨率')
  }
  assertCount('图片', imageFiles.length, capability.imageInput)
  assertCount('视频', videoFiles.length, capability.videoInput)
  assertCount('音频', audioFiles.length, capability.audioInput)
  assertTransport('图片', imageFiles, capability.imageInput)
  assertTransport('视频', videoFiles, capability.videoInput)
  assertTransport('音频', audioFiles, capability.audioInput)
}

async function readImageInputs(options: SubmitVideoTaskOptions): Promise<string[]> {
  const values: string[] = []
  const imageUrl = options.imageUrl?.trim()
  if (imageUrl) values.push(imageUrl)
  for (const file of options.imageFiles) {
    values.push(await blobToBase64(file.blob))
  }
  return values
}

async function readVideoInputs(options: SubmitVideoTaskOptions): Promise<string[]> {
  const values: string[] = []
  for (const file of options.videoFiles) {
    values.push(await blobToBase64(file.blob))
  }
  return values
}

async function buildGrokJsonBody(options: SubmitVideoTaskOptions): Promise<Record<string, unknown>> {
  const body: Record<string, unknown> = {
    model: options.modelId,
    prompt: options.prompt,
  }
  if (options.durationSeconds != null) body.duration = options.durationSeconds
  const resolution = parseResolution(options.resolution)
  if (resolution) {
    body.width = resolution.width
    body.height = resolution.height
  }
  const imageValue = (await readImageInputs(options))[0] ?? ''
  if (imageValue) body.image = imageValue
  body.metadata = buildMetadataWithResolution(options, resolution)
  return body
}

async function buildOmniJsonBody(options: SubmitVideoTaskOptions): Promise<Record<string, unknown>> {
  const body: Record<string, unknown> = {
    model: options.modelId,
    prompt: options.prompt,
  }
  if (options.durationSeconds != null) body.duration = options.durationSeconds
  const resolution = parseResolution(options.resolution)
  if (resolution) {
    body.width = resolution.width
    body.height = resolution.height
  }
  const images = await readImageInputs(options)
  const videos = await readVideoInputs(options)
  body.metadata = {
    ...buildMetadataWithResolution(options, resolution),
    ...(images.length > 0 ? { images } : {}),
    ...(videos.length > 0 ? { video: videos[0] } : {}),
  }
  return body
}

async function buildVideoJsonBody(options: SubmitVideoTaskOptions): Promise<Record<string, unknown>> {
  return options.templateId === 'omni' ? buildOmniJsonBody(options) : buildGrokJsonBody(options)
}

function getTemplate(profile: ApiProfile) {
  return getVideoTemplate(profile.videoConfig?.templateId ?? getDefaultVideoTemplateId())
}

export async function refreshVideoModels(profile: ApiProfile): Promise<RefreshVideoModelsResult> {
  const proxyConfig = readClientDevProxyConfig()
  const useApiProxy = shouldUseApiProxy(profile.apiProxy, proxyConfig)
  const { controller, timeoutId } = createAbortController(profile)
  try {
    const response = await fetch(buildApiUrl(profile.baseUrl, 'models', proxyConfig, useApiProxy), {
      method: 'GET',
      headers: createHeaders(profile),
      cache: 'no-store',
      signal: controller.signal,
    })
    if (!response.ok) throw new Error(await getApiErrorMessage(response))
    const payload = await response.json()
    return { models: normalizeModelsPayload(payload) }
  } finally {
    clearTimeout(timeoutId)
  }
}

export async function submitVideoTask(options: SubmitVideoTaskOptions): Promise<VideoTaskSnapshot> {
  assertCapability(options)
  const { profile, capability } = options
  const template = getVideoTemplate(options.templateId)
  if (!template) throw new Error('当前视频模板不可用')

  const proxyConfig = readClientDevProxyConfig()
  const useApiProxy = shouldUseApiProxy(profile.apiProxy, proxyConfig)
  const { controller, timeoutId } = createAbortController(profile)

  try {
    const response = await fetch(buildApiUrl(profile.baseUrl, template.submitPath, proxyConfig, useApiProxy), {
      method: 'POST',
      headers: {
        ...createHeaders(profile),
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
      body: JSON.stringify(await buildVideoJsonBody(options)),
      signal: controller.signal,
    })
    if (!response.ok) throw new Error(await getApiErrorMessage(response))
    const payload = await response.json()
    return normalizeVideoTaskPayload(payload)
  } finally {
    clearTimeout(timeoutId)
  }
}

export async function getVideoTaskSnapshot(profile: ApiProfile, templateId: VideoTemplateId, remoteTaskId: string): Promise<VideoTaskSnapshot> {
  const template = getVideoTemplate(templateId) ?? getTemplate(profile)
  if (!template) throw new Error('当前视频模板不可用')
  const proxyConfig = readClientDevProxyConfig()
  const useApiProxy = shouldUseApiProxy(profile.apiProxy, proxyConfig)
  const { controller, timeoutId } = createAbortController(profile)
  try {
    const response = await fetch(buildApiUrl(profile.baseUrl, `${template.taskPathPrefix}/${encodeURIComponent(remoteTaskId)}`, proxyConfig, useApiProxy), {
      method: 'GET',
      headers: createHeaders(profile),
      cache: 'no-store',
      signal: controller.signal,
    })
    if (!response.ok) throw new Error(await getApiErrorMessage(response))
    const payload = await response.json()
    return normalizeVideoTaskPayload(payload)
  } finally {
    clearTimeout(timeoutId)
  }
}

export const __videoApiTestUtils = {
  normalizeModelsPayload,
  normalizeVideoTaskPayload,
  buildGrokJsonBody,
  buildOmniJsonBody,
  buildVideoJsonBody,
  formatAspectRatio,
}
