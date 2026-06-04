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
import { useEffect, useMemo, useState } from 'react'
import {
  ChevronDown,
  ChevronUp,
  Code2,
  Copy,
  Plus,
  Trash2,
  Wand2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  type VideoModelRoutingCondition,
  type VideoModelRoutingOperation,
  type VideoModelRoutingRule,
  parseVideoModelRoutingRules,
  stringifyVideoModelRoutingRules,
} from '../../lib'
import { ParamOverrideEditorDialog } from './param-override-editor-dialog'

type EditableCondition = {
  id: string
  path: string
  mode: string
  value_text: string
  invert: boolean
  pass_missing_key: boolean
}

type EditableRule = {
  id: string
  name: string
  enabled: boolean
  logic: string
  target_model: string
  conditions: EditableCondition[]
  operations: VideoModelRoutingOperation[]
}

export type VideoModelRoutingEditorDialogProps = {
  open: boolean
  value: string
  onOpenChange: (open: boolean) => void
  onSave: (value: string) => void
}

const CONDITION_MODE_OPTIONS = [
  { label: 'Exact Match', value: 'full' },
  { label: 'Prefix', value: 'prefix' },
  { label: 'Suffix', value: 'suffix' },
  { label: 'Contains', value: 'contains' },
  { label: 'Greater Than', value: 'gt' },
  { label: 'Greater Than or Equal', value: 'gte' },
  { label: 'Less Than', value: 'lt' },
  { label: 'Less Than or Equal', value: 'lte' },
]

const UNSUPPORTED_OPERATION_MODES = new Set([
  'set_header',
  'delete_header',
  'copy_header',
  'move_header',
  'pass_headers',
])

let localIdSeed = 0
const nextLocalId = () => `vmr_${Date.now()}_${localIdSeed++}`

