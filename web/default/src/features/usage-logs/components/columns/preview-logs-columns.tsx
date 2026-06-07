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
/* eslint-disable react-refresh/only-export-components */
import { useEffect, useMemo, useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Eye, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { api } from '@/lib/api'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { CopyButton } from '@/components/copy-button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { DataTableColumnHeader } from '@/components/data-table'
import { useUsageLogsContext } from '../usage-logs-provider'
import type { PreviewLog } from '../../types'
import { createChannelColumn } from './column-helpers'

type PreviewRequestPayload = {
  raw_http?: string
  method?: string
  path?: string
  query?: Record<string, string[]>
  url?: string
  headers?: Record<string, string>
}

type ChannelPreviewResponse = {
  object?: string
  channel?: {
    id?: number
    type?: number
    base_url?: string
    origin_model?: string
    upstream_model?: string
    request_preview_mode_enabled?: boolean
  }
  relay?: {
    request_path?: string
    relay_mode?: string
    client_requested_stream?: boolean
    response_mode?: string
    request_conversion_chain?: string[]
    final_request_relay_format?: string
  }
  downstream_request?: PreviewRequestPayload
  upstream_request?: PreviewRequestPayload
}

function parsePreviewPayload(payload: string): ChannelPreviewResponse | null {
  try {
    const parsed = JSON.parse(payload) as ChannelPreviewResponse
    if (parsed?.object === 'channel_request_preview') {
      return parsed
    }
  } catch {}
  return null
}

function stringifyPreviewValue(value: unknown): string {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function PreviewInfoBlock({
  label,
  value,
}: {
  label: string
  value: unknown
}) {
  const displayValue = stringifyPreviewValue(value)
  const multiline = displayValue.includes('\n') || displayValue.length > 80

  return (
    <div className='rounded-lg border bg-muted/20 p-3'>
      <div className='text-muted-foreground mb-1 text-[11px] font-medium uppercase tracking-wide'>
        {label}
      </div>
      {multiline ? (
        <pre className='overflow-x-auto whitespace-pre-wrap break-all text-xs leading-5'>
          {displayValue}
        </pre>
      ) : (
        <div className='font-mono text-xs'>{displayValue}</div>
      )}
    </div>
  )
}

function PreviewPacketPanel({
  title,
  packet,
}: {
  title: string
  packet?: PreviewRequestPayload
}) {
  const packetText = useMemo(() => {
    if (packet?.raw_http) return packet.raw_http
    if (!packet) return '-'

    const lines: string[] = []
    const target = packet.path || packet.url || '/'
    lines.push(`${packet.method || 'POST'} ${target} HTTP/1.1`)
    Object.entries(packet.headers || {}).forEach(([key, value]) => {
      lines.push(`${key}: ${value}`)
    })
    lines.push('')
    return lines.join('\r\n')
  }, [packet])

  return (
    <div className='space-y-3 rounded-xl border p-4'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-semibold'>{title}</div>
        {packetText && packetText !== '-' ? (
          <CopyButton value={packetText} variant='ghost' size='icon' className='size-7' />
        ) : null}
      </div>
      <pre className='max-h-[48vh] overflow-auto rounded-lg bg-muted/20 p-3 text-xs leading-5 whitespace-pre-wrap break-all'>
        {packetText || '-'}
      </pre>
    </div>
  )
}

function PreviewPayloadDialog({
  open,
  onOpenChange,
  log,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  log: PreviewLog
}) {
  const { t } = useTranslation()
  const [payload, setPayload] = useState(log.payload || '')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setPayload(log.payload || '')
  }, [log.id, log.payload])

  useEffect(() => {
    if (!open) return
    if (payload) return

    let cancelled = false
    const fetchPayload = async () => {
      setLoading(true)
      try {
        const res = await api.get(`/api/preview-log/${log.id}`)
        if (!cancelled && res.data?.success) {
          setPayload(res.data.data?.payload || '')
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    void fetchPayload()
    return () => {
      cancelled = true
    }
  }, [log.id, open, payload])

  const parsed = useMemo(() => parsePreviewPayload(payload), [payload])
  const formatted = useMemo(() => {
    if (parsed) {
      return JSON.stringify(parsed, null, 2)
    }
    return payload
  }, [parsed, payload])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('Request Preview Details')}</DialogTitle>
        </DialogHeader>
        <ScrollArea className='max-h-[70vh] rounded-md border'>
          {loading ? (
            <div className='text-muted-foreground flex items-center justify-center py-12 text-sm'>
              {t('Loading...')}
            </div>
          ) : parsed ? (
            <div className='space-y-4 p-4'>
              <div className='space-y-3 rounded-xl border p-4'>
                <div className='text-sm font-semibold'>{t('Request Info')}</div>
                <div className='grid gap-3 md:grid-cols-2'>
                  <PreviewInfoBlock label='Channel ID' value={parsed.channel?.id ?? '-'} />
                  <PreviewInfoBlock label='Channel Type' value={parsed.channel?.type ?? '-'} />
                  <PreviewInfoBlock label='Base URL' value={parsed.channel?.base_url || '-'} />
                  <PreviewInfoBlock label='Request Path' value={parsed.relay?.request_path || '-'} />
                  <PreviewInfoBlock label='Relay Mode' value={parsed.relay?.relay_mode || '-'} />
                  <PreviewInfoBlock label='Origin Model' value={parsed.channel?.origin_model || '-'} />
                  <PreviewInfoBlock label='Upstream Model' value={parsed.channel?.upstream_model || '-'} />
                  <PreviewInfoBlock
                    label='Client Requested Stream'
                    value={parsed.relay?.client_requested_stream ? 'true' : 'false'}
                  />
                  <PreviewInfoBlock label='Response Mode' value={parsed.relay?.response_mode || '-'} />
                  <PreviewInfoBlock
                    label='Request Conversion Chain'
                    value={parsed.relay?.request_conversion_chain || []}
                  />
                  <PreviewInfoBlock
                    label='Final Request Relay Format'
                    value={parsed.relay?.final_request_relay_format || '-'}
                  />
                </div>
              </div>
              <div className='grid gap-4 xl:grid-cols-2'>
                <PreviewPacketPanel
                  title={t('Downstream Request')}
                  packet={parsed.downstream_request}
                />
                <PreviewPacketPanel
                  title={t('Upstream Request')}
                  packet={parsed.upstream_request}
                />
              </div>
              <div className='rounded-xl border p-4'>
                <div className='mb-2 text-sm font-semibold'>{t('Raw Payload')}</div>
                <pre className='overflow-x-auto whitespace-pre-wrap break-all text-xs leading-5'>
                  {formatted}
                </pre>
              </div>
            </div>
          ) : (
            <pre className='overflow-x-auto p-4 text-xs leading-5 whitespace-pre-wrap break-all'>
              {formatted || '-'}
            </pre>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

function PreviewActionCell({ log }: { log: PreviewLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <>
      <div className='flex items-center gap-1'>
        <Button
          variant='outline'
          size='sm'
          className='h-7 gap-1 px-2 text-xs'
          onClick={() => setOpen(true)}
        >
          <Eye className='size-3.5' />
          {t('View')}
        </Button>
        {log.payload ? (
          <CopyButton
            value={log.payload}
            variant='ghost'
            size='icon'
            className='size-7'
          />
        ) : null}
      </div>
      <PreviewPayloadDialog open={open} onOpenChange={setOpen} log={log} />
    </>
  )
}

export function usePreviewLogsColumns(
  isAdmin: boolean
): ColumnDef<PreviewLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<PreviewLog>[] = [
    {
      accessorKey: 'created_at',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Time')} />
      ),
      cell: ({ row }) => {
        const timestamp = row.getValue('created_at') as number
        return (
          <span className='font-mono text-xs tabular-nums'>
            {formatTimestampToDate(timestamp)}
          </span>
        )
      },
      meta: { label: t('Time') },
    },
  ]

  if (isAdmin) {
    columns.push(createChannelColumn<PreviewLog>({ headerLabel: t('Channel') }))
    columns.push({
      accessorKey: 'username',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('User')} />
      ),
      cell: ({ row }) => {
        const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
          useUsageLogsContext()
        const log = row.original
        return (
          <button
            type='button'
            className='text-muted-foreground truncate text-left text-sm hover:underline'
            onClick={(e) => {
              e.stopPropagation()
              setSelectedUserId(log.user_id)
              setUserInfoDialogOpen(true)
            }}
          >
            {sensitiveVisible ? log.username || String(log.user_id) : '••••'}
          </button>
        )
      },
      meta: { label: t('User') },
    })
  }

  columns.push(
    {
      accessorKey: 'request_path',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Request Path')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        return (
          <div className='flex min-w-0 max-w-[240px] flex-col gap-0.5'>
            <span className='truncate font-mono text-xs'>{log.request_path}</span>
            <span className='text-muted-foreground truncate text-[11px]'>
              {log.relay_mode}
            </span>
          </div>
        )
      },
      meta: { label: t('Request Path'), mobileTitle: true },
    },
    {
      id: 'models',
      accessorFn: (row) => row.origin_model_name || row.upstream_model_name,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Model')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        const sameModel =
          log.origin_model_name === '' ||
          log.origin_model_name === log.upstream_model_name
        return (
          <div className='flex max-w-[240px] flex-col gap-1'>
            <StatusBadge
              label={log.origin_model_name || '-'}
              size='sm'
              copyable={false}
            />
            <div className='text-muted-foreground flex items-center gap-1 text-[11px]'>
              <Route className='size-3' />
              <span className='truncate'>
                {sameModel ? (log.upstream_model_name || '-') : `${log.upstream_model_name || '-'} `}
              </span>
            </div>
          </div>
        )
      },
      meta: { label: t('Model') },
    },
    {
      accessorKey: 'request_id',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Request ID')} />
      ),
      cell: ({ row }) => {
        const requestId = row.getValue('request_id') as string
        if (!requestId) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return (
          <div className='flex items-center gap-1'>
            <span className='max-w-[180px] truncate font-mono text-xs'>
              {requestId}
            </span>
            <CopyButton
              value={requestId}
              variant='ghost'
              size='icon'
              className='size-6'
            />
          </div>
        )
      },
      meta: { label: t('Request ID') },
    },
    {
      accessorKey: 'upstream_url',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Upstream URL')} />
      ),
      cell: ({ row }) => {
        const upstreamURL = row.getValue('upstream_url') as string
        return (
          <div className='flex min-w-0 max-w-[280px] items-center gap-1'>
            <span className='truncate font-mono text-xs'>{upstreamURL || '-'}</span>
            {upstreamURL ? (
              <CopyButton
                value={upstreamURL}
                variant='ghost'
                size='icon'
                className='size-6'
              />
            ) : null}
          </div>
        )
      },
      meta: { label: t('Upstream URL') },
    },
    {
      accessorKey: 'client_requested_stream',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Stream')} />
      ),
      cell: ({ row }) => {
        const requestedStream = row.getValue(
          'client_requested_stream'
        ) as boolean
        return (
          <StatusBadge
            label={requestedStream ? t('Requested') : t('Disabled')}
            variant={requestedStream ? 'info' : 'secondary'}
            size='sm'
            copyable={false}
          />
        )
      },
      meta: { label: t('Stream'), mobileBadge: true },
    },
    {
      id: 'payload',
      accessorFn: (row) => row.payload,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Preview')} />
      ),
      cell: ({ row }) => <PreviewActionCell log={row.original} />,
      meta: { label: t('Preview') },
    }
  )

  return columns
}
