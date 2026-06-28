import type {
  ubaservicev1_BehaviorEvent,
  ubaservicev1_GetBehaviorEventRequest,
  ubaservicev1_ListBehaviorEventResponse,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 行为事件明细（用户行为时间轴用）
// ==============================
export function useListBehaviorEvents(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListBehaviorEventResponse, Error>,
) {
  return useQuery({
    queryKey: ['listBehaviorEvents', query],
    queryFn: () => apiClient.behaviorEventService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListBehaviorEvents(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listBehaviorEvents', params],
    queryFn: () => apiClient.behaviorEventService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetBehaviorEvent(
  req: ubaservicev1_GetBehaviorEventRequest,
  options?: UseQueryOptions<ubaservicev1_BehaviorEvent, Error>,
) {
  return useQuery({
    queryKey: ['getBehaviorEvent', req],
    queryFn: () => apiClient.behaviorEventService.Get(req),
    ...options,
  });
}
