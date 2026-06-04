/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { SpottedFrogModelMap } from '../types'

export type VideoModelRoutingCondition = {
  path: string
  mode: string
  value: unknown
  invert?: boolean
  pass_missing_key?: boolean
}

export type VideoModelRoutingOperation = {
  path?: string
  mode: string
  value?: unknown
  keep_origin?: boolean
  from?: string
  to?: string
  conditions?: VideoModelRoutingCondition[]
  logic?: string
}

export type VideoModelRoutingRule = {
  name?: string
  enabled?: boolean
  logic?: string
  conditions?: VideoModelRoutingCondition[]
  target_model?: string
  operations?: VideoModelRoutingOperation[]
}

export type VideoModelRoutingRuleSet = {
  version: number
  rules: VideoModelRoutingRule[]
}

export const EMPTY_VIDEO_MODEL_ROUTING_RULE_SET: VideoModelRoutingRuleSet = {
  version: 1,
  rules: [],
}

const LEGACY_SPOTTEDFROG_DEFAULTS: SpottedFrogModelMap = {
  sora_2_16x9_4s: 'sora-2-4s-16x9',
  sora_2_16x9_8s: 'sora-2-8s-16x9',
  sora_2_16x9_12s: 'sora-2-12s-16x9',
  sora_2_9x16_4s: 'sora-2-4s-9x16',
  sora_2_9x16_8s: 'sora-2-8s-9x16',
  sora_2_9x16_12s: 'sora-2-12s-9x16',
  sora_2_pro_16x9_12s: 'sora2-pro-12s-16x9',
  sora_2_pro_9x16_12s: 'sora2-pro-12s-9x16',
  omni_flash: 'omni_flash',
  grok_imagine_video: 'grok-imagine-video',
  veo_fast_16x9_8s_1080p: 'firefly-veo31-fast-8s-16x9-1080p',
  veo_fast_9x16_8s_1080p: 'firefly-veo31-fast-8s-9x16-1080p',
  veo_standard_16x9_8s_1080p: 'firefly-veo31-standard-8s-16x9-1080p',
  veo_standard_9x16_8s_1080p: 'firefly-veo31-standard-8s-9x16-1080p',
  veo_ref_16x9_8s_1080p: 'firefly-veo31-ref-8s-16x9-1080p',
  veo_ref_9x16_8s_1080p: 'firefly-veo31-ref-8s-9x16-1080p',
}

const condition = (
  path: string,
  value: unknown,
  options?: Partial<VideoModelRoutingCondition>
): VideoModelRoutingCondition => ({
  path,
  mode: options?.mode || 'full',
  value,
  invert: options?.invert,
  pass_missing_key: options?.pass_missing_key,
})

const mergeSpottedFrogDefaults = (
  overrides?: Partial<SpottedFrogModelMap>
): SpottedFrogModelMap => ({
  ...LEGACY_SPOTTEDFROG_DEFAULTS,
  ...(overrides || {}),
})

export function parseVideoModelRoutingRules(
  value: string | undefined
): VideoModelRoutingRuleSet {
  if (!value?.trim()) {
    return EMPTY_VIDEO_MODEL_ROUTING_RULE_SET
  }
  const parsed = JSON.parse(value) as Partial<VideoModelRoutingRuleSet>
  return {
    version:
      typeof parsed.version === 'number' && parsed.version > 0
        ? parsed.version
        : 1,
    rules: Array.isArray(parsed.rules) ? parsed.rules : [],
  }
}

export function stringifyVideoModelRoutingRules(
  ruleSet: VideoModelRoutingRuleSet
): string {
  return JSON.stringify(
    {
      version: ruleSet.version || 1,
      rules: ruleSet.rules || [],
    },
    null,
    2
  )
}

