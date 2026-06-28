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

// ID类型枚举值 → 显示名（前端硬编码兜底，字典表无数据时使用）
// key 统一用小写蛇形；匹配时对 source 做归一化，兼容 user_id / USER_ID / userId / ID_TYPE_USER 等变体
const ID_TYPE_NAME_MAP: Record<string, string> = {
  user_id: '用户ID',
  device_id: '设备ID',
  cookie: 'Cookie',
  email: '邮箱',
  phone: '手机号',
  openid: 'OpenID',
  unionid: 'UnionID',
  global_user_id: '全局用户ID',
};

// 把任意命名形式归一化为小写蛇形 key（user_id / USER_ID / userId / ID_TYPE_USER → user_id）
function normalizeIdType(source: string): string {
  const lower = source.toLowerCase();
  // 去掉常见前缀（ID_TYPE_ 前缀的枚举形式）
  const stripped = lower.replace(/^id_type_/, '');
  // 驼峰转蛇形：userId → user_id
  const snake = stripped.replaceAll(/([a-z])([A-Z])/g, '$1_$2').toLowerCase();
  return snake;
}

export function idTypeDict() {
  const fromDict = getDictEntriesOptionsByTypeCode('ID_TYPE');
  // 字典表无数据时回退到前端硬编码选项
  return fromDict.length > 0
    ? fromDict
    : Object.entries(ID_TYPE_NAME_MAP).map(([value, label]) => ({
        label,
        value,
      }));
}

export function idTypeToName(source?: string) {
  if (!source) return '';
  // 优先用字典翻译，字典缺失时回退到前端硬编码映射
  const dictEntries = getDictEntriesByTypeCode('ID_TYPE');
  const fromDict = getDictEntryLabelByValue(source, dictEntries);
  if (fromDict && fromDict !== source) return fromDict;
  // 硬编码兜底：归一化后匹配，兼容多种命名变体
  const key = normalizeIdType(source);
  return ID_TYPE_NAME_MAP[key] ?? source;
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
  if (!type) return ID_TYPE_COLOR_MAP.DEFAULT;
  // 归一化后匹配，兼容 user_id / USER_ID / userId / ID_TYPE_USER 等变体
  const key = normalizeIdType(type);
  return (
    ID_TYPE_COLOR_MAP[key as keyof typeof ID_TYPE_COLOR_MAP] ||
    ID_TYPE_COLOR_MAP.DEFAULT
  );
}
