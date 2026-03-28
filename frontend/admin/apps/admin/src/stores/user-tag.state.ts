import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createUserTagServiceClient } from '#/generated/api/admin/service/v1';
import {getDictEntryLabelByValue, useDictStore} from '#/stores/dict.state';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useUserTagListStore = defineStore('user-tag-list', () => {
  const service = createUserTagServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询用户标签列表
   */
  async function listUserTag(
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
   * 获取用户标签
   */
  async function getUserTag(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建用户标签
   */
  async function createUserTag(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新用户标签
   */
  async function updateUserTag(id: number, values: Record<string, any> = {}) {
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
   * 删除用户标签
   */
  async function deleteUserTag(id: number) {
    return await service.Delete({ id });
  }

  function $reset() {}

  return {
    $reset,
    listUserTag,
    getUserTag,
    createUserTag,
    updateUserTag,
    deleteUserTag,
  };
});

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

export function userTagSourceDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('TAG_SOURCE');
}

export function userTagSourceToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('TAG_SOURCE'),
  );
}
