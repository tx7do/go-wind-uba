import type {
  ubaservicev1_RiskEvent_Status as RiskEvent_Status,
  ubaservicev1_CountRiskEventResponse,
  ubaservicev1_GetRiskEventRequest,
  ubaservicev1_ListRiskEventResponse,
  ubaservicev1_RiskEvent,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import {
  getDictEntriesByTypeCode,
  getDictEntriesOptionsByTypeCode,
  getDictEntryLabelByValue,
} from '#/api/composables/dict';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 风险事件
// ==============================

export function useListRiskEvents(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListRiskEventResponse, Error>,
) {
  return useQuery({
    queryKey: ['listRiskEvents', query],
    queryFn: () => apiClient.riskEventService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListRiskEvents(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listRiskEvents', params],
    queryFn: () => apiClient.riskEventService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCountRiskEvents(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountRiskEventResponse, Error>,
) {
  return useQuery({
    queryKey: ['countRiskEvents', query],
    queryFn: () => apiClient.riskEventService.Count(query.toRawParams()),
    ...options,
  });
}

export function useGetRiskEvent(
  req: ubaservicev1_GetRiskEventRequest,
  options?: UseQueryOptions<ubaservicev1_RiskEvent, Error>,
) {
  return useQuery({
    queryKey: ['getRiskEvent', req],
    queryFn: () => apiClient.riskEventService.Get(req),
    ...options,
  });
}

export function useCreateRiskEvent(
  options?: UseMutationOptions<object, Error, Record<string, any>>,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.riskEventService.Create({ data: { ...values } as any }),
    ...options,
  });
}

// ==============================
// 风险事件枚举与工具函数
// ==============================

// 风险等级枚举值 → 显示名（前端硬编码兜底，字典表无数据时使用）
const RISK_LEVEL_NAME_MAP: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重',
};

// 风险类型枚举值 → 显示名（前端硬编码兜底）
const RISK_TYPE_NAME_MAP: Record<string, string> = {
  login_anomaly: '登录异常',
  brute_force: '暴力破解',
  credential_stuffing: '撞库攻击',
  frequent_operation: '频繁操作',
  abnormal_flow: '异常流量',
  data_exfiltration: '数据外泄',
  device_change: '设备变更',
  location_anomaly: '位置异常',
  proxy_detected: '代理检测',
  fraud_payment: '欺诈支付',
  abuse_promotion: '滥用促销',
};

// 风险事件处置状态枚举值 → 显示名（前端硬编码兜底）
const RISK_EVENT_STATUS_NAME_MAP: Record<string, string> = {
  pending: '待处理',
  investigating: '调查中',
  confirmed: '已确认',
  false_positive: '误报',
  ignored: '已忽略',
  auto_blocked: '自动拦截',
};

export function riskLevelDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('RISK_LEVEL');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(RISK_LEVEL_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function riskLevelToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('RISK_LEVEL');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (RISK_LEVEL_NAME_MAP[source] ?? source);
}

export function riskTypeDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('RISK_TYPE');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(RISK_TYPE_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function riskTypeToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('RISK_TYPE');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (RISK_TYPE_NAME_MAP[source] ?? source);
}

export function riskEventStatusDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('RISK_EVENT_STATUS');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(RISK_EVENT_STATUS_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function riskEventStatusToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('RISK_EVENT_STATUS');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (RISK_EVENT_STATUS_NAME_MAP[source] ?? source);
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
