import type {
  ubaservicev1_CountUserTagResponse,
  ubaservicev1_GetUserTagRequest,
  ubaservicev1_ListUserTagResponse,
  ubaservicev1_UserTag,
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
// 用户标签
// ==============================

export function useListUserTags(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListUserTagResponse, Error>,
) {
  return useQuery({
    queryKey: ['listUserTags', query],
    queryFn: () => apiClient.userTagService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListUserTags(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listUserTags', params],
    queryFn: () => apiClient.userTagService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCountUserTags(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountUserTagResponse, Error>,
) {
  return useQuery({
    queryKey: ['countUserTags', query],
    queryFn: () => apiClient.userTagService.Count(query.toRawParams()),
    ...options,
  });
}

export function useGetUserTag(
  req: ubaservicev1_GetUserTagRequest,
  options?: UseQueryOptions<ubaservicev1_UserTag, Error>,
) {
  return useQuery({
    queryKey: ['getUserTag', req],
    queryFn: () => apiClient.userTagService.Get(req),
    ...options,
  });
}

export function useCreateUserTag(
  options?: UseMutationOptions<
    ubaservicev1_UserTag,
    Error,
    Record<string, any>
  >,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.userTagService.Create({ data: { ...values } as any }),
    ...options,
  });
}

export function useUpdateUserTag(
  options?: UseMutationOptions<
    ubaservicev1_UserTag,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.userTagService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteUserTag(
  options?: UseMutationOptions<object, Error, number>,
) {
  return useMutation({
    mutationFn: (id) => apiClient.userTagService.Delete({ id }),
    ...options,
  });
}

// ==============================
// 用户标签枚举与工具函数
// ==============================

const TAG_SOURCE_COLOR_MAP = {
  manual: '#4096FF',
  rule: '#00B42A',
  model: '#F77234',
  import: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function userTagSourceToColor(source?: string) {
  return (
    TAG_SOURCE_COLOR_MAP[source as keyof typeof TAG_SOURCE_COLOR_MAP] ||
    TAG_SOURCE_COLOR_MAP.DEFAULT
  );
}

// 标签来源枚举值 → 显示名（前端硬编码兜底，字典表无数据时使用）
const TAG_SOURCE_NAME_MAP: Record<string, string> = {
  TAG_SOURCE_UNSPECIFIED: '未指定',
  TAG_SOURCE_MANUAL: '人工打标',
  TAG_SOURCE_RULE: '规则引擎',
  TAG_SOURCE_MODEL: '算法模型',
  TAG_SOURCE_IMPORT: '批量导入',
};

export function userTagSourceDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('TAG_SOURCE');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(TAG_SOURCE_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function userTagSourceToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('TAG_SOURCE');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (TAG_SOURCE_NAME_MAP[source] ?? source);
}
