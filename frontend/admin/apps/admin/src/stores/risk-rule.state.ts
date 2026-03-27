import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createRiskRuleServiceClient,
  type ubaservicev1_RiskAction_ActionType as RiskAction_ActionType,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useRiskRuleListStore = defineStore('risk-rule-list', () => {
  const service = createRiskRuleServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询风险规则列表
   */
  async function listRiskRule(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await service.List({
      // @ts-ignore proto generated code is error.
      fieldMask,
      orderBy: makeOrderBy(orderBy),
      query: makeQueryString(formValues, userStore.isTenantUser()),
      page: paging?.page,
      pageSize: paging?.pageSize,
      noPaging,
    });
  }

  /**
   * 获取风险规则
   */
  async function getRiskRule(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建风险规则
   */
  async function createRiskRule(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新风险规则
   */
  async function updateRiskRule(id: number, values: Record<string, any> = {}) {
    if ('id' in values) delete values.id;

    return await service.Update({
      id,
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
      // @ts-ignore proto generated code is error.
      updateMask: makeUpdateMask(Object.keys(values ?? [])),
    });
  }

  /**
   * 删除风险规则
   */
  async function deleteRiskRule(id: number) {
    return await service.Delete({ id });
  }

  function $reset() {}

  return {
    $reset,
    listRiskRule,
    getRiskRule,
    createRiskRule,
    updateRiskRule,
    deleteRiskRule,
  };
});

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
