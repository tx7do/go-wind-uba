import type {
  ubaservicev1_Application,
  ubaservicev1_GetApplicationRequest,
  ubaservicev1_ListApplicationResponse,
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
// 应用管理
// ==============================

export function useListApplications(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListApplicationResponse, Error>,
) {
  return useQuery({
    queryKey: ['listApplications', query],
    queryFn: () => apiClient.applicationService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListApplications(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listApplications', params],
    queryFn: () => apiClient.applicationService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetApplication(
  req: ubaservicev1_GetApplicationRequest,
  options?: UseQueryOptions<ubaservicev1_Application, Error>,
) {
  return useQuery({
    queryKey: ['getApplication', req],
    queryFn: () => apiClient.applicationService.Get(req),
    ...options,
  });
}

export function useCreateApplication(
  options?: UseMutationOptions<
    ubaservicev1_Application,
    Error,
    Record<string, any>
  >,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.applicationService.Create({ data: { ...values } as any }),
    ...options,
  });
}

export function useUpdateApplication(
  options?: UseMutationOptions<
    ubaservicev1_Application,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.applicationService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteApplication(
  options?: UseMutationOptions<object, Error, number>,
) {
  return useMutation({
    mutationFn: (id) => apiClient.applicationService.Delete({ id }),
    ...options,
  });
}

// ==============================
// 应用枚举与工具函数
// ==============================

// 应用平台枚举值 → 显示名（前端硬编码兜底，字典表无数据时使用）
const APP_PLATFORM_NAME_MAP: Record<string, string> = {
  web: 'Web',
  ios: 'iOS',
  android: 'Android',
  windows: 'Windows',
  macos: 'macOS',
  linux: 'Linux',
  mini_program: '小程序',
  h5: 'H5',
};

// 应用类型枚举值 → 显示名（前端硬编码兜底）
const APP_TYPE_NAME_MAP: Record<string, string> = {
  game: '游戏',
  ecommerce: '电商',
  content: '内容',
  tool: '工具',
  finance: '金融',
  social: '社交',
  education: '教育',
  other: '其他',
};

export function appPlatformDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('APP_PLATFORM');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(APP_PLATFORM_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function appPlatformToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('APP_PLATFORM');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (APP_PLATFORM_NAME_MAP[source] ?? source);
}

export function appTypeDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('APP_TYPE');
  return fromDict.length > 0
    ? fromDict
    : Object.entries(APP_TYPE_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function appTypeToName(source?: string) {
  if (!source) return '';
  const dictEntries = getDictEntriesByTypeCode('APP_TYPE');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  return fromDict && fromDict !== source
    ? fromDict
    : (APP_TYPE_NAME_MAP[source] ?? source);
}

const PLATFORM_COLOR_MAP = {
  web: '#4096FF',
  ios: '#1890FF',
  android: '#34C759',
  windows: '#0078D4',
  macos: '#A8B1C1',
  linux: '#E95420',
  mini_program: '#07C160',
  h5: '#52C41A',
  DEFAULT: '#86909C',
} as const;

export function platformToColor(platform?: string) {
  return (
    PLATFORM_COLOR_MAP[platform as keyof typeof PLATFORM_COLOR_MAP] ||
    PLATFORM_COLOR_MAP.DEFAULT
  );
}

const APPLICATION_TYPE_COLOR_MAP = {
  game: '#4E6CFE',
  ecommerce: '#FF4D4F',
  content: '#20C997',
  tool: '#4096FF',
  finance: '#00B42A',
  social: '#FF7D00',
  education: '#165DFF',
  other: '#86909C',
  DEFAULT: '#A8B1C1',
} as const;

export function applicationTypeToColor(type?: string) {
  return (
    APPLICATION_TYPE_COLOR_MAP[
      type as keyof typeof APPLICATION_TYPE_COLOR_MAP
    ] || APPLICATION_TYPE_COLOR_MAP.DEFAULT
  );
}
