import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createEventPathServiceClient } from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useEventPathListStore = defineStore('event-path-list', () => {
  const service = createEventPathServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询事件路径列表
   */
  async function listEventPath(
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
   * 获取事件路径
   */
  async function getEventPath(id: string) {
    return await service.Get({ id });
  }

  function $reset() {}

  return {
    $reset,
    listEventPath,
    getEventPath,
  };
});
