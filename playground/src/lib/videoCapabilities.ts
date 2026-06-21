import type {
  ApiProfile,
  ApiProfileVideoConfig,
  VideoEndpointFamily,
  VideoInputAssetKind,
  VideoInputCapability,
  VideoInputTransport,
  VideoModelCapability,
  VideoTemplateConfig,
  VideoTemplateDefinition,
  VideoTemplateId,
} from '../types'

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function normalizeStringArray(value: unknown, fallback: string[] = []): string[] {
  if (!Array.isArray(value)) return [...fallback]
  const items = value
    .filter((item): item is string => typeof item === 'string')
    .map((item) => item.trim())
    .filter(Boolean)
  return items.length ? Array.from(new Set(items)) : [...fallback]
}

function normalizeNumberArray(value: unknown, fallback: number[] = []): number[] {
  if (!Array.isArray(value)) return [...fallback]
  const items = value
    .map((item) => typeof item === 'number' ? item : Number(item))
    .filter((item) => Number.isFinite(item) && item > 0)
    .map((item) => Math.trunc(item))
  return items.length ? Array.from(new Set(items)).sort((a, b) => a - b) : [...fallback]
}

function normalizeTransport(value: unknown, fallback: VideoInputTransport): VideoInputTransport {
  return value === 'multipart-file' || value === 'base64-json' || value === 'url' || value === 'metadata' ? value : fallback
}

function normalizeMetadataDefaults(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) return {}
  return Object.fromEntries(Object.entries(value))
}

function normalizeTemplateId(value: unknown, fallback: VideoTemplateId = getDefaultVideoTemplateId()): VideoTemplateId {
  return typeof value === 'string' && getVideoTemplate(value) ? value as VideoTemplateId : fallback
}

export function createDefaultVideoInputCapability(overrides: Partial<VideoInputCapability> = {}): VideoInputCapability {
  return {
    min: 0,
    max: 0,
    transport: 'metadata',
    fieldNames: [],
    ...overrides,
  }
}

function createGrokTemplate(): VideoTemplateDefinition {
  return {
    id: 'grok',
    name: 'grok',
    submitPath: 'video/generations',
    taskPathPrefix: 'video/generations',
    imageInput: createDefaultVideoInputCapability({
      min: 0,
      max: 1,
      transport: 'base64-json',
      fieldNames: ['image'],
    }),
    videoInput: createDefaultVideoInputCapability({ min: 0, max: 0, transport: 'metadata' }),
    audioInput: createDefaultVideoInputCapability({ min: 0, max: 0, transport: 'metadata' }),
    defaultConfig: {
      durationOptions: [10],
      resolutionOptions: ['1280x720'],
      metadataDefaults: {
        resolution: '720p',
        aspect_ratio: '16:9',
        audio: true,
      },
    },
  }
}

function createOmniTemplate(): VideoTemplateDefinition {
  return {
    id: 'omni',
    name: 'omni',
    submitPath: 'video/generations',
    taskPathPrefix: 'video/generations',
    imageInput: createDefaultVideoInputCapability({
      min: 0,
      max: 7,
      transport: 'base64-json',
      fieldNames: ['metadata.images'],
    }),
    videoInput: createDefaultVideoInputCapability({
      min: 0,
      max: 1,
      transport: 'base64-json',
      fieldNames: ['metadata.video'],
    }),
    audioInput: createDefaultVideoInputCapability({ min: 0, max: 0, transport: 'metadata' }),
    defaultConfig: {
      durationOptions: [10],
      resolutionOptions: ['1920x1080'],
      metadataDefaults: {
        resolution: '1080p',
        aspect_ratio: '16:9',
        audio: false,
      },
    },
  }
}

export const VIDEO_TEMPLATES: Record<VideoTemplateId, VideoTemplateDefinition> = {
  grok: createGrokTemplate(),
  omni: createOmniTemplate(),
}

export function getDefaultVideoTemplateId(): VideoTemplateId {
  return 'grok'
}

export function getVideoTemplate(templateId?: string | null): VideoTemplateDefinition | null {
  return templateId === 'grok' || templateId === 'omni' ? VIDEO_TEMPLATES[templateId] : null
}

export function getDefaultVideoTemplateConfig(templateId: VideoTemplateId): VideoTemplateConfig {
  const template = VIDEO_TEMPLATES[templateId]
  return {
    durationOptions: [...template.defaultConfig.durationOptions],
    resolutionOptions: [...template.defaultConfig.resolutionOptions],
    metadataDefaults: { ...template.defaultConfig.metadataDefaults },
  }
}

