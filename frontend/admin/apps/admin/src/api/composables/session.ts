import type {
  ubaservicev1_GetSessionRequest,
  ubaservicev1_ListSessionResponse,
  ubaservicev1_Session,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 会话
// ==============================

export function useListSessions(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListSessionResponse, Error>,
) {
  return useQuery({
    queryKey: ['listSessions', query],
    queryFn: () => apiClient.sessionService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListSessions(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listSessions', params],
    queryFn: () => apiClient.sessionService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetSession(
  req: ubaservicev1_GetSessionRequest,
  options?: UseQueryOptions<ubaservicev1_Session, Error>,
) {
  return useQuery({
    queryKey: ['getSession', req],
    queryFn: () => apiClient.sessionService.Get(req),
    ...options,
  });
}
