import type {
  ubaservicev1_GetUserBehaviorProfileRequest,
  ubaservicev1_ListUserBehaviorProfileResponse,
  ubaservicev1_UserBehaviorProfile,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 用户画像
// ==============================

export function useListUserBehaviorProfiles(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListUserBehaviorProfileResponse, Error>,
) {
  return useQuery({
    queryKey: ['listUserBehaviorProfiles', query],
    queryFn: () =>
      apiClient.userBehaviorProfileService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListUserBehaviorProfiles(
  params: PaginationQuery,
) {
  return queryClient.fetchQuery({
    queryKey: ['listUserBehaviorProfiles', params],
    queryFn: () =>
      apiClient.userBehaviorProfileService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetUserBehaviorProfile(
  req: ubaservicev1_GetUserBehaviorProfileRequest,
  options?: UseQueryOptions<ubaservicev1_UserBehaviorProfile, Error>,
) {
  return useQuery({
    queryKey: ['getUserBehaviorProfile', req],
    queryFn: () => apiClient.userBehaviorProfileService.Get(req),
    ...options,
  });
}