export function normalizeVideoTemplateConfig(
  value: unknown,
  fallback: VideoTemplateConfig = getDefaultVideoTemplateConfig('grok'),
): VideoTemplateConfig {
  const record = isRecord(value) ? value : {}
  return {
    templateId: normalizeTemplateId(record.templateId, fallback.templateId ?? getDefaultVideoTemplateId()),
    durationOptions: normalizeNumberArray(record.durationOptions, fallback.durationOptions),
    resolutionOptions: normalizeStringArray(record.resolutionOptions, fallback.resolutionOptions),
    metadataDefaults: normalizeMetadataDefaults(record.metadataDefaults),
  }
}

export function normalizeApiProfileVideoConfig(value: unknown): ApiProfileVideoConfig | undefined {
  if (!isRecord(value)) return undefined
  const templateId = normalizeTemplateId(value.templateId)
  const defaults = getDefaultVideoTemplateConfig(templateId)
  const rawOverrides = isRecord(value.capabilityOverrides) ? value.capabilityOverrides : {}
  const capabilityOverrides = Object.fromEntries(
    Object.entries(rawOverrides)
      .map(([modelId, config]) => {
        const modelTemplateId = isRecord(config) ? normalizeTemplateId(config.templateId, templateId) : templateId
        return [modelId.trim(), normalizeVideoTemplateConfig(config, { ...getDefaultVideoTemplateConfig(modelTemplateId), templateId: modelTemplateId })] as const
      })
      .filter((entry): entry is [string, VideoTemplateConfig] => Boolean(entry[0])),
  )
  return {
    templateId,
    availableModels: normalizeStringArray(value.availableModels),
    capabilityOverrides,
  }
}

export function getVideoModelCapability(profile: ApiProfile, modelId: string): VideoModelCapability | null {
  const override = profile.videoConfig?.capabilityOverrides?.[modelId.trim()]
  const templateId = override?.templateId ?? profile.videoConfig?.templateId ?? getDefaultVideoTemplateId()
  const template = getVideoTemplate(templateId)
  if (!template || !modelId.trim()) return null
  const config = normalizeVideoTemplateConfig(override, { ...template.defaultConfig, templateId })
  return {
    endpointFamily: 'video-generations',
    imageInput: template.imageInput,
    videoInput: template.videoInput,
    audioInput: template.audioInput,
    durationOptions: [...config.durationOptions],
    resolutionOptions: [...config.resolutionOptions],
    resolutionEncoding: 'width-height',
    metadataDefaults: { ...template.defaultConfig.metadataDefaults, ...config.metadataDefaults },
  }
}

export function getVideoInputTransportLabel(capability: VideoModelCapability, kind: VideoInputAssetKind): string {
  const inputCapability = kind === 'image' ? capability.imageInput : kind === 'video' ? capability.videoInput : capability.audioInput
  if (inputCapability.transport === 'base64-json') return 'Base64(JSON)'
  if (inputCapability.transport === 'multipart-file') return '本地上传'
  if (inputCapability.transport === 'url') return '仅 URL'
  return '仅 metadata'
}

export function getVideoTemplateFromProfile(profile: ApiProfile) {
  return getVideoTemplate(profile.videoConfig?.templateId ?? getDefaultVideoTemplateId())
}

export function getVideoTemplateForModel(profile: ApiProfile, modelId: string) {
  const templateId = profile.videoConfig?.capabilityOverrides?.[modelId.trim()]?.templateId ?? profile.videoConfig?.templateId ?? getDefaultVideoTemplateId()
  return getVideoTemplate(templateId)
}

export function createVideoTemplateConfigPatch(templateId: VideoTemplateId, patch: Partial<ApiProfileVideoConfig>): ApiProfileVideoConfig {
  const defaults = getDefaultVideoTemplateConfig(templateId)
  const capabilityOverrides = patch.capabilityOverrides ?? {}
  return {
    templateId,
    availableModels: normalizeStringArray(patch.availableModels),
    capabilityOverrides: Object.fromEntries(
      Object.entries(capabilityOverrides).map(([modelId, config]) => {
        const modelTemplateId = config.templateId ?? templateId
        return [modelId.trim(), normalizeVideoTemplateConfig(config, { ...getDefaultVideoTemplateConfig(modelTemplateId), templateId: modelTemplateId })] as const
      }).filter((entry): entry is [string, VideoTemplateConfig] => Boolean(entry[0])),
    ),
  }
}
