import type {
  ubaservicev1_ActiveUsersRequest,
  ubaservicev1_ActiveUsersResponse,
  ubaservicev1_AttributionRequest,
  ubaservicev1_AttributionResponse,
  ubaservicev1_BehaviorSequenceRequest,
  ubaservicev1_BehaviorSequenceResponse,
  ubaservicev1_ClickRequest,
  ubaservicev1_ClickResponse,
  ubaservicev1_ChurnRequest,
  ubaservicev1_ChurnResponse,
  ubaservicev1_IntervalRequest,
  ubaservicev1_IntervalResponse,
  ubaservicev1_LifecycleRequest,
  ubaservicev1_LifecycleResponse,
  ubaservicev1_MatrixRequest,
  ubaservicev1_MatrixResponse,
  ubaservicev1_AnomalyRequest,
  ubaservicev1_AnomalyResponse,
  ubaservicev1_NewVsOldRequest,
  ubaservicev1_NewVsOldResponse,
  ubaservicev1_PathSankeyRequest,
  ubaservicev1_PathSankeyResponse,
  ubaservicev1_LevelAnalysisRequest,
  ubaservicev1_LevelAnalysisResponse,
  ubaservicev1_LTVRequest,
  ubaservicev1_LTVResponse,
  ubaservicev1_WhaleTierRequest,
  ubaservicev1_WhaleTierResponse,
  ubaservicev1_RevenueRequest,
  ubaservicev1_RevenueResponse,
  ubaservicev1_SessionAnalysisRequest,
  ubaservicev1_SessionAnalysisResponse,
  ubaservicev1_DistributionRequest,
  ubaservicev1_DistributionResponse,
  ubaservicev1_EventTrendRequest,
  ubaservicev1_EventTrendResponse,
  ubaservicev1_FunnelRequest,
  ubaservicev1_FunnelResponse,
  ubaservicev1_GroupByRequest,
  ubaservicev1_GroupByResponse,
  ubaservicev1_RetentionRequest,
  ubaservicev1_RetentionResponse,
  ubaservicev1_SegmentationRequest,
  ubaservicev1_SegmentationResponse,
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
// 归因分析（首触/末触渠道归因）
// ==============================
export function useAttribution(
  req: ubaservicev1_AttributionRequest,
  options?: UseQueryOptions<ubaservicev1_AttributionResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'attribution', req],
    queryFn: () => apiClient.analyticsService.Attribution(req),
    ...options,
  });
}

export async function fetchAttribution(req: ubaservicev1_AttributionRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'attribution', req],
    queryFn: () => apiClient.analyticsService.Attribution(req),
    staleTime: 60_000,
  });
}

// ==============================
// 分布分析（事件时长分桶 + 分位数）
// ==============================
export function useDistribution(
  req: ubaservicev1_DistributionRequest,
  options?: UseQueryOptions<ubaservicev1_DistributionResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'distribution', req],
    queryFn: () => apiClient.analyticsService.Distribution(req),
    ...options,
  });
}

export async function fetchDistribution(req: ubaservicev1_DistributionRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'distribution', req],
    queryFn: () => apiClient.analyticsService.Distribution(req),
    staleTime: 60_000,
  });
}

// ==============================
// 行为序列（指定用户的行为时间线）
// ==============================
export function useBehaviorSequence(
  req: ubaservicev1_BehaviorSequenceRequest,
  options?: UseQueryOptions<ubaservicev1_BehaviorSequenceResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'behaviorSequence', req],
    queryFn: () => apiClient.analyticsService.BehaviorSequence(req),
    ...options,
  });
}

export async function fetchBehaviorSequence(
  req: ubaservicev1_BehaviorSequenceRequest,
) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'behaviorSequence', req],
    queryFn: () => apiClient.analyticsService.BehaviorSequence(req),
    staleTime: 60_000,
  });
}

// ==============================
// 用户分群/圈选
// ==============================
export function useSegmentation(
  req: ubaservicev1_SegmentationRequest,
  options?: UseQueryOptions<ubaservicev1_SegmentationResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'segmentation', req],
    queryFn: () => apiClient.analyticsService.Segmentation(req),
    ...options,
  });
}

export async function fetchSegmentation(req: ubaservicev1_SegmentationRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'segmentation', req],
    queryFn: () => apiClient.analyticsService.Segmentation(req),
    staleTime: 60_000,
  });
}

// ==============================
// 点击热力图（按页面网格分桶聚合）
// ==============================
export function useClick(
  req: ubaservicev1_ClickRequest,
  options?: UseQueryOptions<ubaservicev1_ClickResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'click', req],
    queryFn: () => apiClient.analyticsService.Click(req),
    ...options,
  });
}

export async function fetchClick(req: ubaservicev1_ClickRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'click', req],
    queryFn: () => apiClient.analyticsService.Click(req),
    staleTime: 60_000,
  });
}

// ==============================
// 用户生命周期
// ==============================
export function useLifecycle(
  req: ubaservicev1_LifecycleRequest,
  options?: UseQueryOptions<ubaservicev1_LifecycleResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'lifecycle', req],
    queryFn: () => apiClient.analyticsService.Lifecycle(req),
    ...options,
  });
}

export async function fetchLifecycle(req: ubaservicev1_LifecycleRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'lifecycle', req],
    queryFn: () => apiClient.analyticsService.Lifecycle(req),
    staleTime: 60_000,
  });
}

// ==============================
// 流失与回流
// ==============================
export function useChurn(
  req: ubaservicev1_ChurnRequest,
  options?: UseQueryOptions<ubaservicev1_ChurnResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'churn', req],
    queryFn: () => apiClient.analyticsService.Churn(req),
    ...options,
  });
}

