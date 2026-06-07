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
import { useState, useEffect, useCallback } from 'react'
import { useQueryClient, useIsFetching } from '@tanstack/react-query'
import { useNavigate, getRouteApi } from '@tanstack/react-router'
import { type Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { useIsAdmin } from '@/hooks/use-admin'
import { buildSearchParams } from '../lib/filter'
import { getDefaultTimeRange } from '../lib/utils'
import type { PreviewLogFilters } from '../types'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from './logs-filter-toolbar'

const route = getRouteApi('/_authenticated/usage-logs/$section')

interface PreviewLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function PreviewLogsFilterBar<TData>(
  props: PreviewLogsFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const isAdmin = useIsAdmin()
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })

  const [filters, setFilters] = useState<PreviewLogFilters>(() => {
    const { start, end } = getDefaultTimeRange()
    return { startTime: start, endTime: end }
  })

  useEffect(() => {
    const { start, end } = getDefaultTimeRange()
    setFilters({
      startTime: searchParams.startTime
        ? new Date(searchParams.startTime)
        : start,
      endTime: searchParams.endTime ? new Date(searchParams.endTime) : end,
      channel: searchParams.channel || undefined,
      model: searchParams.model || undefined,
      username: searchParams.username || undefined,
      requestId: searchParams.requestId || undefined,
      path: searchParams.path || undefined,
    })
  }, [
    searchParams.startTime,
    searchParams.endTime,
    searchParams.channel,
    searchParams.model,
    searchParams.username,
    searchParams.requestId,
    searchParams.path,
  ])

  const handleChange = useCallback(
    (field: keyof PreviewLogFilters, value: Date | string | undefined) => {
      setFilters((prev) => ({ ...prev, [field]: value }))
    },
    []
  )

  const handleSearch = useCallback(() => {
    const filterParams = buildSearchParams(filters, 'preview')
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'preview' },
      search: {
        ...filterParams,
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [filters, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    const resetFilters: PreviewLogFilters = {
      startTime: start,
      endTime: end,
    }
    setFilters(resetFilters)
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'preview' },
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [navigate, queryClient])

  const hasActiveFilters =
    !!filters.model ||
    !!filters.username ||
    !!filters.requestId ||
    !!filters.path ||
    !!filters.channel

  return (
    <LogsFilterToolbar
      table={props.table}
      primaryFilters={
        <>
          <LogsFilterField wide>
            <CompactDateTimeRangePicker
              start={filters.startTime}
              end={filters.endTime}
              onChange={({ start, end }) => {
                handleChange('startTime', start)
                handleChange('endTime', end)
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Model Name')}
              value={filters.model || ''}
              onChange={(e) => handleChange('model', e.target.value)}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Request ID')}
              value={filters.requestId || ''}
              onChange={(e) => handleChange('requestId', e.target.value)}
            />
          </LogsFilterField>
        </>
      }
      advancedFilters={
        <>
          <LogsFilterField>
            <LogsFilterInput
              placeholder={t('Request Path')}
              value={filters.path || ''}
              onChange={(e) => handleChange('path', e.target.value)}
            />
          </LogsFilterField>
          {isAdmin && (
            <>
              <LogsFilterField>
                <LogsFilterInput
                  placeholder={t('Channel ID')}
                  value={filters.channel || ''}
                  onChange={(e) => handleChange('channel', e.target.value)}
                />
              </LogsFilterField>
              <LogsFilterField>
                <LogsFilterInput
                  placeholder={t('Username')}
                  value={filters.username || ''}
                  onChange={(e) => handleChange('username', e.target.value)}
                />
              </LogsFilterField>
            </>
          )}
        </>
      }
      hasActiveFilters={hasActiveFilters}
      hasAdvancedActiveFilters={
        !!filters.path || (!!isAdmin && (!!filters.channel || !!filters.username))
      }
      advancedFilterCount={[
        filters.path,
        isAdmin ? filters.channel : undefined,
        isAdmin ? filters.username : undefined,
      ].filter(Boolean).length}
      searchLoading={fetchingLogs > 0}
      onReset={handleReset}
      onSearch={handleSearch}
    />
  )
}
