import { useEffect, useMemo, useState } from 'react'
import { getDefaultVideoTemplateConfig, getDefaultVideoTemplateId, getVideoTemplate, normalizeVideoTemplateConfig, VIDEO_TEMPLATES } from '../lib/videoCapabilities'
import type { ApiProfile, VideoTemplateConfig, VideoTemplateId } from '../types'
import Select from './Select'

function formatNumberList(values: number[]) {
  return values.join(', ')
}

function parseNumberList(text: string) {
  return Array.from(new Set(text.split(/[\s,，]+/).map((item) => Number(item.trim())).filter((item) => Number.isFinite(item) && item > 0).map((item) => Math.trunc(item)))).sort((a, b) => a - b)
}

function parseStringLines(text: string) {
  return Array.from(new Set(text.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)))
}

function normalizeJsonInput(text: string, fallback: Record<string, unknown>) {
  try {
    const parsed = JSON.parse(text)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : fallback
  } catch {
    return fallback
  }
}

export default function VideoCapabilityManager({
  profile,
  enabled,
  onCommitProfilePatch,
}: {
  profile: ApiProfile
  enabled: boolean
  onCommitProfilePatch: (patch: Partial<ApiProfile>) => void
}) {
  const [templateId, setTemplateId] = useState<VideoTemplateId>(profile.videoConfig?.templateId ?? getDefaultVideoTemplateId())
  const [modelId, setModelId] = useState(profile.model.trim())
  const [durationText, setDurationText] = useState('')
  const [resolutionText, setResolutionText] = useState('')
  const [metadataText, setMetadataText] = useState('{}')

  const availableModels = useMemo(() => profile.videoConfig?.availableModels ?? [], [profile.videoConfig?.availableModels])
  const modelIds = useMemo(() => Array.from(new Set([profile.model, modelId, ...availableModels])).filter(Boolean), [availableModels, modelId, profile.model])
  const currentTemplate = getVideoTemplate(templateId) ?? getVideoTemplate(getDefaultVideoTemplateId())
  const configuredModelIds = useMemo(() => Object.keys(profile.videoConfig?.capabilityOverrides ?? {}).sort((a, b) => a.localeCompare(b)), [profile.videoConfig?.capabilityOverrides])
  const currentConfig = useMemo(() => {
    const patch = profile.videoConfig?.capabilityOverrides?.[modelId] ?? currentTemplate?.defaultConfig
    return normalizeVideoTemplateConfig(patch, currentTemplate?.defaultConfig ?? getDefaultVideoTemplateConfig(getDefaultVideoTemplateId()))
  }, [currentTemplate, modelId, profile.videoConfig?.capabilityOverrides])

  useEffect(() => {
    setModelId(profile.model.trim())
  }, [profile.model])

  useEffect(() => {
    const modelTemplateId = profile.videoConfig?.capabilityOverrides?.[modelId]?.templateId ?? profile.videoConfig?.templateId ?? getDefaultVideoTemplateId()
    setTemplateId(modelTemplateId)
  }, [modelId, profile.videoConfig?.capabilityOverrides, profile.videoConfig?.templateId])

  useEffect(() => {
    setDurationText(formatNumberList(currentConfig.durationOptions))
    setResolutionText(currentConfig.resolutionOptions.join('\n'))
    setMetadataText(JSON.stringify(currentConfig.metadataDefaults, null, 2))
  }, [currentConfig])

  const commitVideoConfig = (next: ApiProfile['videoConfig']) => {
    onCommitProfilePatch({ videoConfig: next })
  }

  const updateCurrentConfig = (patch: Partial<VideoTemplateConfig>) => {
    if (!modelId.trim()) return
    const nextConfig = normalizeVideoTemplateConfig({
      durationOptions: patch.durationOptions ?? currentConfig.durationOptions,
      resolutionOptions: patch.resolutionOptions ?? currentConfig.resolutionOptions,
      metadataDefaults: patch.metadataDefaults ?? currentConfig.metadataDefaults,
    }, currentConfig)
    commitVideoConfig({
      templateId: profile.videoConfig?.templateId ?? getDefaultVideoTemplateId(),
      availableModels: Array.from(new Set([...availableModels, modelId.trim()])),
      capabilityOverrides: {
        ...(profile.videoConfig?.capabilityOverrides ?? {}),
        [modelId.trim()]: { ...nextConfig, templateId },
      },
    })
  }

  const updateCurrentTemplate = (nextTemplateId: VideoTemplateId) => {
    setTemplateId(nextTemplateId)
    if (!modelId.trim()) return
    const defaults = getDefaultVideoTemplateConfig(nextTemplateId)
    commitVideoConfig({
      templateId: profile.videoConfig?.templateId ?? getDefaultVideoTemplateId(),
      availableModels: Array.from(new Set([...availableModels, modelId.trim()])),
      capabilityOverrides: {
        ...(profile.videoConfig?.capabilityOverrides ?? {}),
        [modelId.trim()]: normalizeVideoTemplateConfig({ ...defaults, templateId: nextTemplateId }, { ...defaults, templateId: nextTemplateId }),
      },
    })
  }

  const applyTemplateDefaults = () => {
    const defaults = getDefaultVideoTemplateConfig(templateId)
    updateCurrentConfig(defaults)
  }

  const addModelConfig = () => {
    const nextModelId = modelId.trim() || profile.model.trim() || availableModels[0]?.trim()
    if (!nextModelId) return
    const defaults = getDefaultVideoTemplateConfig(templateId)
    setModelId(nextModelId)
    commitVideoConfig({
      templateId: profile.videoConfig?.templateId ?? getDefaultVideoTemplateId(),
      availableModels: Array.from(new Set([...availableModels, nextModelId])),
      capabilityOverrides: {
        ...(profile.videoConfig?.capabilityOverrides ?? {}),
        [nextModelId]: normalizeVideoTemplateConfig({ ...defaults, templateId }, { ...defaults, templateId }),
      },
    })
  }

  const deleteModelConfig = (targetModelId: string) => {
    const nextOverrides = { ...(profile.videoConfig?.capabilityOverrides ?? {}) }
    delete nextOverrides[targetModelId]
    const nextModelId = targetModelId === modelId
      ? (Object.keys(nextOverrides)[0] ?? profile.model.trim() ?? '')
      : modelId
    setModelId(nextModelId)
    commitVideoConfig({
      templateId: profile.videoConfig?.templateId ?? getDefaultVideoTemplateId(),
      availableModels,
      capabilityOverrides: nextOverrides,
    })
  }

  return (
    <div className="rounded-2xl border border-gray-200/70 bg-white/80 p-4 shadow-sm dark:border-white/[0.08] dark:bg-white/[0.02]">
      <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h4 className="text-sm font-bold text-gray-800 dark:text-gray-100">视频模板管理</h4>
          <div className="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
            这里只管理模板绑定和模板参数。底层请求格式已经固定为模板定义，前端不再编辑上传协议或底层端点。
          </div>
        </div>
        <div className="text-xs text-gray-400 dark:text-gray-500">
          已发现 {availableModels.length} 个模型
        </div>
      </div>

      {!enabled ? (
        <div className="rounded-2xl border border-dashed border-gray-200 px-4 py-5 text-sm text-gray-500 dark:border-white/[0.08] dark:text-gray-400">
          当前服务商不是 OpenAI 兼容接口，视频模板管理不可用。
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid gap-4">
            <label className="block">
              <span className="mb-1.5 block text-sm text-gray-600 dark:text-gray-300">模板</span>
              <Select
                value={templateId}
                onChange={(value) => updateCurrentTemplate(value as VideoTemplateId)}
                options={Object.values(VIDEO_TEMPLATES).map((template) => ({ label: template.name, value: template.id }))}
                className="w-full rounded-xl border border-gray-200/70 bg-white/60 px-3 py-2.5 text-sm text-gray-700 outline-none transition focus:border-blue-300 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200 dark:focus:border-blue-500/50"
              />
            </label>
            <label className="block">
              <span className="mb-1.5 block text-sm text-gray-600 dark:text-gray-300">模型</span>
              <Select
                value={modelId}
                onChange={(value) => setModelId(String(value))}
                options={modelIds.length > 0 ? modelIds.map((item) => ({ label: item, value: item })) : [{ label: '请先填写模型 ID 或刷新模型列表', value: '' }]}
                className="w-full min-h-[48px] rounded-xl border border-gray-200/70 bg-white/60 px-3 py-2.5 text-sm text-gray-700 outline-none transition focus:border-blue-300 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200 dark:focus:border-blue-500/50"
              />
              <div className="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                选中的模型会绑定到当前模板。
              </div>
            </label>
          </div>

          <div className="flex flex-wrap gap-2">
            <button type="button" onClick={addModelConfig} className="rounded-xl bg-blue-500 px-3 py-2 text-xs font-medium text-white transition hover:bg-blue-600">
              新增/绑定当前模型
            </button>
            <button type="button" onClick={applyTemplateDefaults} className="rounded-xl border border-gray-200/70 bg-white px-3 py-2 text-xs font-medium text-gray-700 transition hover:bg-gray-50 dark:border-white/[0.08] dark:bg-white/[0.04] dark:text-gray-200 dark:hover:bg-white/[0.07]">
              恢复模板默认值
            </button>
          </div>

          <div className="rounded-2xl border border-gray-200/70 bg-gray-50/70 p-3 dark:border-white/[0.08] dark:bg-white/[0.03]">
            <div className="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">已配置模型</div>
            {configuredModelIds.length === 0 ? (
              <div className="text-xs text-gray-500 dark:text-gray-400">暂无显式模型配置。点击“新增/绑定当前模型”后会把当前模型加入配置列表。</div>
            ) : (
              <div className="space-y-2">
                {configuredModelIds.map((item) => {
                  const itemTemplateId = profile.videoConfig?.capabilityOverrides?.[item]?.templateId ?? profile.videoConfig?.templateId ?? getDefaultVideoTemplateId()
                  const config = normalizeVideoTemplateConfig(profile.videoConfig?.capabilityOverrides?.[item], getDefaultVideoTemplateConfig(itemTemplateId))
                  return (
                    <div key={item} className={`flex min-w-0 flex-col gap-2 rounded-xl border px-3 py-2 text-xs sm:flex-row sm:items-center sm:justify-between ${item === modelId ? 'border-blue-200 bg-blue-50/70 text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-300' : 'border-gray-200/70 bg-white/70 text-gray-600 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-300'}`}>
                      <button type="button" onClick={() => setModelId(item)} className="min-w-0 text-left">
                        <div className="break-all font-medium">{item}</div>
                        <div className="mt-1 text-[11px] opacity-75">
                          模板 {itemTemplateId} · 时长 {config.durationOptions.join(', ') || '未配置'} · 分辨率 {config.resolutionOptions.join(', ') || '未配置'}
                        </div>
                      </button>
                      <button type="button" onClick={() => deleteModelConfig(item)} className="shrink-0 rounded-lg border border-red-200/70 bg-white px-2.5 py-1 text-[11px] font-medium text-red-500 transition hover:bg-red-50 dark:border-red-500/20 dark:bg-white/[0.04] dark:hover:bg-red-500/10">
                        删除
                      </button>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <label className="block">
              <span className="mb-1.5 block text-sm text-gray-600 dark:text-gray-300">时长白名单（秒）</span>
              <input
                type="text"
                value={durationText}
                onChange={(event) => setDurationText(event.target.value)}
                onBlur={() => updateCurrentConfig({ durationOptions: parseNumberList(durationText) })}
                placeholder="例如 10"
                className="w-full rounded-xl border border-gray-200/70 bg-white/80 px-3 py-2.5 text-sm text-gray-700 outline-none transition focus:border-blue-300 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200 dark:focus:border-blue-500/50"
              />
            </label>
            <label className="block">
              <span className="mb-1.5 block text-sm text-gray-600 dark:text-gray-300">分辨率白名单</span>
              <textarea
                value={resolutionText}
                onChange={(event) => setResolutionText(event.target.value)}
                onBlur={() => updateCurrentConfig({ resolutionOptions: parseStringLines(resolutionText) })}
                rows={3}
                placeholder={'每行一个，如\n1280x720'}
                className="w-full rounded-2xl border border-gray-200/70 bg-white/80 px-3 py-2.5 text-sm text-gray-700 outline-none transition focus:border-blue-300 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200 dark:focus:border-blue-500/50"
              />
            </label>
          </div>

          <label className="block">
            <span className="mb-1.5 block text-sm text-gray-600 dark:text-gray-300">默认 metadata（JSON）</span>
            <textarea
              value={metadataText}
              onChange={(event) => setMetadataText(event.target.value)}
              onBlur={() => updateCurrentConfig({ metadataDefaults: normalizeJsonInput(metadataText, currentConfig.metadataDefaults) })}
              rows={8}
              spellCheck={false}
              className="w-full rounded-2xl border border-gray-200/70 bg-white/80 px-3 py-2.5 font-mono text-xs leading-relaxed text-gray-700 outline-none transition focus:border-blue-300 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200 dark:focus:border-blue-500/50"
            />
          </label>
        </div>
      )}
    </div>
  )
}
