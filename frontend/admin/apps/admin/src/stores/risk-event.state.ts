import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createRiskEventServiceClient,
  type ubaservicev1_RiskEvent_Status as RiskEvent_Status,
} from '#/generated/api/admin/service/v1';
import { getDictEntryLabelByValue, useDictStore } from '#/stores/dict.state';
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

export function riskLevelDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('RISK_LEVEL');
}

export function riskLevelToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('RISK_LEVEL'),
  );
}

export function riskTypeDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('RISK_TYPE');
}

export function riskTypeToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('RISK_TYPE'),
  );
}

export function riskEventStatusDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('RISK_EVENT_STATUS');
}

export function riskEventStatusToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('RISK_EVENT_STATUS'),
  );
}

const RISK_TYPE_COLOR_MAP = {
  login_anomaly: '#F53F3F',
  brute_force: '#F77234',
  credential_stuffing: '#FF9A2E',
  frequent_operation: '#4096FF',
  abnormal_flow: '#722ED1',
  data_exfiltration: '#F53F3F',
  device_change: '#F77234',
  location_anomaly: '#FF9A2E',
  proxy_detected: '#4096FF',
  fraud_payment: '#F53F3F',
  abuse_promotion: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function riskEventTypeToColor(type?: any) {
  return (
    RISK_TYPE_COLOR_MAP[type as keyof typeof RISK_TYPE_COLOR_MAP] ||
    RISK_TYPE_COLOR_MAP.DEFAULT
  );
}

const RISK_LEVEL_COLOR_MAP = {
  low: '#00B42A',
  medium: '#FF9A2E',
  high: '#F77234',
  critical: '#F53F3F',
  DEFAULT: '#86909C',
} as const;

export function riskLevelToColor(level?: any) {
  return (
    RISK_LEVEL_COLOR_MAP[level as keyof typeof RISK_LEVEL_COLOR_MAP] ||
    RISK_LEVEL_COLOR_MAP.DEFAULT
  );
}

const RISK_EVENT_STATUS_COLOR_MAP = {
  pending: '#FF9A2E',
  investigating: '#4096FF',
  confirmed: '#F53F3F',
  false_positive: '#00B42A',
  ignored: '#C9CDD4',
  auto_blocked: '#F77234',

  DEFAULT: '#86909C',
} as const;

export function riskEventStatusToColor(status?: RiskEvent_Status) {
  return (
    RISK_EVENT_STATUS_COLOR_MAP[
      status as keyof typeof RISK_EVENT_STATUS_COLOR_MAP
    ] || RISK_EVENT_STATUS_COLOR_MAP.DEFAULT
  );
}
