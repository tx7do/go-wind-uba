import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createObjectServiceClient } from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useObjectDimListStore = defineStore('object-dim-list', () => {
  const service = createObjectServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询对象维度列表
   */
  async function listObjectDim(
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
   * 获取对象维度
   */
  async function getObjectDim(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建对象维度
   */
  async function createObjectDim(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  function $reset() {}

  return {
    $reset,
    listObjectDim,
    getObjectDim,
    createObjectDim,
  };
});
