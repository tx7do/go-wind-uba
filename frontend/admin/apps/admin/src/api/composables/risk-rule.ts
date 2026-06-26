import type {
  ubaservicev1_CountRiskRuleResponse,
  ubaservicev1_GetRiskRuleRequest,
  ubaservicev1_ListRiskRuleResponse,
  ubaservicev1_RiskAction_ActionType as RiskAction_ActionType,
  ubaservicev1_RiskRule,
} from '#/generated/api/admin/service/v1';

import { computed } from 'vue';

import { $t } from '@vben/locales';

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
// 风险规则
// ==============================

export function useListRiskRules(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListRiskRuleResponse, Error>,
) {
  return useQuery({
    queryKey: ['listRiskRules', query],
    queryFn: () => apiClient.riskRuleService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListRiskRules(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listRiskRules', params],
    queryFn: () => apiClient.riskRuleService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCountRiskRules(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountRiskRuleResponse, Error>,
) {
  return useQuery({
    queryKey: ['countRiskRules', query],
    queryFn: () => apiClient.riskRuleService.Count(query.toRawParams()),
    ...options,
  });
}

export function useGetRiskRule(
  req: ubaservicev1_GetRiskRuleRequest,
  options?: UseQueryOptions<ubaservicev1_RiskRule, Error>,
) {
  return useQuery({
    queryKey: ['getRiskRule', req],
    queryFn: () => apiClient.riskRuleService.Get(req),
    ...options,
  });
}

export function useCreateRiskRule(
  options?: UseMutationOptions<
    ubaservicev1_RiskRule,
    Error,
    Record<string, any>
  >,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.riskRuleService.Create({ data: { ...values } as any }),
    ...options,
  });
}

export function useUpdateRiskRule(
  options?: UseMutationOptions<
    ubaservicev1_RiskRule,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.riskRuleService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteRiskRule(
  options?: UseMutationOptions<object, Error, number>,
) {
  return useMutation({
    mutationFn: (id) => apiClient.riskRuleService.Delete({ id }),
    ...options,
  });
}

// ==============================
// 风险规则枚举与工具函数
// ==============================

export const riskActionTypeList = computed(() => [
  { value: 'ACTION_UNSPECIFIED', label: $t('enum.risk.action.UNSPECIFIED') },
  { value: 'BLOCK_USER', label: $t('enum.risk.action.BLOCK_USER') },
  { value: 'BLOCK_DEVICE', label: $t('enum.risk.action.BLOCK_DEVICE') },
  { value: 'REQUIRE_MFA', label: $t('enum.risk.action.REQUIRE_MFA') },
  { value: 'LIMIT_RATE', label: $t('enum.risk.action.LIMIT_RATE') },
  { value: 'NOTIFY_ADMIN', label: $t('enum.risk.action.NOTIFY_ADMIN') },
]);

const RISK_ACTION_TYPE_COLOR_MAP = {
  ACTION_UNSPECIFIED: '#86909C',
  BLOCK_USER: '#F53F3F',
  BLOCK_DEVICE: '#F77234',
  REQUIRE_MFA: '#FF9A2E',
  LIMIT_RATE: '#4096FF',
  NOTIFY_ADMIN: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function riskActionTypeToColor(type?: RiskAction_ActionType) {
  return (
    RISK_ACTION_TYPE_COLOR_MAP[
      type as keyof typeof RISK_ACTION_TYPE_COLOR_MAP
    ] || RISK_ACTION_TYPE_COLOR_MAP.DEFAULT
  );
}

export function riskActionTypeToName(type?: RiskAction_ActionType) {
  const values = riskActionTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : type;
}
