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

import React, { useEffect, useMemo, useState } from 'react';
import { Layout, TabPane, Tabs } from '@douyinfe/semi-ui';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import UsageLogsPage from '../../components/table/usage-logs';
import DrawingLogsPage from '../../components/table/mj-logs';
import PreviewLogsPage from '../../components/table/preview-logs';
import TaskLogsPage from '../../components/table/task-logs';
import { useSidebar } from '../../hooks/common/useSidebar';
import { isAdmin } from '../../helpers';

const LOG_TABS = {
  usage: 'usage',
  drawing: 'drawing',
  task: 'task',
  preview: 'preview',
};

const Log = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { isModuleVisible } = useSidebar();
  const [activeTab, setActiveTab] = useState(LOG_TABS.usage);

  const drawingEnabled = localStorage.getItem('enable_drawing') === 'true';
  const taskEnabled = localStorage.getItem('enable_task') === 'true';
  const isAdminUser = isAdmin();

  const tabs = useMemo(
    () =>
      [
        {
          itemKey: LOG_TABS.usage,
          tab: t('使用日志'),
          visible: isModuleVisible('console', 'log'),
          content: <UsageLogsPage />,
        },
        {
          itemKey: LOG_TABS.drawing,
          tab: t('绘图日志'),
          visible: drawingEnabled && isModuleVisible('console', 'midjourney'),
          content: <DrawingLogsPage />,
        },
        {
          itemKey: LOG_TABS.task,
          tab: t('任务日志'),
          visible: taskEnabled && isModuleVisible('console', 'task'),
          content: <TaskLogsPage />,
        },
        {
          itemKey: LOG_TABS.preview,
          tab: t('请求预览日志'),
          visible: isAdminUser && isModuleVisible('console', 'log'),
          content: <PreviewLogsPage />,
        },
      ].filter((tab) => tab.visible),
    [drawingEnabled, isAdminUser, isModuleVisible, t, taskEnabled],
  );

  useEffect(() => {
    if (tabs.length === 0) {
      return;
    }

    const searchParams = new URLSearchParams(location.search);
    const requestedTab = searchParams.get('tab');
    const nextTab = tabs.some((tab) => tab.itemKey === requestedTab)
      ? requestedTab
      : tabs[0].itemKey;

    if (nextTab && activeTab !== nextTab) {
      setActiveTab(nextTab);
    }

    if (requestedTab !== nextTab) {
      navigate(`/console/log?tab=${nextTab}`, { replace: true });
    }
  }, [activeTab, location.search, navigate, tabs]);

  const handleTabChange = (key) => {
    setActiveTab(key);
    navigate(`/console/log?tab=${key}`);
  };

  return (
    <div className='mt-[60px] px-2'>
      <Layout>
        <Layout.Content>
          {tabs.length > 1 ? (
            <Tabs
              type='card'
              collapsible
              activeKey={activeTab}
              onChange={handleTabChange}
            >
              {tabs.map((tab) => (
                <TabPane tab={tab.tab} itemKey={tab.itemKey} key={tab.itemKey}>
                  {activeTab === tab.itemKey ? tab.content : null}
                </TabPane>
              ))}
            </Tabs>
          ) : (
            (tabs[0]?.content ?? <UsageLogsPage />)
          )}
        </Layout.Content>
      </Layout>
    </div>
  );
};

export default Log;
