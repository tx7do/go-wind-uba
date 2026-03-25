import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createIDMappingServiceClient,
  type ubaservicev1_IDType as IDType,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useIDMappingListStore = defineStore('id-mapping-list', () => {
  const service = createIDMappingServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询 ID 映射列表
   */
  async function listIDMapping(
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
   * 获取 ID 映射
   */
  async function getIDMapping(id: number) {
    return await service.Get({ id });
  }

  function $reset() {}

  return {
    $reset,
    listIDMapping,
    getIDMapping,
  };
});

export const idTypeList = computed(() => [
  { value: 'ID_TYPE_UNSPECIFIED', label: $t('enum.id.type.UNSPECIFIED') },
  { value: 'ID_TYPE_USER_ID', label: $t('enum.id.type.USER_ID') },
  { value: 'ID_TYPE_DEVICE_ID', label: $t('enum.id.type.DEVICE_ID') },
  { value: 'ID_TYPE_COOKIE', label: $t('enum.id.type.COOKIE') },
  { value: 'ID_TYPE_EMAIL', label: $t('enum.id.type.EMAIL') },
  { value: 'ID_TYPE_PHONE', label: $t('enum.id.type.PHONE') },
  { value: 'ID_TYPE_OPENID', label: $t('enum.id.type.OPENID') },
]);

const ID_TYPE_COLOR_MAP = {
  ID_TYPE_UNSPECIFIED: '#86909C',
  ID_TYPE_USER_ID: '#4096FF',
  ID_TYPE_DEVICE_ID: '#00B42A',
  ID_TYPE_COOKIE: '#F77234',
  ID_TYPE_EMAIL: '#722ED1',
  ID_TYPE_PHONE: '#FF9A2E',
  ID_TYPE_OPENID: '#1FB5AD',
  DEFAULT: '#86909C',
} as const;

export function idTypeToColor(type?: IDType) {
  return (
    ID_TYPE_COLOR_MAP[type as keyof typeof ID_TYPE_COLOR_MAP] ||
    ID_TYPE_COLOR_MAP.DEFAULT
  );
}

export function idTypeToName(type?: IDType) {
  const values = idTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : '';
}
