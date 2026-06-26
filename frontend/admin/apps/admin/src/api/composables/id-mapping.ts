import type {
  ubaservicev1_GetIDMappingRequest,
  ubaservicev1_IDMapping,
  ubaservicev1_ListIDMappingResponse,
} from '#/generated/api/admin/service/v1';

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import {
  getDictEntriesByTypeCode,
  getDictEntriesOptionsByTypeCode,
  getDictEntryLabelByValue,
} from '#/api/composables/dict';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// ID 映射
// ==============================

export function useListIDMappings(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListIDMappingResponse, Error>,
) {
  return useQuery({
    queryKey: ['listIDMappings', query],
    queryFn: () => apiClient.iDMappingService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListIDMappings(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listIDMappings', params],
    queryFn: () => apiClient.iDMappingService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetIDMapping(
  req: ubaservicev1_GetIDMappingRequest,
  options?: UseQueryOptions<ubaservicev1_IDMapping, Error>,
) {
  return useQuery({
    queryKey: ['getIDMapping', req],
    queryFn: () => apiClient.iDMappingService.Get(req),
    ...options,
  });
}

// ==============================
// ID映射枚举与工具函数
// ==============================

export function idTypeDict() {
  return getDictEntriesOptionsByTypeCode('ID_TYPE');
}

export function idTypeToName(source?: string) {
  return getDictEntryLabelByValue(source, getDictEntriesByTypeCode('ID_TYPE'));
}

const ID_TYPE_COLOR_MAP = {
  user_id: '#4096FF',
  device_id: '#00B42A',
  cookie: '#F77234',
  email: '#722ED1',
  phone: '#FF9A2E',
  openid: '#1FB5AD',
  unionid: '#1FB5AD',
  global_user_id: '#1FB5AD',
  DEFAULT: '#86909C',
} as const;

export function idMappingIdTypeToColor(type?: string) {
  return (
    ID_TYPE_COLOR_MAP[type as keyof typeof ID_TYPE_COLOR_MAP] ||
    ID_TYPE_COLOR_MAP.DEFAULT
  );
}
