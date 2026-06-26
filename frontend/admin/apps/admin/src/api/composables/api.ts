import type {
  resourceservicev1_Api,
  resourceservicev1_DeleteApiRequest,
  resourceservicev1_GetApiRequest,
  resourceservicev1_ListApiResponse,
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
// API 管理
// ==============================

export function useListApis(
  query: PaginationQuery,
  options?: UseQueryOptions<resourceservicev1_ListApiResponse, Error>,
) {
  return useQuery({
    queryKey: ['listApis', query],
    queryFn: () => apiClient.apiService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListApis(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listApis', params],
    queryFn: () => apiClient.apiService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetApi(
  req: resourceservicev1_GetApiRequest,
  options?: UseQueryOptions<resourceservicev1_Api, Error>,
) {
  return useQuery({
    queryKey: ['getApi', req],
    queryFn: () => apiClient.apiService.Get(req),
    ...options,
  });
}

export function useCreateApi(
  options?: UseMutationOptions<object, Error, Record<string, any>>,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.apiService.Create({ data: { ...values } as resourceservicev1_Api }),
    ...options,
  });
}

export function useUpdateApi(
  options?: UseMutationOptions<
    object,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.apiService.Update({
        id,
        data: {
          ...values,
        },
        updateMask: makeUpdateMask(Object.keys(values ?? [])),
      }),
    ...options,
  });
}

export function useDeleteApi(
  options?: UseMutationOptions<
    object,
    Error,
    resourceservicev1_DeleteApiRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => apiClient.apiService.Delete(data),
    ...options,
  });
}

export function useSyncApisApi(options?: UseMutationOptions<object, Error>) {
  return useMutation({
    mutationFn: () => apiClient.apiService.SyncApis({}),
    ...options,
  });
}

// ==============================
// API 枚举与工具函数
// ==============================

export function convertApiToTree(apis: any[]): any[] {
  const tree: any[] = [];
  for (const api of apis) {
    if (!api) continue;
    if (api.parentId !== 0 && api.parentId !== undefined) continue;
    tree.push(api);
  }
  for (const api of apis) {
    if (!api) continue;
    if (api.parentId === 0 || api.parentId === undefined) continue;
    function findParent(nodes: any[]): boolean {
      for (const node of nodes) {
        if (node.id === api.parentId) {
          if (node.children !== undefined) node.children.push(api);
          return true;
        }
        if (node.children && findParent(node.children)) return true;
      }
      return false;
    }
    if (findParent(tree)) continue;
    tree.push(api);
  }
  return tree;
}
