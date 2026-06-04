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
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconChevronDown,
  IconChevronUp,
  IconCopy,
  IconDelete,
  IconPlus,
} from '@douyinfe/semi-icons';
import { showError } from '../../../../helpers';
import ParamOverrideEditorModal from './ParamOverrideEditorModal';

const { Text } = Typography;

const CONDITION_MODE_OPTIONS = [
  { label: '完全匹配', value: 'full' },
  { label: '前缀匹配', value: 'prefix' },
  { label: '后缀匹配', value: 'suffix' },
  { label: '包含', value: 'contains' },
  { label: '大于', value: 'gt' },
  { label: '大于等于', value: 'gte' },
  { label: '小于', value: 'lt' },
  { label: '小于等于', value: 'lte' },
];

const UNSUPPORTED_OPERATION_MODES = new Set([
  'set_header',
  'delete_header',
  'copy_header',
  'move_header',
  'pass_headers',
]);

let localIdSeed = 0;
const nextLocalId = () => `vmr_${Date.now()}_${localIdSeed++}`;

const toValueText = (value) => {
  if (value === undefined) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch (error) {
    return String(value);
  }
};

const parseLooseValue = (valueText) => {
  const raw = String(valueText || '').trim();
  if (!raw) return '';
  try {
    return JSON.parse(raw);
  } catch (error) {
    return raw;
  }
};

const createDefaultCondition = () => ({
  id: nextLocalId(),
  path: '',
  mode: 'full',
  value_text: '',
  invert: false,
  pass_missing_key: false,
});

const createDefaultRule = () => ({
  id: nextLocalId(),
  name: '',
  enabled: true,
  logic: 'AND',
  target_model: '',
  conditions: [createDefaultCondition()],
  operations: [],
});

const normalizeCondition = (condition = {}) => ({
  id: nextLocalId(),
  path: typeof condition.path === 'string' ? condition.path : '',
  mode: typeof condition.mode === 'string' ? condition.mode : 'full',
  value_text: toValueText(condition.value),
  invert: condition.invert === true,
  pass_missing_key: condition.pass_missing_key === true,
});

const normalizeRule = (rule = {}) => ({
  id: nextLocalId(),
  name: typeof rule.name === 'string' ? rule.name : '',
  enabled: rule.enabled !== false,
  logic: String(rule.logic || 'AND').toUpperCase() === 'OR' ? 'OR' : 'AND',
  target_model: typeof rule.target_model === 'string' ? rule.target_model : '',
  conditions: Array.isArray(rule.conditions)
    ? rule.conditions.map(normalizeCondition)
    : [createDefaultCondition()],
  operations: Array.isArray(rule.operations) ? rule.operations : [],
});

const buildRulePayload = (rule) => ({
  name: rule.name.trim() || undefined,
  enabled: rule.enabled,
  logic: rule.logic || 'AND',
  target_model: rule.target_model.trim(),
  conditions: rule.conditions
    .map((condition) => {
      const path = String(condition.path || '').trim();
      const valueText = String(condition.value_text || '').trim();
      if (!path || !valueText) return null;
      return {
        path,
        mode: condition.mode || 'full',
        value: parseLooseValue(valueText),
        invert: condition.invert || undefined,
        pass_missing_key: condition.pass_missing_key || undefined,
      };
    })
    .filter(Boolean),
  operations: Array.isArray(rule.operations) ? rule.operations : [],
});

const stringifyRuleSet = (rules) =>
  JSON.stringify(
    {
      version: 1,
      rules: rules.map(buildRulePayload),
    },
    null,
    2,
  );

const parseRuleSet = (value) => {
  if (!value || !value.trim()) {
    return { version: 1, rules: [] };
  }
  const parsed = JSON.parse(value);
  return {
    version:
      typeof parsed.version === 'number' && parsed.version > 0
        ? parsed.version
        : 1,
    rules: Array.isArray(parsed.rules) ? parsed.rules : [],
  };
};

