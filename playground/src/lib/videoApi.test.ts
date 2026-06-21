import { afterEach, describe, expect, it, vi } from 'vitest'
import { createDefaultOpenAIProfile, DEFAULT_SETTINGS } from './apiProfiles'
import { __videoApiTestUtils, getVideoTaskSnapshot, refreshVideoModels, submitVideoTask } from './videoApi'
import { getDefaultVideoTemplateConfig, getDefaultVideoTemplateId, getVideoModelCapability, getVideoTemplate } from './videoCapabilities'

describe('videoApi', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })

  it('parses /v1/models payloads into a unique sorted list', () => {
    expect(__videoApiTestUtils.normalizeModelsPayload({
      data: [{ id: 'b' }, { id: 'a' }, { id: 'b' }, { id: '  c  ' }],
    })).toEqual(['a', 'b', 'c'])
  })

  it('keeps video template bindings per model', () => {
    const profile = createDefaultOpenAIProfile({
      videoConfig: {
        templateId: 'grok',
        availableModels: ['grok-model', 'omni_flash'],
        capabilityOverrides: {
          'grok-model': { ...getDefaultVideoTemplateConfig('grok'), templateId: 'grok' },
          omni_flash: { ...getDefaultVideoTemplateConfig('omni'), templateId: 'omni' },
        },
      },
    })

    expect(getVideoModelCapability(profile, 'grok-model')?.imageInput.max).toBe(1)
    expect(getVideoModelCapability(profile, 'grok-model')?.videoInput.max).toBe(0)
    expect(getVideoModelCapability(profile, 'omni_flash')?.imageInput.max).toBe(7)
    expect(getVideoModelCapability(profile, 'omni_flash')?.videoInput.max).toBe(1)
  })

  it('builds video-generations request bodies with metadata defaults', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-1',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = getDefaultVideoTemplateId()
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing grok template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: template.imageInput,
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
      durationOptions: [5, 10],
      resolutionOptions: ['1280x720'],
      metadataDefaults: { foo: 'bar' },
    }

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'video-model',
      prompt: 'prompt',
      capability,
      durationSeconds: 5,
      resolution: '1280x720',
      imageFiles: [],
      videoFiles: [],
      audioFiles: [],
    })

    const [, init] = fetchMock.mock.calls[0]
    expect(JSON.parse(String((init as RequestInit).body))).toEqual({
      model: 'video-model',
      prompt: 'prompt',
      duration: 5,
      width: 1280,
      height: 720,
      metadata: { foo: 'bar', aspect_ratio: '16:9' },
    })
  })

  it('encodes video-generations images as data URLs', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-1',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = getDefaultVideoTemplateId()
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing grok template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: { ...template.imageInput, transport: 'base64-json' as const },
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
    }
    const file = new File(['demo'], 'demo.png', { type: 'image/png' })

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'video-model',
      prompt: 'prompt',
      capability,
      durationSeconds: null,
      resolution: '',
      imageFiles: [{ id: 'i1', kind: 'image', name: 'demo.png', mimeType: 'image/png', size: file.size, blob: file, createdAt: Date.now() }],
      videoFiles: [],
      audioFiles: [],
    })

    const [, init] = fetchMock.mock.calls[0]
    const body = JSON.parse(String((init as RequestInit).body))
    expect(typeof body.image).toBe('string')
    expect(body.image).toMatch(/^[A-Za-z0-9+/=]+$/)
  })

  it('builds grok request bodies with imageUrl and metadata', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-2',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = getDefaultVideoTemplateId()
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing grok template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: template.imageInput,
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
    }

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'video-model',
      prompt: 'prompt',
      capability,
      durationSeconds: 10,
      resolution: '1280x720',
      imageFiles: [],
      videoFiles: [],
      audioFiles: [],
      imageUrl: 'https://cdn.example.com/input.png',
    })

    const [, init] = fetchMock.mock.calls[0]
    expect(JSON.parse(String((init as RequestInit).body))).toEqual({
      model: 'video-model',
      prompt: 'prompt',
      duration: 10,
      width: 1280,
      height: 720,
      image: 'https://cdn.example.com/input.png',
      metadata: { resolution: '720p', aspect_ratio: '16:9', audio: true },
    })
  })

  it('derives metadata aspect_ratio from the selected resolution', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-2b',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = getDefaultVideoTemplateId()
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing grok template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: template.imageInput,
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
      resolutionOptions: ['1024x768'],
      metadataDefaults: { resolution: '768p', aspect_ratio: '16:9', audio: true },
    }

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'video-model',
      prompt: 'prompt',
      capability,
      durationSeconds: 10,
      resolution: '1024x768',
      imageFiles: [],
      videoFiles: [],
      audioFiles: [],
    })

    const [, init] = fetchMock.mock.calls[0]
    expect(JSON.parse(String((init as RequestInit).body)).metadata.aspect_ratio).toBe('4:3')
  })

  it('builds omni request bodies with metadata images array', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-3',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = 'omni' as const
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing omni template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: template.imageInput,
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
    }

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'omni_flash',
      prompt: 'prompt',
      capability,
      durationSeconds: 10,
      resolution: '1920x1080',
      imageFiles: [],
      videoFiles: [],
      audioFiles: [],
    })

    const [, init] = fetchMock.mock.calls[0]
    expect(JSON.parse(String((init as RequestInit).body))).toEqual({
      model: 'omni_flash',
      prompt: 'prompt',
      duration: 10,
      width: 1920,
      height: 1080,
      metadata: { resolution: '1080p', aspect_ratio: '16:9', audio: false },
    })
  })

  it('builds omni request bodies with up to seven images and one video in metadata', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-4',
      status: 'queued',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const templateId = 'omni' as const
    const template = getVideoTemplate(templateId)
    if (!template) throw new Error('missing omni template')
    const capability = {
      endpointFamily: 'video-generations' as const,
      imageInput: template.imageInput,
      videoInput: template.videoInput,
      audioInput: template.audioInput,
      ...getDefaultVideoTemplateConfig(templateId),
      resolutionEncoding: 'width-height' as const,
    }
    const imageFiles = Array.from({ length: 7 }, (_, index) => {
      const file = new File([`image-${index}`], `image-${index}.png`, { type: 'image/png' })
      return { id: `i${index}`, kind: 'image' as const, name: file.name, mimeType: file.type, size: file.size, blob: file, createdAt: Date.now() }
    })
    const videoFile = new File(['video-demo'], 'demo.mp4', { type: 'video/mp4' })

    await submitVideoTask({
      profile,
      templateId,
      modelId: 'omni_flash',
      prompt: 'prompt',
      capability,
      durationSeconds: 10,
      resolution: '1920x1080',
      imageFiles,
      videoFiles: [{ id: 'v1', kind: 'video', name: 'demo.mp4', mimeType: 'video/mp4', size: videoFile.size, blob: videoFile, createdAt: Date.now() }],
      audioFiles: [],
    })

    const [, init] = fetchMock.mock.calls[0]
    const body = JSON.parse(String((init as RequestInit).body))
    expect(body.metadata.images).toHaveLength(7)
    expect(typeof body.metadata.video).toBe('string')
    expect(body).not.toHaveProperty('image')
  })

  it('normalizes video task snapshots from alternate payload shapes', async () => {
    const response = __videoApiTestUtils.normalizeVideoTaskPayload({
      data: {
        id: 'video-1',
        status: 'completed',
        video_url: 'https://cdn.example.com/video.mp4',
      },
    })

    expect(response).toMatchObject({
      remoteTaskId: 'video-1',
      remoteStatus: 'completed',
      resultUrl: 'https://cdn.example.com/video.mp4',
      status: 'done',
    })
  })

  it('normalizes NewAPI wrapped video result payloads', async () => {
    const response = __videoApiTestUtils.normalizeVideoTaskPayload({
      code: 'success',
      message: '',
      data: {
        id: 50,
        task_id: 'task_SAYju3X0ArZuNgSbS9datsqowEbE96O6',
        status: 'SUCCESS',
        result_url: 'https://file.bandianwa.com/ziyou2/video/demo.mp4',
        data: {
          id: 'task_PPLu20o7eYxRchl9JacyM9W8PNuQzEp8',
          task_id: 'task_PPLu20o7eYxRchl9JacyM9W8PNuQzEp8',
          status: 'completed',
          url: 'https://file.bandianwa.com/ziyou2/video/demo.mp4',
          video_url: 'https://file.bandianwa.com/ziyou2/video/demo.mp4',
        },
      },
    })

    expect(response).toMatchObject({
      remoteTaskId: 'task_SAYju3X0ArZuNgSbS9datsqowEbE96O6',
      remoteStatus: 'SUCCESS',
      resultUrl: 'https://file.bandianwa.com/ziyou2/video/demo.mp4',
      status: 'done',
      error: null,
    })
  })

  it('uses the models endpoint with the configured base URL', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      data: [{ id: 'video-1' }],
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const result = await refreshVideoModels(profile)

    expect(result.models).toEqual(['video-1'])
    expect(fetchMock).toHaveBeenCalledWith('https://api.example.com/v1/models', expect.objectContaining({ method: 'GET' }))
  })

  it('polls a video task by id', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 'task-1',
      status: 'done',
      url: 'https://cdn.example.com/video.mp4',
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))

    const profile = createDefaultOpenAIProfile({ apiKey: 'test-key', baseUrl: 'https://api.example.com/v1' })
    const snapshot = await getVideoTaskSnapshot(profile, getDefaultVideoTemplateId(), 'task-1')

    expect(snapshot).toMatchObject({
      remoteTaskId: 'task-1',
      remoteStatus: 'done',
      resultUrl: 'https://cdn.example.com/video.mp4',
      status: 'done',
    })
    expect(fetchMock).toHaveBeenCalledWith('https://api.example.com/v1/video/generations/task-1', expect.objectContaining({ method: 'GET' }))
  })
})
