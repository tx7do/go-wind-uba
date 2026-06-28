import type {
  ubaservicev1_ActiveUsersRequest,
  ubaservicev1_ActiveUsersResponse,
  ubaservicev1_EventTrendRequest,
  ubaservicev1_EventTrendResponse,
  ubaservicev1_FunnelRequest,
  ubaservicev1_FunnelResponse,
  ubaservicev1_GroupByRequest,
  ubaservicev1_GroupByResponse,
  ubaservicev1_RetentionRequest,
  ubaservicev1_RetentionResponse,
  ubaservicev1_TimeRange,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';

// ==============================
// 时间范围工具（前端默认最近 7 天）
// ==============================

/** 构造最近 N 天的时间范围（毫秒） */
export function lastDaysRange(days: number): ubaservicev1_TimeRange {
  const end = Date.now();
  return {
    endMs: end,
    startMs: end - days * 24 * 60 * 60 * 1000,
  };
}

// ==============================
// 事件量趋势
// ==============================
export function useEventTrend(
  req: ubaservicev1_EventTrendRequest,
  options?: UseQueryOptions<ubaservicev1_EventTrendResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'eventTrend', req],
    queryFn: () => apiClient.analyticsService.EventTrend(req),
    ...options,
  });
}

export async function fetchEventTrend(req: ubaservicev1_EventTrendRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'eventTrend', req],
    queryFn: () => apiClient.analyticsService.EventTrend(req),
    staleTime: 60_000,
  });
}

// ==============================
// 漏斗分析
// ==============================
export function useFunnel(
  req: ubaservicev1_FunnelRequest,
  options?: UseQueryOptions<ubaservicev1_FunnelResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'funnel', req],
    queryFn: () => apiClient.analyticsService.Funnel(req),
    ...options,
  });
}

export async function fetchFunnel(req: ubaservicev1_FunnelRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'funnel', req],
    queryFn: () => apiClient.analyticsService.Funnel(req),
    staleTime: 60_000,
  });
}

// ==============================
// 留存分析
// ==============================
export function useRetention(
  req: ubaservicev1_RetentionRequest,
  options?: UseQueryOptions<ubaservicev1_RetentionResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'retention', req],
    queryFn: () => apiClient.analyticsService.Retention(req),
    ...options,
  });
}

export async function fetchRetention(req: ubaservicev1_RetentionRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'retention', req],
    queryFn: () => apiClient.analyticsService.Retention(req),
    staleTime: 60_000,
  });
}

// ==============================
// 维度分组聚合
// ==============================
export function useGroupBy(
  req: ubaservicev1_GroupByRequest,
  options?: UseQueryOptions<ubaservicev1_GroupByResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'groupBy', req],
    queryFn: () => apiClient.analyticsService.GroupBy(req),
    ...options,
  });
}

export async function fetchGroupBy(req: ubaservicev1_GroupByRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'groupBy', req],
    queryFn: () => apiClient.analyticsService.GroupBy(req),
    staleTime: 60_000,
  });
}

// ==============================
// 活跃用户（DAU/WAU/MAU）
// ==============================
export function useActiveUsers(
  req: ubaservicev1_ActiveUsersRequest,
  options?: UseQueryOptions<ubaservicev1_ActiveUsersResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'activeUsers', req],
    queryFn: () => apiClient.analyticsService.ActiveUsers(req),
    ...options,
  });
}

export async function fetchActiveUsers(req: ubaservicev1_ActiveUsersRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'activeUsers', req],
    queryFn: () => apiClient.analyticsService.ActiveUsers(req),
    staleTime: 60_000,
  });
}

// ==============================
// 维度枚举（与后端 allowedDimension 白名单对齐）
// ==============================
export type AnalyticsDimension =
  | 'app_version'
  | 'channel'
  | 'country'
  | 'event_category'
  | 'event_name'
  | 'network'
  | 'os'
  | 'platform';

export type AnalyticsMetric = 'COUNT' | 'SUM_AMOUNT' | 'UNIQUE_USER';

export type AnalyticsGranularity =
  | 'ANALYTICS_GRANULARITY_UNSPECIFIED'
  | 'DAY'
  | 'HOUR'
  | 'MONTH'
  | 'WEEK';