export async function fetchChurn(req: ubaservicev1_ChurnRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'churn', req],
    queryFn: () => apiClient.analyticsService.Churn(req),
    staleTime: 60_000,
  });
}

// ==============================
// 间隔时间分析
// ==============================
export function useInterval(
  req: ubaservicev1_IntervalRequest,
  options?: UseQueryOptions<ubaservicev1_IntervalResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'interval', req],
    queryFn: () => apiClient.analyticsService.Interval(req),
    ...options,
  });
}

export async function fetchInterval(req: ubaservicev1_IntervalRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'interval', req],
    queryFn: () => apiClient.analyticsService.Interval(req),
    staleTime: 60_000,
  });
}

// ==============================
// 矩阵/象限分析
// ==============================
export function useMatrix(
  req: ubaservicev1_MatrixRequest,
  options?: UseQueryOptions<ubaservicev1_MatrixResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'matrix', req],
    queryFn: () => apiClient.analyticsService.Matrix(req),
    ...options,
  });
}

export async function fetchMatrix(req: ubaservicev1_MatrixRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'matrix', req],
    queryFn: () => apiClient.analyticsService.Matrix(req),
    staleTime: 60_000,
  });
}

// ==============================
// 付费/营收分析
// ==============================
export function useRevenue(
  req: ubaservicev1_RevenueRequest,
  options?: UseQueryOptions<ubaservicev1_RevenueResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'revenue', req],
    queryFn: () => apiClient.analyticsService.Revenue(req),
    ...options,
  });
}

export async function fetchRevenue(req: ubaservicev1_RevenueRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'revenue', req],
    queryFn: () => apiClient.analyticsService.Revenue(req),
    staleTime: 60_000,
  });
}

// ==============================
// 会话分析
// ==============================
export function useSessionAnalysis(
  req: ubaservicev1_SessionAnalysisRequest,
  options?: UseQueryOptions<ubaservicev1_SessionAnalysisResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'sessionAnalysis', req],
    queryFn: () => apiClient.analyticsService.SessionAnalysis(req),
    ...options,
  });
}

export async function fetchSessionAnalysis(
  req: ubaservicev1_SessionAnalysisRequest,
) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'sessionAnalysis', req],
    queryFn: () => apiClient.analyticsService.SessionAnalysis(req),
    staleTime: 60_000,
  });
}

// ==============================
// 同比环比/异常检测
// ==============================
export function useAnomaly(
  req: ubaservicev1_AnomalyRequest,
  options?: UseQueryOptions<ubaservicev1_AnomalyResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'anomaly', req],
    queryFn: () => apiClient.analyticsService.Anomaly(req),
    ...options,
  });
}

export async function fetchAnomaly(req: ubaservicev1_AnomalyRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'anomaly', req],
    queryFn: () => apiClient.analyticsService.Anomaly(req),
    staleTime: 60_000,
  });
}

// ==============================
// 新老用户对比
// ==============================
export function useNewVsOld(
  req: ubaservicev1_NewVsOldRequest,
  options?: UseQueryOptions<ubaservicev1_NewVsOldResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'newVsOld', req],
    queryFn: () => apiClient.analyticsService.NewVsOld(req),
    ...options,
  });
}

export async function fetchNewVsOld(req: ubaservicev1_NewVsOldRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'newVsOld', req],
    queryFn: () => apiClient.analyticsService.NewVsOld(req),
    staleTime: 60_000,
  });
}

// ==============================
// 热门转化路径
// ==============================
export function usePathSankey(
  req: ubaservicev1_PathSankeyRequest,
  options?: UseQueryOptions<ubaservicev1_PathSankeyResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'pathSankey', req],
    queryFn: () => apiClient.analyticsService.PathSankey(req),
    ...options,
  });
}

export async function fetchPathSankey(req: ubaservicev1_PathSankeyRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'pathSankey', req],
    queryFn: () => apiClient.analyticsService.PathSankey(req),
    staleTime: 60_000,
  });
}

// ==============================
// 关卡/数值平衡分析
// ==============================
export function useLevelAnalysis(
  req: ubaservicev1_LevelAnalysisRequest,
  options?: UseQueryOptions<ubaservicev1_LevelAnalysisResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'levelAnalysis', req],
    queryFn: () => apiClient.analyticsService.LevelAnalysis(req),
    ...options,
  });
}

export async function fetchLevelAnalysis(
  req: ubaservicev1_LevelAnalysisRequest,
) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'levelAnalysis', req],
    queryFn: () => apiClient.analyticsService.LevelAnalysis(req),
    staleTime: 60_000,
  });
}

// ==============================
// 鲸鱼用户/付费分层
// ==============================
export function useWhaleTier(
  req: ubaservicev1_WhaleTierRequest,
  options?: UseQueryOptions<ubaservicev1_WhaleTierResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'whaleTier', req],
    queryFn: () => apiClient.analyticsService.WhaleTier(req),
    ...options,
  });
}

export async function fetchWhaleTier(req: ubaservicev1_WhaleTierRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'whaleTier', req],
    queryFn: () => apiClient.analyticsService.WhaleTier(req),
    staleTime: 60_000,
  });
}

// ==============================
// 历史生命周期价值 LTV
// ==============================
export function useLTV(
  req: ubaservicev1_LTVRequest,
  options?: UseQueryOptions<ubaservicev1_LTVResponse, Error>,
) {
  return useQuery({
    queryKey: ['analytics', 'ltv', req],
    queryFn: () => apiClient.analyticsService.LTV(req),
    ...options,
  });
}

export async function fetchLTV(req: ubaservicev1_LTVRequest) {
  return queryClient.fetchQuery({
    queryKey: ['analytics', 'ltv', req],
    queryFn: () => apiClient.analyticsService.LTV(req),
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
