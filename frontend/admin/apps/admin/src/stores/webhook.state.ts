import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createWebhookServiceClient } from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useWebhookListStore = defineStore('webhook-list', () => {
  const service = createWebhookServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询网络钩子列表
   */
  async function listWebhook(
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
   * 获取网络钩子
   */
  async function getWebhook(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建网络钩子
   */
  async function createWebhook(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新网络钩子
   */
  async function updateWebhook(id: number, values: Record<string, any> = {}) {
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
   * 删除网络钩子
   */
  async function deleteWebhook(id: number) {
    return await service.Delete({ id });
  }

  function $reset() {}

  return {
    $reset,
    listWebhook,
    getWebhook,
    createWebhook,
    updateWebhook,
    deleteWebhook,
  };
});