const toValueText = (value: unknown): string => {
  if (value === undefined) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const parseLooseValue = (valueText: string): unknown => {
  const raw = String(valueText ?? '').trim()
  if (raw === '') return ''
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

const createDefaultCondition = (): EditableCondition => ({
  id: nextLocalId(),
  path: '',
  mode: 'full',
  value_text: '',
  invert: false,
  pass_missing_key: false,
})

const createDefaultRule = (): EditableRule => ({
  id: nextLocalId(),
  name: '',
  enabled: true,
  logic: 'AND',
  target_model: '',
  conditions: [createDefaultCondition()],
  operations: [],
})

const normalizeCondition = (
  condition: VideoModelRoutingCondition | undefined
): EditableCondition => ({
  id: nextLocalId(),
  path: typeof condition?.path === 'string' ? condition.path : '',
  mode: typeof condition?.mode === 'string' ? condition.mode : 'full',
  value_text: toValueText(condition?.value),
  invert: condition?.invert === true,
  pass_missing_key: condition?.pass_missing_key === true,
})

const normalizeRule = (rule: VideoModelRoutingRule | undefined): EditableRule => ({
  id: nextLocalId(),
  name: typeof rule?.name === 'string' ? rule.name : '',
  enabled: rule?.enabled !== false,
  logic: String(rule?.logic || 'AND').toUpperCase() === 'OR' ? 'OR' : 'AND',
  target_model:
    typeof rule?.target_model === 'string' ? rule.target_model : '',
  conditions: Array.isArray(rule?.conditions)
    ? rule.conditions.map(normalizeCondition)
    : [createDefaultCondition()],
  operations: Array.isArray(rule?.operations) ? rule.operations : [],
})

const buildConditionPayload = (
  condition: EditableCondition
): VideoModelRoutingCondition | null => {
  const path = condition.path.trim()
  const valueText = condition.value_text.trim()
  if (!path || !valueText) return null
  return {
    path,
    mode: condition.mode || 'full',
    value: parseLooseValue(valueText),
    invert: condition.invert || undefined,
    pass_missing_key: condition.pass_missing_key || undefined,
  }
}

const buildRulePayload = (rule: EditableRule): VideoModelRoutingRule => ({
  name: rule.name.trim() || undefined,
  enabled: rule.enabled,
  logic: rule.logic || 'AND',
  target_model: rule.target_model.trim(),
  conditions: rule.conditions
    .map(buildConditionPayload)
    .filter(Boolean) as VideoModelRoutingCondition[],
  operations: rule.operations,
})

const summarizeOperations = (operations: VideoModelRoutingOperation[]): string => {
  if (operations.length === 0) return 'No operations'
  return operations
    .slice(0, 2)
    .map((operation) => {
      const mode = String(operation.mode || 'set')
      const path =
        String(operation.path || operation.from || operation.to || '').trim() ||
        '-'
      return `${mode}:${path}`
    })
    .join(' · ')
}

const hasUnsupportedOperationModes = (
  operations: VideoModelRoutingOperation[]
): string | null => {
  for (const operation of operations) {
    const mode = String(operation.mode || '').trim().toLowerCase()
    if (UNSUPPORTED_OPERATION_MODES.has(mode)) {
      return mode
    }
  }
  return null
}

export function VideoModelRoutingEditorDialog({
  open,
  value,
  onOpenChange,
  onSave,
}: VideoModelRoutingEditorDialogProps) {
  const { t } = useTranslation()
  const [rules, setRules] = useState<EditableRule[]>([])
  const [rawJson, setRawJson] = useState('')
  const [viewMode, setViewMode] = useState<'visual' | 'json'>('visual')
  const [operationsEditorIndex, setOperationsEditorIndex] = useState<
    number | null
  >(null)

  useEffect(() => {
    if (!open) return
    try {
      const parsed = parseVideoModelRoutingRules(value)
      setRules(parsed.rules.map(normalizeRule))
      setRawJson(stringifyVideoModelRoutingRules(parsed))
      setViewMode('visual')
    } catch {
      setRules([])
      setRawJson(value || '')
      setViewMode('json')
    }
  }, [open, value])

  const serializedVisualRules = useMemo(
    () =>
      stringifyVideoModelRoutingRules({
        version: 1,
        rules: rules.map(buildRulePayload),
      }),
    [rules]
  )

  const syncJsonToVisual = () => {
    try {
      const parsed = parseVideoModelRoutingRules(rawJson)
      setRules(parsed.rules.map(normalizeRule))
      setRawJson(stringifyVideoModelRoutingRules(parsed))
      setViewMode('visual')
    } catch {
      toast.error(t('Model routing rules JSON is invalid'))
    }
  }

  const handleSave = () => {
    const finalJson = viewMode === 'visual' ? serializedVisualRules : rawJson
    let parsed
    try {
      parsed = parseVideoModelRoutingRules(finalJson)
    } catch {
      toast.error(t('Model routing rules JSON is invalid'))
      return
    }
    for (const rule of parsed.rules) {
      const targetModel = String(rule.target_model || '').trim()
      if (!targetModel) {
        toast.error(t('Each routing rule must define a target model'))
        return
      }
      const blockedMode = hasUnsupportedOperationModes(rule.operations || [])
      if (blockedMode) {
        toast.error(
          t('Operation mode {{mode}} is not supported in video routing rules', {
            mode: blockedMode,
          })
        )
        return
      }
    }
    onSave(stringifyVideoModelRoutingRules(parsed))
    onOpenChange(false)
  }

  const updateRule = (ruleId: string, updater: (rule: EditableRule) => EditableRule) => {
    setRules((current) =>
      current.map((rule) => (rule.id === ruleId ? updater(rule) : rule))
    )
  }

  const moveRule = (index: number, direction: -1 | 1) => {
    setRules((current) => {
      const target = index + direction
      if (target < 0 || target >= current.length) return current
      const next = [...current]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  const operationsEditorValue =
    operationsEditorIndex === null
      ? ''
      : JSON.stringify(
          {
            operations: rules[operationsEditorIndex]?.operations || [],
          },
          null,
          2
        )

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='max-h-[90vh] max-w-6xl overflow-hidden'>
          <DialogHeader>
            <DialogTitle>{t('Video Model Routing Rules')}</DialogTitle>
            <DialogDescription>
              {t(
                'Route downstream video models to different upstream models with conditions and request-body operations.'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant={viewMode === 'visual' ? 'default' : 'outline'}
              size='sm'
              onClick={() => setViewMode('visual')}
            >
              <Wand2 className='mr-2 h-4 w-4' />
              {t('Visual')}
            </Button>
            <Button
              type='button'
              variant={viewMode === 'json' ? 'default' : 'outline'}
              size='sm'
              onClick={() => {
                setRawJson(serializedVisualRules)
                setViewMode('json')
              }}
            >
              <Code2 className='mr-2 h-4 w-4' />
              {t('JSON')}
            </Button>
            {viewMode === 'json' && (
              <Button type='button' variant='outline' size='sm' onClick={syncJsonToVisual}>
                {t('Apply JSON to Visual')}
              </Button>
            )}
          </div>

          {viewMode === 'json' ? (
            <Textarea
              value={rawJson}
              onChange={(e) => setRawJson(e.target.value)}
              rows={24}
              className='font-mono text-xs'
            />
          ) : (
            <div className='space-y-4 overflow-hidden'>
              <div className='flex items-center justify-between'>
                <div className='text-muted-foreground text-sm'>
                  {t('Rules are evaluated from top to bottom. The first match wins.')}
                </div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setRules((current) => [...current, createDefaultRule()])}
                >
                  <Plus className='mr-2 h-4 w-4' />
                  {t('Add rule')}
                </Button>
              </div>

              <ScrollArea className='h-[58vh] pr-4'>
                <div className='space-y-4'>
                  {rules.length === 0 && (
                    <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-sm'>
                      {t('No routing rules yet. Add one to start building your video routing logic.')}
                    </div>
                  )}

                  {rules.map((rule, index) => (
                    <div key={rule.id} className='space-y-4 rounded-xl border p-4'>
                      <div className='flex flex-wrap items-center gap-2'>
                        <Badge variant='secondary'>{index + 1}</Badge>
                        <Input
                          value={rule.name}
                          onChange={(e) =>
                            updateRule(rule.id, (current) => ({
                              ...current,
                              name: e.target.value,
                            }))
                          }
                          placeholder={t('Rule name')}
                          className='max-w-xs'
                        />
                        <div className='ml-auto flex items-center gap-2'>
                          <div className='flex items-center gap-2 text-sm'>
                            <span>{t('Enabled')}</span>
                            <Switch
                              checked={rule.enabled}
                              onCheckedChange={(checked) =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  enabled: checked,
                                }))
                              }
                            />
                          </div>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() =>
                              setRules((current) => [
                                ...current,
                                normalizeRule(buildRulePayload(rule)),
                              ])
                            }
                          >
                            <Copy className='h-4 w-4' />
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => moveRule(index, -1)}
                            disabled={index === 0}
                          >
                            <ChevronUp className='h-4 w-4' />
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => moveRule(index, 1)}
                            disabled={index === rules.length - 1}
                          >
                            <ChevronDown className='h-4 w-4' />
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() =>
                              setRules((current) =>
                                current.filter((item) => item.id !== rule.id)
                              )
                            }
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      </div>

                      <div className='grid gap-4 md:grid-cols-2'>
                        <div className='space-y-2'>
                          <div className='text-sm font-medium'>
                            {t('Target upstream model')}
                          </div>
                          <Input
                            value={rule.target_model}
                            onChange={(e) =>
                              updateRule(rule.id, (current) => ({
                                ...current,
                                target_model: e.target.value,
                              }))
                            }
                            placeholder='sora-2-8s-16x9'
                          />
                        </div>
                        <div className='space-y-2'>
                          <div className='text-sm font-medium'>{t('Condition logic')}</div>
                          <Select
                            value={rule.logic}
                            onValueChange={(nextValue) =>
                              updateRule(rule.id, (current) => ({
                                ...current,
                                logic: nextValue === 'OR' ? 'OR' : 'AND',
                              }))
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='AND'>AND</SelectItem>
                              <SelectItem value='OR'>OR</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>

                      <div className='space-y-3'>
                        <div className='flex items-center justify-between'>
                          <div className='text-sm font-medium'>{t('Conditions')}</div>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              updateRule(rule.id, (current) => ({
                                ...current,
                                conditions: [...current.conditions, createDefaultCondition()],
                              }))
                            }
                          >
                            <Plus className='mr-2 h-4 w-4' />
                            {t('Add condition')}
                          </Button>
                        </div>

                        <div className='space-y-3'>
                          {rule.conditions.map((condition, conditionIndex) => (
                            <div
                              key={condition.id}
                              className='grid gap-3 rounded-lg border p-3 lg:grid-cols-[1.4fr_0.9fr_1.2fr_auto]'
                            >
                              <Input
                                value={condition.path}
                                onChange={(e) =>
                                  updateRule(rule.id, (current) => ({
                                    ...current,
                                    conditions: current.conditions.map((item) =>
                                      item.id === condition.id
                                        ? { ...item, path: e.target.value }
                                        : item
                                    ),
                                  }))
                                }
                                placeholder='derived.original_model'
                              />
                              <Select
                                value={condition.mode}
                                onValueChange={(nextValue) =>
                                  updateRule(rule.id, (current) => ({
                                    ...current,
                                    conditions: current.conditions.map((item) =>
                                      item.id === condition.id
                                        ? { ...item, mode: nextValue || item.mode }
                                        : item
                                    ),
                                  }))
                                }
                              >
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  {CONDITION_MODE_OPTIONS.map((option) => (
                                    <SelectItem key={option.value} value={option.value}>
                                      {t(option.label)}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <Input
                                value={condition.value_text}
                                onChange={(e) =>
                                  updateRule(rule.id, (current) => ({
                                    ...current,
                                    conditions: current.conditions.map((item) =>
                                      item.id === condition.id
                                        ? { ...item, value_text: e.target.value }
                                        : item
                                    ),
                                  }))
                                }
                                placeholder='sora-2'
                              />
                              <div className='flex items-center justify-end gap-2'>
                                <div className='flex flex-col gap-2 text-xs'>
                                  <label className='flex items-center gap-2'>
                                    <Switch
                                      checked={condition.invert}
                                      onCheckedChange={(checked) =>
                                        updateRule(rule.id, (current) => ({
                                          ...current,
                                          conditions: current.conditions.map((item) =>
                                            item.id === condition.id
                                              ? { ...item, invert: checked }
                                              : item
                                          ),
                                        }))
                                      }
                                    />
                                    {t('NOT')}
                                  </label>
                                  <label className='flex items-center gap-2'>
                                    <Switch
                                      checked={condition.pass_missing_key}
                                      onCheckedChange={(checked) =>
                                        updateRule(rule.id, (current) => ({
                                          ...current,
                                          conditions: current.conditions.map((item) =>
                                            item.id === condition.id
                                              ? {
                                                  ...item,
                                                  pass_missing_key: checked,
                                                }
                                              : item
                                          ),
                                        }))
                                      }
                                    />
                                    {t('Pass missing')}
                                  </label>
                                </div>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  onClick={() =>
                                    updateRule(rule.id, (current) => ({
                                      ...current,
                                      conditions: current.conditions.filter(
                                        (item) => item.id !== condition.id
                                      ),
                                    }))
                                  }
                                  disabled={rule.conditions.length === 1}
                                >
                                  <Trash2 className='h-4 w-4' />
                                </Button>
                              </div>
                              <div className='text-muted-foreground lg:col-span-4 text-xs'>
                                {t('Examples: derived.original_model, derived.duration, request.metadata.variant, derived.image_count')}
                                {conditionIndex < rule.conditions.length - 1
                                  ? ` · ${t('Combined with')} ${rule.logic}`
                                  : ''}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>

                      <div className='space-y-3 rounded-lg border border-dashed p-3'>
                        <div className='flex items-center justify-between gap-3'>
                          <div>
                            <div className='text-sm font-medium'>
                              {t('Request operations')}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              {t(
                                'Edit request-body operations for this matched rule. Header operations are blocked for video routing.'
                              )}
                            </div>
                          </div>
                          <div className='flex gap-2'>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={() => setOperationsEditorIndex(index)}
                            >
                              <Wand2 className='mr-2 h-4 w-4' />
                              {t('Edit operations')}
                            </Button>
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              onClick={() =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  operations: [],
                                }))
                              }
                              disabled={rule.operations.length === 0}
                            >
                              {t('Clear')}
                            </Button>
                          </div>
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {rule.operations.length > 0
                            ? summarizeOperations(rule.operations)
                            : t('No operations')}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </div>
          )}

          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
              {t('Cancel')}
            </Button>
            <Button type='button' onClick={handleSave}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {operationsEditorIndex !== null && (
        <ParamOverrideEditorDialog
          open={operationsEditorIndex !== null}
          value={operationsEditorValue}
          onOpenChange={(nextOpen) => {
            if (!nextOpen) setOperationsEditorIndex(null)
          }}
          onSave={(nextValue) => {
            try {
              const parsed = JSON.parse(nextValue) as {
                operations?: VideoModelRoutingOperation[]
              }
              const operations = Array.isArray(parsed.operations)
                ? parsed.operations
                : []
              const blockedMode = hasUnsupportedOperationModes(operations)
              if (blockedMode) {
                toast.error(
                  t(
                    'Operation mode {{mode}} is not supported in video routing rules',
                    {
                      mode: blockedMode,
                    }
                  )
                )
                return
              }
              setRules((current) =>
                current.map((rule, index) =>
                  index === operationsEditorIndex
                    ? { ...rule, operations }
                    : rule
                )
              )
              setOperationsEditorIndex(null)
            } catch {
              toast.error(t('Operations JSON is invalid'))
            }
          }}
        />
      )}
    </>
  )
}
