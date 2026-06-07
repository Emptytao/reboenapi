/*
Copyright (C) 2025 QuantumNous

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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Empty,
  Form,
  Modal,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import { API, copy, getTodayStartTimestamp, isAdmin, showError, showSuccess, timestamp2string } from '../../../helpers';
import { ITEMS_PER_PAGE } from '../../../constants';
import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { useTableCompactMode } from '../../../hooks/common/useTableCompactMode';
import { createCardProPagination } from '../../../helpers/utils';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

function prettyPreviewPayload(payload) {
  if (!payload) return '';
  try {
    return JSON.stringify(JSON.parse(payload), null, 2);
  } catch {
    return payload;
  }
}

const PreviewLogsPage = () => {
  const { t } = useTranslation();
  const isAdminUser = isAdmin();
  const isMobile = useIsMobile();
  const [compactMode, setCompactMode] = useTableCompactMode('requestPreviewLogs');
  const [formApi, setFormApi] = useState(null);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState([]);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [logCount, setLogCount] = useState(0);
  const [previewModalOpen, setPreviewModalOpen] = useState(false);
  const [previewPayload, setPreviewPayload] = useState('');

  const now = new Date();
  const formInitValues = {
    model_name: '',
    request_id: '',
    request_path: '',
    channel: '',
    username: '',
    dateRange: [
      timestamp2string(getTodayStartTimestamp()),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const getFormValues = useCallback(() => {
    const formValues = formApi ? formApi.getValues() : {};
    let startTimestamp = timestamp2string(getTodayStartTimestamp());
    let endTimestamp = timestamp2string(now.getTime() / 1000 + 3600);

    if (
      formValues.dateRange &&
      Array.isArray(formValues.dateRange) &&
      formValues.dateRange.length === 2
    ) {
      startTimestamp = formValues.dateRange[0];
      endTimestamp = formValues.dateRange[1];
    }

    return {
      model_name: formValues.model_name || '',
      request_id: formValues.request_id || '',
      request_path: formValues.request_path || '',
      channel: formValues.channel || '',
      username: formValues.username || '',
      start_timestamp: startTimestamp,
      end_timestamp: endTimestamp,
    };
  }, [formApi, now]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const values = getFormValues();
      const query = new URLSearchParams({
        p: String(activePage),
        page_size: String(pageSize),
      });

      Object.entries(values).forEach(([key, value]) => {
        if (value) {
          if (key === 'start_timestamp' || key === 'end_timestamp') {
            query.append(key, String(Date.parse(value) / 1000));
          } else {
            query.append(key, String(value));
          }
        }
      });

      const path = isAdminUser
        ? `/api/preview-log/?${query.toString()}`
        : `/api/preview-log/self?${query.toString()}`;
      const res = await API.get(path);
      if (res.data.success) {
        setLogs(res.data.data.items || []);
        setLogCount(res.data.data.total || 0);
      } else {
        showError(res.data.message || t('获取日志失败'));
      }
    } catch (error) {
      showError(t('获取日志失败'));
    } finally {
      setLoading(false);
    }
  }, [activePage, getFormValues, isAdminUser, pageSize, t]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const openPreview = useCallback((payload) => {
    setPreviewPayload(prettyPreviewPayload(payload));
    setPreviewModalOpen(true);
  }, []);

  const columns = useMemo(() => {
    const baseColumns = [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        render: (value) => (
          <span className='font-mono text-xs'>
            {timestamp2string(value)}
          </span>
        ),
      },
      {
        title: t('请求路径'),
        dataIndex: 'request_path',
        key: 'request_path',
        render: (value, record) => (
          <div className='flex flex-col items-start'>
            <Text className='font-mono text-xs'>{value || '-'}</Text>
            <Text type='secondary' size='small'>
              {record.relay_mode || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('模型'),
        dataIndex: 'origin_model_name',
        key: 'model',
        render: (_, record) => (
          <div className='flex flex-col items-start gap-1'>
            <Tag color='blue' shape='circle'>
              {record.origin_model_name || '-'}
            </Tag>
            <Text type='secondary' size='small'>
              {t('转发模型')}：{record.upstream_model_name || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('Request ID'),
        dataIndex: 'request_id',
        key: 'request_id',
        render: (value) => (
          <div className='flex items-center gap-1 justify-end'>
            <Text className='font-mono text-xs'>{value || '-'}</Text>
            {value ? (
              <Button
                type='tertiary'
                size='small'
                onClick={() => {
                  copy(value);
                  showSuccess(t('已复制到剪贴板'));
                }}
              >
                {t('复制')}
              </Button>
            ) : null}
          </div>
        ),
      },
      {
        title: t('上游地址'),
        dataIndex: 'upstream_url',
        key: 'upstream_url',
        render: (value) => (
          <div className='flex items-center gap-1 justify-end'>
            <Text className='font-mono text-xs'>{value || '-'}</Text>
            {value ? (
              <Button
                type='tertiary'
                size='small'
                onClick={() => {
                  copy(value);
                  showSuccess(t('已复制到剪贴板'));
                }}
              >
                {t('复制')}
              </Button>
            ) : null}
          </div>
        ),
      },
      {
        title: t('客户端流式'),
        dataIndex: 'client_requested_stream',
        key: 'client_requested_stream',
        render: (value) => (
          <Tag color={value ? 'green' : 'grey'} shape='circle'>
            {value ? t('已请求') : t('未请求')}
          </Tag>
        ),
      },
      {
        title: t('预览'),
        dataIndex: 'payload',
        key: 'payload',
        render: (value) => (
          <Space>
            <Button type='primary' size='small' onClick={() => openPreview(value)}>
              {t('查看预览')}
            </Button>
            <Button
              type='tertiary'
              size='small'
              onClick={() => {
                copy(prettyPreviewPayload(value));
                showSuccess(t('已复制到剪贴板'));
              }}
            >
              {t('复制')}
            </Button>
          </Space>
        ),
      },
    ];

    if (isAdminUser) {
      baseColumns.splice(1, 0, {
        title: t('渠道'),
        dataIndex: 'channel_id',
        key: 'channel_id',
        render: (value, record) => (
          <div className='flex flex-col items-start'>
            <Text>{record.channel_name || `#${value}`}</Text>
            <Text type='secondary' size='small'>
              ID: {value}
            </Text>
          </div>
        ),
      });
      baseColumns.splice(2, 0, {
        title: t('用户名称'),
        dataIndex: 'username',
        key: 'username',
      });
    }

    return compactMode
      ? baseColumns.map(({ fixed, ...rest }) => rest)
      : baseColumns;
  }, [compactMode, isAdminUser, openPreview, t]);

  return (
    <>
      <Modal
        title={t('请求预览详情')}
        visible={previewModalOpen}
        onCancel={() => setPreviewModalOpen(false)}
        footer={
          <Space>
            <Button
              type='tertiary'
              onClick={() => {
                copy(previewPayload);
                showSuccess(t('已复制到剪贴板'));
              }}
            >
              {t('复制预览')}
            </Button>
            <Button type='primary' onClick={() => setPreviewModalOpen(false)}>
              {t('关闭')}
            </Button>
          </Space>
        }
        style={{ width: isMobile ? '94vw' : 960 }}
        bodyStyle={{ paddingTop: 8 }}
      >
        <pre
          className='max-h-[65vh] overflow-auto rounded-xl p-3 text-xs leading-6'
          style={{
            background: 'var(--semi-color-fill-0)',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {previewPayload}
        </pre>
      </Modal>

      <CardPro
        type='type2'
        statsArea={
          <div className='flex justify-end'>
            <CompactModeToggle
              compactMode={compactMode}
              setCompactMode={setCompactMode}
              t={t}
            />
          </div>
        }
        searchArea={
          <Form
            initValues={formInitValues}
            getFormApi={(api) => setFormApi(api)}
            onSubmit={() => {
              setActivePage(1);
              setTimeout(() => refresh(), 0);
            }}
            allowEmpty={true}
            autoComplete='off'
            layout='vertical'
            trigger='change'
            stopValidateWithError={false}
          >
            <div className='flex flex-col gap-2'>
              <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
                <div className='col-span-1 lg:col-span-2'>
                  <Form.DatePicker
                    field='dateRange'
                    className='w-full'
                    type='dateTimeRange'
                    placeholder={[t('开始时间'), t('结束时间')]}
                    showClear
                    pure
                    size='small'
                    presets={DATE_RANGE_PRESETS.map((preset) => ({
                      text: t(preset.text),
                      start: preset.start(),
                      end: preset.end(),
                    }))}
                  />
                </div>
                <Form.Input
                  field='model_name'
                  prefix={<IconSearch />}
                  placeholder={t('模型名称')}
                  showClear
                  pure
                  size='small'
                />
                <Form.Input
                  field='request_id'
                  prefix={<IconSearch />}
                  placeholder={t('Request ID')}
                  showClear
                  pure
                  size='small'
                />
                <Form.Input
                  field='request_path'
                  prefix={<IconSearch />}
                  placeholder={t('请求路径')}
                  showClear
                  pure
                  size='small'
                />
                {isAdminUser && (
                  <>
                    <Form.Input
                      field='channel'
                      prefix={<IconSearch />}
                      placeholder={t('渠道 ID')}
                      showClear
                      pure
                      size='small'
                    />
                    <Form.Input
                      field='username'
                      prefix={<IconSearch />}
                      placeholder={t('用户名称')}
                      showClear
                      pure
                      size='small'
                    />
                  </>
                )}
              </div>
              <div className='flex gap-2 w-full justify-end'>
                <Button type='tertiary' htmlType='submit' loading={loading} size='small'>
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  size='small'
                  onClick={() => {
                    if (formApi) {
                      formApi.reset();
                      setActivePage(1);
                      setTimeout(() => refresh(), 100);
                    }
                  }}
                >
                  {t('重置')}
                </Button>
              </div>
            </div>
          </Form>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize: pageSize,
          total: logCount,
          onPageChange: (page) => setActivePage(page),
          onPageSizeChange: (size) => {
            setPageSize(size);
            setActivePage(1);
          },
          isMobile: isMobile,
          t: t,
        })}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={logs}
          rowKey='id'
          loading={loading}
          scroll={compactMode ? undefined : { x: 'max-content' }}
          className='rounded-xl overflow-hidden'
          size='small'
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无请求预览日志')}
              style={{ padding: 30 }}
            />
          }
          pagination={{
            currentPage: activePage,
            pageSize: pageSize,
            total: logCount,
            pageSizeOptions: [10, 20, 50, 100],
            showSizeChanger: true,
            onPageSizeChange: (size) => {
              setPageSize(size);
              setActivePage(1);
            },
            onPageChange: (page) => setActivePage(page),
          }}
          hidePagination={true}
        />
      </CardPro>
    </>
  );
};

export default PreviewLogsPage;
