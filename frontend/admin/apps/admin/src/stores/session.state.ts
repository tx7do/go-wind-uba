import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createSessionServiceClient } from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useSessionListStore = defineStore('session-list', () => {
  const service = createSessionServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询会话列表
   */
  async function listSession(
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
   * 获取会话
   */
  async function getSession(id: number) {
    return await service.Get({ id });
  }

  function $reset() {}

  return {
    $reset,
    listSession,
    getSession,
  };
});
