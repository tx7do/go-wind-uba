import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createApplicationServiceClient } from '#/generated/api/admin/service/v1';
import { getDictEntryLabelByValue, useDictStore } from '#/stores/dict.state';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useApplicationListStore = defineStore('application-list', () => {
  const service = createApplicationServiceClient(requestClientRequestHandler);
  const userStore = useUserStore();

  /**
   * 查询应用列表
   */
  async function listApplication(
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
   * 获取应用
   */
  async function getApplication(id: number) {
    return await service.Get({ id });
  }

  /**
   * 创建应用
   */
  async function createApplication(values: Record<string, any> = {}) {
    return await service.Create({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新应用
   */
  async function updateApplication(
    id: number,
    values: Record<string, any> = {},
  ) {
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
   * 删除应用
   */
  async function deleteApplication(id: number) {
    return await service.Delete({ id });
  }

  function $reset() {}

  return {
    $reset,
    listApplication,
    getApplication,
    createApplication,
    updateApplication,
    deleteApplication,
  };
});

export function appPlatformDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('APP_PLATFORM');
}

export function appPlatformToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('APP_PLATFORM'),
  );
}

export function appTypeDict() {
  const dictStore = useDictStore();
  return dictStore.getDictEntriesOptionsByTypeCode('APP_TYPE');
}

export function appTypeToName(source?: string) {
  const dictStore = useDictStore();
  return getDictEntryLabelByValue(
    source,
    dictStore.getDictEntriesByTypeCode('APP_TYPE'),
  );
}

const PLATFORM_COLOR_MAP = {
  web: '#4096FF', // Web：经典蓝（保留）
  ios: '#1890FF', // iOS：更清爽的苹果蓝
  android: '#34C759', // Android：官方绿（更柔和）
  windows: '#0078D4', // Windows：微软蓝（保留）
  macos: '#A8B1C1', // macOS：高级浅灰
  linux: '#E95420', // Linux：官方橙色（更标准）
  mini_program: '#07C160', // 小程序：微信绿（保留）
  h5: '#52C41A', // H5：官方绿（更标准）
  DEFAULT: '#86909C', // 默认中性灰
} as const;

export function platformToColor(platform?: string) {
  return (
    PLATFORM_COLOR_MAP[platform as keyof typeof PLATFORM_COLOR_MAP] ||
    PLATFORM_COLOR_MAP.DEFAULT
  );
}

const APPLICATION_TYPE_COLOR_MAP = {
  game: '#4E6CFE', // 游戏：更舒服的科技蓝
  ecommerce: '#FF4D4F', // 电商：活力红（行业标准）
  content: '#20C997', // 内容：清新绿
  tool: '#4096FF', // 工具：稳定蓝
  finance: '#00B42A', // 金融：安全绿
  social: '#FF7D00', // 社交：活力橙
  education: '#165DFF', // 教育：科技蓝
  other: '#86909C', // 其他：中性灰
  DEFAULT: '#A8B1C1', // 默认：浅灰（更柔和）
} as const;

export function applicationTypeToColor(type?: string) {
  return (
    APPLICATION_TYPE_COLOR_MAP[
      type as keyof typeof APPLICATION_TYPE_COLOR_MAP
    ] || APPLICATION_TYPE_COLOR_MAP.DEFAULT
  );
}
