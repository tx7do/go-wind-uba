import type {
  ubaservicev1_GetObjectDimRequest,
  ubaservicev1_ListObjectDimResponse,
  ubaservicev1_ObjectDim,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { type PaginationQuery } from '#/transport/rest';

// ==============================
// 对象维度
// ==============================

export function useListObjectDims(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListObjectDimResponse, Error>,
) {
  return useQuery({
    queryKey: ['listObjectDims', query],
    queryFn: () => apiClient.objectService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListObjectDims(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listObjectDims', params],
    queryFn: () => apiClient.objectService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetObjectDim(
  req: ubaservicev1_GetObjectDimRequest,
  options?: UseQueryOptions<ubaservicev1_ObjectDim, Error>,
) {
  return useQuery({
    queryKey: ['getObjectDim', req],
    queryFn: () => apiClient.objectService.Get(req),
    ...options,
  });
}

export function useCreateObjectDim(
  options?: UseMutationOptions<object, Error, Record<string, any>>,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.objectService.Create({ data: { ...values } as any }),
    ...options,
  });
}
