import type {
  internal_messageservicev1_InternalMessageCategory,
  internal_messageservicev1_ListInternalMessageCategoryResponse,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { makeUpdateMask, type PaginationQuery } from '#/transport/rest';

// ==============================
// 站内信分类
// ==============================

export function useListInternalMessageCategories(
  query: PaginationQuery,
  options?: UseQueryOptions<
    internal_messageservicev1_ListInternalMessageCategoryResponse,
    Error
  >,
) {
  return useQuery({
    queryKey: ['listInternalMessageCategories', query],
    queryFn: () =>
      apiClient.internalMessageCategoryService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListInternalMessageCategories(
  params: PaginationQuery,
) {
  return queryClient.fetchQuery({
    queryKey: ['listInternalMessageCategories', params],
    queryFn: () =>
      apiClient.internalMessageCategoryService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCreateInternalMessageCategory(
  options?: UseMutationOptions<object, Error, Record<string, any>>,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.internalMessageCategoryService.Create({
        data: { ...values } as any,
      }),
    ...options,
  });
}

export function useUpdateInternalMessageCategory(
  options?: UseMutationOptions<
    object,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.internalMessageCategoryService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteInternalMessageCategory(
  options?: UseMutationOptions<
    object,
    Error,
    internal_messageservicev1_InternalMessageCategory
  >,
) {
  return useMutation({
    mutationFn: (data) =>
      apiClient.internalMessageCategoryService.Delete(data as any),
    ...options,
  });
}
