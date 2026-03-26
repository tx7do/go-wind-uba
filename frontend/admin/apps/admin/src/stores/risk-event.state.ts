import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createRiskEventServiceClient,
  type ubaservicev1_RiskEvent_Status as RiskEvent_Status,
  type ubaservicev1_RiskLevel as RiskLevel,
  type ubaservicev1_RiskType as RiskType,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useRiskEventListStore = defineStore('risk-event-list', () => {
  const service = createRiskEventServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询风险事件列表
   */
  async function listRiskEvent(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await service.List({
      // @ts-ignore proto generated code is error.
      fieldMask,
      orderBy: makeOrderBy(orderBy),
      query: makeQueryString(formValues, userStore.isTenantUser()),
      page: paging?.page,
      pageSize: paging?.pageSize,
      noPaging,
    });
  }

  /**
   * 获取风险事件
   */
  async function getRiskEvent(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建风险事件
   */
  async function createRiskEvent(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  function $reset() {}

  return {
    $reset,
    listRiskEvent,
    getRiskEvent,
    createRiskEvent,
  };
});

export const riskEventTypeList = computed(() => [
  { value: 'RISK_TYPE_UNSPECIFIED', label: $t('enum.risk.type.UNSPECIFIED') },
  {
    value: 'RISK_TYPE_LOGIN_ANOMALY',
    label: $t('enum.risk.type.LOGIN_ANOMALY'),
  },
  { value: 'RISK_TYPE_BRUTE_FORCE', label: $t('enum.risk.type.BRUTE_FORCE') },
  {
    value: 'RISK_TYPE_CREDENTIAL_STUFFING',
    label: $t('enum.risk.type.CREDENTIAL_STUFFING'),
  },
  {
    value: 'RISK_TYPE_FREQUENT_OPERATION',
    label: $t('enum.risk.type.FREQUENT_OPERATION'),
  },
  {
    value: 'RISK_TYPE_ABNORMAL_FLOW',
    label: $t('enum.risk.type.ABNORMAL_FLOW'),
  },
  {
    value: 'RISK_TYPE_DATA_EXFILTRATION',
    label: $t('enum.risk.type.DATA_EXFILTRATION'),
  },
  {
    value: 'RISK_TYPE_DEVICE_CHANGE',
    label: $t('enum.risk.type.DEVICE_CHANGE'),
  },
  {
    value: 'RISK_TYPE_LOCATION_ANOMALY',
    label: $t('enum.risk.type.LOCATION_ANOMALY'),
  },
  {
    value: 'RISK_TYPE_PROXY_DETECTED',
    label: $t('enum.risk.type.PROXY_DETECTED'),
  },
  {
    value: 'RISK_TYPE_FRAUD_PAYMENT',
    label: $t('enum.risk.type.FRAUD_PAYMENT'),
  },
  {
    value: 'RISK_TYPE_ABUSE_PROMOTION',
    label: $t('enum.risk.type.ABUSE_PROMOTION'),
  },
]);

const RISK_TYPE_COLOR_MAP = {
  RISK_TYPE_UNSPECIFIED: '#86909C',
  RISK_TYPE_LOGIN_ANOMALY: '#F53F3F',
  RISK_TYPE_BRUTE_FORCE: '#F77234',
  RISK_TYPE_CREDENTIAL_STUFFING: '#FF9A2E',
  RISK_TYPE_FREQUENT_OPERATION: '#4096FF',
  RISK_TYPE_ABNORMAL_FLOW: '#722ED1',
  RISK_TYPE_DATA_EXFILTRATION: '#F53F3F',
  RISK_TYPE_DEVICE_CHANGE: '#F77234',
  RISK_TYPE_LOCATION_ANOMALY: '#FF9A2E',
  RISK_TYPE_PROXY_DETECTED: '#4096FF',
  RISK_TYPE_FRAUD_PAYMENT: '#F53F3F',
  RISK_TYPE_ABUSE_PROMOTION: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function riskEventTypeToColor(type?: RiskType) {
  return (
    RISK_TYPE_COLOR_MAP[type as keyof typeof RISK_TYPE_COLOR_MAP] ||
    RISK_TYPE_COLOR_MAP.DEFAULT
  );
}

export function riskEventTypeToName(type?: RiskType) {
  const values = riskEventTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : '';
}

export const riskLevelList = computed(() => [
  { value: 'RISK_LEVEL_NORMAL', label: $t('enum.riskLevel.RISK_LEVEL_NORMAL') },
  {
    value: 'RISK_LEVEL_SUSPICIOUS',
    label: $t('enum.riskLevel.RISK_LEVEL_SUSPICIOUS'),
  },
  { value: 'RISK_LEVEL_HIGH', label: $t('enum.riskLevel.RISK_LEVEL_HIGH') },
  {
    value: 'RISK_LEVEL_CRITICAL',
    label: $t('enum.riskLevel.RISK_LEVEL_CRITICAL'),
  },
]);

const RISK_LEVEL_COLOR_MAP = {
  RISK_LEVEL_NORMAL: '#00B42A',
  RISK_LEVEL_SUSPICIOUS: '#FF9A2E',
  RISK_LEVEL_HIGH: '#F77234',
  RISK_LEVEL_CRITICAL: '#F53F3F',
  DEFAULT: '#86909C',
} as const;

export function riskLevelToColor(level?: RiskLevel) {
  return (
    RISK_LEVEL_COLOR_MAP[level as keyof typeof RISK_LEVEL_COLOR_MAP] ||
    RISK_LEVEL_COLOR_MAP.DEFAULT
  );
}

export function riskLevelToName(level?: RiskLevel) {
  const values = riskLevelList.value;
  const matchedItem = values.find((item) => item.value === level);
  return matchedItem ? matchedItem.label : '';
}

export const riskEventStatusList = computed(() => [
  { value: 'PENDING', label: $t('enum.risk.event.status.PENDING') },
  { value: 'INVESTIGATING', label: $t('enum.risk.event.status.INVESTIGATING') },
  { value: 'CONFIRMED', label: $t('enum.risk.event.status.CONFIRMED') },
  {
    value: 'FALSE_POSITIVE',
    label: $t('enum.risk.event.status.FALSE_POSITIVE'),
  },
  { value: 'IGNORED', label: $t('enum.risk.event.status.IGNORED') },
  { value: 'AUTO_BLOCKED', label: $t('enum.risk.event.status.AUTO_BLOCKED') },
]);

const RISK_EVENT_STATUS_COLOR_MAP = {
  PENDING: '#FF9A2E',
  INVESTIGATING: '#4096FF',
  CONFIRMED: '#F53F3F',
  FALSE_POSITIVE: '#00B42A',
  IGNORED: '#C9CDD4',
  AUTO_BLOCKED: '#F77234',
  DEFAULT: '#86909C',
} as const;

export function riskEventStatusToColor(status?: RiskEvent_Status) {
  return (
    RISK_EVENT_STATUS_COLOR_MAP[
      status as keyof typeof RISK_EVENT_STATUS_COLOR_MAP
    ] || RISK_EVENT_STATUS_COLOR_MAP.DEFAULT
  );
}

export function riskEventStatusToName(status?: RiskEvent_Status) {
  const values = riskEventStatusList.value;
  const matchedItem = values.find((item) => item.value === status);
  return matchedItem ? matchedItem.label : '';
}
