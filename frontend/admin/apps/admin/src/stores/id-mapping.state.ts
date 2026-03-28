import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createIDMappingServiceClient } from '#/generated/api/admin/service/v1';
import { getDictEntryLabelByValue, useDictStore } from '#/stores/dict.state';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useIdMappingListStore = defineStore('id-mapping-list', () => {
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

export function idTypeDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('ID_TYPE');
}

export function idTypeToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('ID_TYPE'),
  );
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
  return (
    ID_TYPE_COLOR_MAP[type as keyof typeof ID_TYPE_COLOR_MAP] ||
    ID_TYPE_COLOR_MAP.DEFAULT
  );
}