const summarizeOperations = (operations) => {
  if (!Array.isArray(operations) || operations.length === 0) {
    return '无操作';
  }
  return operations
    .slice(0, 2)
    .map((operation) => {
      const mode = String(operation.mode || 'set');
      const path = String(
        operation.path || operation.from || operation.to || '-',
      ).trim();
      return `${mode}:${path || '-'}`;
    })
    .join(' · ');
};

const getUnsupportedOperationMode = (operations) => {
  if (!Array.isArray(operations)) return null;
  for (const operation of operations) {
    const mode = String(operation.mode || '').trim().toLowerCase();
    if (UNSUPPORTED_OPERATION_MODES.has(mode)) {
      return mode;
    }
  }
  return null;
};

export default function VideoModelRoutingEditorModal({
  visible,
  value,
  onCancel,
  onSave,
}) {
  const { t } = useTranslation();
  const [rules, setRules] = useState([]);
  const [rawJson, setRawJson] = useState('');
  const [viewMode, setViewMode] = useState('visual');
  const [operationsEditorIndex, setOperationsEditorIndex] = useState(null);

  useEffect(() => {
    if (!visible) return;
    try {
      const parsed = parseRuleSet(value);
      setRules(parsed.rules.map(normalizeRule));
      setRawJson(JSON.stringify(parsed, null, 2));
      setViewMode('visual');
    } catch (error) {
      setRules([]);
      setRawJson(value || '');
      setViewMode('json');
    }
  }, [visible, value]);

  const visualJson = useMemo(() => stringifyRuleSet(rules), [rules]);

  const updateRule = (ruleId, updater) => {
    setRules((current) =>
      current.map((rule) => (rule.id === ruleId ? updater(rule) : rule)),
    );
  };

  const moveRule = (index, direction) => {
    setRules((current) => {
      const target = index + direction;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  const handleSave = () => {
    const finalJson = viewMode === 'visual' ? visualJson : rawJson;
    let parsed;
    try {
      parsed = parseRuleSet(finalJson);
    } catch (error) {
      showError(t('视频模型路由规则 JSON 不合法'));
      return;
    }
    for (const rule of parsed.rules) {
      if (!String(rule.target_model || '').trim()) {
        showError(t('每条路由规则都必须填写目标上游模型'));
        return;
      }
      const blockedMode = getUnsupportedOperationMode(rule.operations);
      if (blockedMode) {
        showError(
          t('操作模式 {{mode}} 不支持用于视频路由规则', {
            mode: blockedMode,
          }),
        );
        return;
      }
    }
    onSave(JSON.stringify(parsed, null, 2));
  };

  const operationsEditorValue =
    operationsEditorIndex === null
      ? ''
      : JSON.stringify(
          {
            operations: rules[operationsEditorIndex]?.operations || [],
          },
          null,
          2,
        );

  return (
    <>
      <Modal
        title={t('视频模型路由规则')}
        visible={visible}
        onCancel={onCancel}
        onOk={handleSave}
        width={1100}
        style={{ maxWidth: '96vw' }}
      >
        <Space className='mb-3'>
          <Button
            type={viewMode === 'visual' ? 'primary' : 'tertiary'}
            onClick={() => setViewMode('visual')}
          >
            {t('可视化')}
          </Button>
          <Button
            type={viewMode === 'json' ? 'primary' : 'tertiary'}
            onClick={() => {
              setRawJson(visualJson);
              setViewMode('json');
            }}
          >
            JSON
          </Button>
          {viewMode === 'json' && (
            <Button
              type='tertiary'
              onClick={() => {
                try {
                  const parsed = parseRuleSet(rawJson);
                  setRules(parsed.rules.map(normalizeRule));
                  setRawJson(JSON.stringify(parsed, null, 2));
                  setViewMode('visual');
                } catch (error) {
                  showError(t('视频模型路由规则 JSON 不合法'));
                }
              }}
            >
              {t('应用到可视化')}
            </Button>
          )}
        </Space>

        {viewMode === 'json' ? (
          <TextArea
            autosize={{ minRows: 18, maxRows: 28 }}
            value={rawJson}
            onChange={setRawJson}
          />
        ) : (
          <div className='space-y-3 max-h-[65vh] overflow-y-auto pr-1'>
            <div className='flex items-center justify-between'>
              <Text type='tertiary'>
                {t('按顺序从上到下匹配，第一条命中即停止。')}
              </Text>
              <Button
                type='tertiary'
                icon={<IconPlus size={14} />}
                onClick={() =>
                  setRules((current) => [...current, createDefaultRule()])
                }
              >
                {t('新增规则')}
              </Button>
            </div>

            {rules.length === 0 && (
              <Card>
                <Text type='tertiary'>
                  {t('还没有任何路由规则，请先新增一条。')}
                </Text>
              </Card>
            )}

            {rules.map((rule, index) => (
              <Card
                key={rule.id}
                title={`${index + 1}. ${rule.name || t('未命名规则')}`}
                headerExtraContent={
                  <Space>
                    <Text>{t('启用')}</Text>
                    <Switch
                      checked={rule.enabled}
                      onChange={(checked) =>
                        updateRule(rule.id, (current) => ({
                          ...current,
                          enabled: checked,
                        }))
                      }
                    />
                    <Button
                      theme='borderless'
                      icon={<IconCopy size={14} />}
                      onClick={() =>
                        setRules((current) => [
                          ...current,
                          normalizeRule(buildRulePayload(rule)),
                        ])
                      }
                    />
                    <Button
                      theme='borderless'
                      icon={<IconChevronUp size={14} />}
                      disabled={index === 0}
                      onClick={() => moveRule(index, -1)}
                    />
                    <Button
                      theme='borderless'
                      icon={<IconChevronDown size={14} />}
                      disabled={index === rules.length - 1}
                      onClick={() => moveRule(index, 1)}
                    />
                    <Button
                      theme='borderless'
                      icon={<IconDelete size={14} />}
                      onClick={() =>
                        setRules((current) =>
                          current.filter((item) => item.id !== rule.id),
                        )
                      }
                    />
                  </Space>
                }
              >
                <Row gutter={12}>
                  <Col span={12}>
                    <Input
                      value={rule.name}
                      onChange={(nextValue) =>
                        updateRule(rule.id, (current) => ({
                          ...current,
                          name: nextValue,
                        }))
                      }
                      placeholder={t('规则名称')}
                    />
                  </Col>
                  <Col span={12}>
                    <Input
                      value={rule.target_model}
                      onChange={(nextValue) =>
                        updateRule(rule.id, (current) => ({
                          ...current,
                          target_model: nextValue,
                        }))
                      }
                      placeholder='sora-2-8s-16x9'
                    />
                  </Col>
                </Row>

                <div className='mt-3'>
                  <Text className='mb-2 block'>{t('条件逻辑')}</Text>
                  <Select
                    value={rule.logic}
                    optionList={[
                      { label: 'AND', value: 'AND' },
                      { label: 'OR', value: 'OR' },
                    ]}
                    onChange={(nextValue) =>
                      updateRule(rule.id, (current) => ({
                        ...current,
                        logic: nextValue === 'OR' ? 'OR' : 'AND',
                      }))
                    }
                  />
                </div>

                <div className='mt-4'>
                  <div className='mb-2 flex items-center justify-between'>
                    <Text>{t('条件')}</Text>
                    <Button
                      size='small'
                      type='tertiary'
                      icon={<IconPlus size={14} />}
                      onClick={() =>
                        updateRule(rule.id, (current) => ({
                          ...current,
                          conditions: [
                            ...current.conditions,
                            createDefaultCondition(),
                          ],
                        }))
                      }
                    >
                      {t('新增条件')}
                    </Button>
                  </div>
                  <Space vertical style={{ width: '100%' }}>
                    {rule.conditions.map((condition) => (
                      <Card key={condition.id} bodyStyle={{ padding: 12 }}>
                        <Row gutter={12}>
                          <Col span={8}>
                            <Input
                              value={condition.path}
                              onChange={(nextValue) =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  conditions: current.conditions.map((item) =>
                                    item.id === condition.id
                                      ? { ...item, path: nextValue }
                                      : item,
                                  ),
                                }))
                              }
                              placeholder='derived.original_model'
                            />
                          </Col>
                          <Col span={6}>
                            <Select
                              value={condition.mode}
                              optionList={CONDITION_MODE_OPTIONS}
                              onChange={(nextValue) =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  conditions: current.conditions.map((item) =>
                                    item.id === condition.id
                                      ? { ...item, mode: nextValue }
                                      : item,
                                  ),
                                }))
                              }
                            />
                          </Col>
                          <Col span={6}>
                            <Input
                              value={condition.value_text}
                              onChange={(nextValue) =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  conditions: current.conditions.map((item) =>
                                    item.id === condition.id
                                      ? { ...item, value_text: nextValue }
                                      : item,
                                  ),
                                }))
                              }
                              placeholder='sora-2'
                            />
                          </Col>
                          <Col span={4}>
                            <Space>
                              <Switch
                                checked={condition.invert}
                                onChange={(checked) =>
                                  updateRule(rule.id, (current) => ({
                                    ...current,
                                    conditions: current.conditions.map((item) =>
                                      item.id === condition.id
                                        ? { ...item, invert: checked }
                                        : item,
                                    ),
                                  }))
                                }
                              />
                              <Text size='small'>NOT</Text>
                            </Space>
                          </Col>
                        </Row>
                        <div className='mt-2 flex items-center justify-between'>
                          <Space>
                            <Switch
                              checked={condition.pass_missing_key}
                              onChange={(checked) =>
                                updateRule(rule.id, (current) => ({
                                  ...current,
                                  conditions: current.conditions.map((item) =>
                                    item.id === condition.id
                                      ? {
                                          ...item,
                                          pass_missing_key: checked,
                                        }
                                      : item,
                                  ),
                                }))
                              }
                            />
                            <Text size='small'>{t('缺失时放行')}</Text>
                          </Space>
                          <Button
                            size='small'
                            type='danger'
                            theme='borderless'
                            icon={<IconDelete size={14} />}
                            disabled={rule.conditions.length === 1}
                            onClick={() =>
                              updateRule(rule.id, (current) => ({
                                ...current,
                                conditions: current.conditions.filter(
                                  (item) => item.id !== condition.id,
                                ),
                              }))
                            }
                          />
                        </div>
                      </Card>
                    ))}
                  </Space>
                </div>

                <div className='mt-4 rounded-xl border border-dashed p-3'>
                  <div className='flex items-center justify-between gap-2'>
                    <div>
                      <Text className='block'>{t('请求体操作')}</Text>
                      <Text type='tertiary' size='small'>
                        {t('命中规则后再修改请求体字段，请求头操作会被拦截。')}
                      </Text>
                    </div>
                    <Space>
                      <Button
                        size='small'
                        type='primary'
                        onClick={() => setOperationsEditorIndex(index)}
                      >
                        {t('编辑操作')}
                      </Button>
                      <Button
                        size='small'
                        type='tertiary'
                        disabled={!rule.operations.length}
                        onClick={() =>
                          updateRule(rule.id, (current) => ({
                            ...current,
                            operations: [],
                          }))
                        }
                      >
                        {t('清空')}
                      </Button>
                    </Space>
                  </div>
                  <Text type='tertiary' size='small'>
                    {summarizeOperations(rule.operations)}
                  </Text>
                </div>
              </Card>
            ))}
          </div>
        )}
      </Modal>

      <ParamOverrideEditorModal
        visible={operationsEditorIndex !== null}
        value={operationsEditorValue}
        onCancel={() => setOperationsEditorIndex(null)}
        onSave={(nextValue) => {
          try {
            const parsed = JSON.parse(nextValue);
            const operations = Array.isArray(parsed.operations)
              ? parsed.operations
              : [];
            const blockedMode = getUnsupportedOperationMode(operations);
            if (blockedMode) {
              showError(
                t('操作模式 {{mode}} 不支持用于视频路由规则', {
                  mode: blockedMode,
                }),
              );
              return;
            }
            setRules((current) =>
              current.map((rule, index) =>
                index === operationsEditorIndex
                  ? { ...rule, operations }
                  : rule,
              ),
            );
            setOperationsEditorIndex(null);
          } catch (error) {
            showError(t('操作 JSON 不合法'));
          }
        }}
      />
    </>
  );
}
