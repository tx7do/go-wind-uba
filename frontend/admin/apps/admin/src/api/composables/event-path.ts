import type {
  ubaservicev1_EventPath,
  ubaservicev1_GetEventPathRequest,
  ubaservicev1_ListEventPathResponse,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 事件路径
// ==============================

export function useListEventPaths(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListEventPathResponse, Error>,
) {
  return useQuery({
    queryKey: ['listEventPaths', query],
    queryFn: () => apiClient.eventPathService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListEventPaths(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listEventPaths', params],
    queryFn: () => apiClient.eventPathService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetEventPath(
  req: ubaservicev1_GetEventPathRequest,
  options?: UseQueryOptions<ubaservicev1_EventPath, Error>,
) {
  return useQuery({
    queryKey: ['getEventPath', req],
    queryFn: () => apiClient.eventPathService.Get(req),
    ...options,
  });
}
