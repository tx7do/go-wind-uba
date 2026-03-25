import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createUserTagServiceClient,
  type ubaservicev1_TagSource as TagSource,
} from '#/generated/api/admin/service/v1';
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

export const tagSourceList = computed(() => [
  { value: 'TAG_SOURCE_UNSPECIFIED', label: $t('enum.tag.source.UNSPECIFIED') },
  { value: 'TAG_SOURCE_MANUAL', label: $t('enum.tag.source.MANUAL') },
  { value: 'TAG_SOURCE_RULE', label: $t('enum.tag.source.RULE') },
  { value: 'TAG_SOURCE_MODEL', label: $t('enum.tag.source.MODEL') },
  { value: 'TAG_SOURCE_IMPORT', label: $t('enum.tag.source.IMPORT') },
]);

const TAG_SOURCE_COLOR_MAP = {
  TAG_SOURCE_UNSPECIFIED: '#86909C',
  TAG_SOURCE_MANUAL: '#4096FF',
  TAG_SOURCE_RULE: '#00B42A',
  TAG_SOURCE_MODEL: '#F77234',
  TAG_SOURCE_IMPORT: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function tagSourceToColor(source?: TagSource) {
  return (
    TAG_SOURCE_COLOR_MAP[source as keyof typeof TAG_SOURCE_COLOR_MAP] ||
    TAG_SOURCE_COLOR_MAP.DEFAULT
  );
}

export function tagSourceToName(source?: TagSource) {
  const values = tagSourceList.value;
  const matchedItem = values.find((item) => item.value === source);
  return matchedItem ? matchedItem.label : '';
}