export function buildVideoModelRoutingTemplate(): string {
  return stringifyVideoModelRoutingRules({
    version: 1,
    rules: [
      {
        name: 'sora-2 8s 16:9',
        enabled: true,
        logic: 'AND',
        conditions: [
          condition('derived.original_model', 'sora-2'),
          condition('derived.duration', 8),
          condition('derived.aspect_ratio', '16:9'),
        ],
        target_model: 'sora-2-8s-16x9',
        operations: [
          {
            path: 'request.metadata.variant',
            mode: 'delete',
          },
        ],
      },
    ],
  })
}

export function buildSpottedFrogLegacyRoutingRules(
  overrides?: Partial<SpottedFrogModelMap>
): string {
  const effective = mergeSpottedFrogDefaults(overrides)
  const rules: VideoModelRoutingRule[] = [
    {
      name: 'Sora 2 16:9 4s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 4),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_16x9_4s,
    },
    {
      name: 'Sora 2 16:9 8s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 8),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_16x9_8s,
    },
    {
      name: 'Sora 2 16:9 12s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 12, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_16x9_12s,
    },
    {
      name: 'Sora 2 9:16 4s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 4),
        condition('derived.aspect_ratio', '9:16'),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_9x16_4s,
    },
    {
      name: 'Sora 2 9:16 8s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 8),
        condition('derived.aspect_ratio', '9:16'),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_9x16_8s,
    },
    {
      name: 'Sora 2 9:16 12s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('derived.duration', 12, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '9:16'),
        condition('request.metadata.variant', 'pro', {
          invert: true,
          pass_missing_key: true,
        }),
      ],
      target_model: effective.sora_2_9x16_12s,
    },
    {
      name: 'Sora 2 Pro 16:9 12s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('request.metadata.variant', 'pro'),
        condition('derived.duration', 12, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
      ],
      target_model: effective.sora_2_pro_16x9_12s,
    },
    {
      name: 'Sora 2 Pro 9:16 12s',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'sora-2'),
        condition('request.metadata.variant', 'pro'),
        condition('derived.duration', 12, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '9:16'),
      ],
      target_model: effective.sora_2_pro_9x16_12s,
    },
    {
      name: 'Omni Flash',
      enabled: true,
      logic: 'AND',
      conditions: [condition('derived.original_model', 'omni_flash')],
      target_model: effective.omni_flash,
    },
    {
      name: 'Grok Imagine Video',
      enabled: true,
      logic: 'AND',
      conditions: [condition('derived.original_model', 'grok-imagine-video')],
      target_model: effective.grok_imagine_video,
    },
    {
      name: 'Veo Ref 16:9 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('derived.image_count', 0, { mode: 'gt' }),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_ref_16x9_8s_1080p,
    },
    {
      name: 'Veo Ref 9:16 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('derived.image_count', 0, { mode: 'gt' }),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '9:16'),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_ref_9x16_8s_1080p,
    },
    {
      name: 'Veo Standard 16:9 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('request.metadata.speed', 'standard'),
        condition('derived.image_count', 0),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_standard_16x9_8s_1080p,
    },
    {
      name: 'Veo Standard 9:16 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('request.metadata.speed', 'standard'),
        condition('derived.image_count', 0),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '9:16'),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_standard_9x16_8s_1080p,
    },
    {
      name: 'Veo Fast 16:9 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('request.metadata.speed', 'standard', {
          invert: true,
          pass_missing_key: true,
        }),
        condition('derived.image_count', 0),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '16:9', { pass_missing_key: true }),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_fast_16x9_8s_1080p,
    },
    {
      name: 'Veo Fast 9:16 8s 1080p',
      enabled: true,
      logic: 'AND',
      conditions: [
        condition('derived.original_model', 'veo'),
        condition('request.metadata.speed', 'standard', {
          invert: true,
          pass_missing_key: true,
        }),
        condition('derived.image_count', 0),
        condition('derived.duration', 8, { pass_missing_key: true }),
        condition('derived.aspect_ratio', '9:16'),
        condition('derived.resolution', '1080p', { pass_missing_key: true }),
      ],
      target_model: effective.veo_fast_9x16_8s_1080p,
    },
  ]

  return stringifyVideoModelRoutingRules({
    version: 1,
    rules,
  })
}
