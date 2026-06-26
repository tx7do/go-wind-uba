import type {
  ubaservicev1_CountTagDefinitionResponse,
  ubaservicev1_GetTagDefinitionRequest,
  ubaservicev1_ListTagDefinitionResponse,
  ubaservicev1_TagDefinition,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import {
  getDictEntriesByTypeCode,
  getDictEntriesOptionsByTypeCode,
  getDictEntryLabelByValue,
} from '#/api/composables/dict';
import { queryClient } from '#/plugins/vue-query';
import { makeUpdateMask, type PaginationQuery } from '#/transport/rest';

// ==============================
// 标签定义
// ==============================

export function useListTagDefinitions(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListTagDefinitionResponse, Error>,
) {
  return useQuery({
    queryKey: ['listTagDefinitions', query],
    queryFn: () => apiClient.tagDefinitionService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListTagDefinitions(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listTagDefinitions', params],
    queryFn: () => apiClient.tagDefinitionService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCountTagDefinitions(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountTagDefinitionResponse, Error>,
) {
  return useQuery({
    queryKey: ['countTagDefinitions', query],
    queryFn: () => apiClient.tagDefinitionService.Count(query.toRawParams()),
    ...options,
  });
}

export function useGetTagDefinition(
  req: ubaservicev1_GetTagDefinitionRequest,
  options?: UseQueryOptions<ubaservicev1_TagDefinition, Error>,
) {
  return useQuery({
    queryKey: ['getTagDefinition', req],
    queryFn: () => apiClient.tagDefinitionService.Get(req),
    ...options,
  });
}

export function useCreateTagDefinition(
  options?: UseMutationOptions<
    ubaservicev1_TagDefinition,
    Error,
    Record<string, any>
  >,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.tagDefinitionService.Create({ data: { ...values } as any }),
    ...options,
  });
}

export function useUpdateTagDefinition(
  options?: UseMutationOptions<
    ubaservicev1_TagDefinition,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.tagDefinitionService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteTagDefinition(
  options?: UseMutationOptions<object, Error, number>,
) {
  return useMutation({
    mutationFn: (id) => apiClient.tagDefinitionService.Delete({ id }),
    ...options,
  });
}

// ==============================
// 标签定义枚举与工具函数
// ==============================

export function tagCategoryDict() {
  return getDictEntriesOptionsByTypeCode('TAG_CATEGORY');
}

export function tagCategoryToName(source?: string) {
  return getDictEntryLabelByValue(source, getDictEntriesByTypeCode('TAG_CATEGORY'));
}

export function tagTypeDict() {
  return getDictEntriesOptionsByTypeCode('TAG_TYPE');
}

export function tagTypeToName(source?: string) {
  return getDictEntryLabelByValue(source, getDictEntriesByTypeCode('TAG_TYPE'));
}

const TAG_CATEGORY_COLOR_MAP = {
  TAG_CATEGORY_UNSPECIFIED: '#86909C',
  TAG_CATEGORY_USER: '#4096FF',
  TAG_CATEGORY_BEHAVIOR: '#00B42A',
  TAG_CATEGORY_RISK: '#F53F3F',
  TAG_CATEGORY_BUSINESS: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function tagCategoryToColor(category?: string) {
  return (
    TAG_CATEGORY_COLOR_MAP[category as keyof typeof TAG_CATEGORY_COLOR_MAP] ||
    TAG_CATEGORY_COLOR_MAP.DEFAULT
  );
}

const TAG_TYPE_COLOR_MAP = {
  TAG_TYPE_UNSPECIFIED: '#86909C',
  TAG_TYPE_BOOLEAN: '#4096FF',
  TAG_TYPE_ENUM: '#00B42A',
  TAG_TYPE_NUMERIC: '#F77234',
  TAG_TYPE_STRING: '#722ED1',
  TAG_TYPE_LIST: '#FF9A2E',
  DEFAULT: '#86909C',
} as const;

export function tagTypeToColor(type?: string) {
  return (
    TAG_TYPE_COLOR_MAP[type as keyof typeof TAG_TYPE_COLOR_MAP] ||
    TAG_TYPE_COLOR_MAP.DEFAULT
  );
}
