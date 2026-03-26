import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  type ubaservicev1_Application_Type as Application_Type,
  createApplicationServiceClient,
  type ubaservicev1_Platform as Platform,
} from '#/generated/api/admin/service/v1';
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

export const platformList = computed(() => [
  { value: 'PLATFORM_WEB', label: $t('enum.platform.PLATFORM_WEB') },
  { value: 'PLATFORM_IOS', label: $t('enum.platform.PLATFORM_IOS') },
  { value: 'PLATFORM_ANDROID', label: $t('enum.platform.PLATFORM_ANDROID') },
  { value: 'PLATFORM_WINDOWS', label: $t('enum.platform.PLATFORM_WINDOWS') },
  { value: 'PLATFORM_MACOS', label: $t('enum.platform.PLATFORM_MACOS') },
  { value: 'PLATFORM_LINUX', label: $t('enum.platform.PLATFORM_LINUX') },
  {
    value: 'PLATFORM_MINI_PROGRAM',
    label: $t('enum.platform.PLATFORM_MINI_PROGRAM'),
  },
]);

const PLATFORM_COLOR_MAP = {
  PLATFORM_WEB: '#4096FF', // Web：经典蓝（保留）
  PLATFORM_IOS: '#1890FF', // iOS：更清爽的苹果蓝
  PLATFORM_ANDROID: '#34C759', // Android：官方绿（更柔和）
  PLATFORM_WINDOWS: '#0078D4', // Windows：微软蓝（保留）
  PLATFORM_MACOS: '#A8B1C1', // macOS：高级浅灰
  PLATFORM_LINUX: '#E95420', // Linux：官方橙色（更标准）
  PLATFORM_MINI_PROGRAM: '#07C160', // 小程序：微信绿（保留）
  PLATFORM_OFFICIAL_ACCOUNT: '#52C41A', // 公众号：更清新的绿（区分小程序）
  DEFAULT: '#86909C', // 默认中性灰
} as const;

export function platformToColor(platform?: Platform) {
  return (
    PLATFORM_COLOR_MAP[platform as keyof typeof PLATFORM_COLOR_MAP] ||
    PLATFORM_COLOR_MAP.DEFAULT
  );
}

export function platformToName(platform?: Platform) {
  const values = platformList.value;
  const matchedItem = values.find((item) => item.value === platform);
  return matchedItem ? matchedItem.label : '';
}

export const applicationTypeList = computed(() => [
  { value: 'GAME', label: $t('enum.application.type.GAME') },
  { value: 'ECOMMERCE', label: $t('enum.application.type.ECOMMERCE') },
  { value: 'CONTENT', label: $t('enum.application.type.CONTENT') },
  { value: 'TOOL', label: $t('enum.application.type.TOOL') },
  { value: 'FINANCE', label: $t('enum.application.type.FINANCE') },
  { value: 'SOCIAL', label: $t('enum.application.type.SOCIAL') },
  { value: 'EDUCATION', label: $t('enum.application.type.EDUCATION') },
  { value: 'OTHER', label: $t('enum.application.type.OTHER') },
]);

const APPLICATION_TYPE_COLOR_MAP = {
  GAME: '#4E6CFE', // 游戏：更舒服的科技蓝
  ECOMMERCE: '#FF4D4F', // 电商：活力红（行业标准）
  CONTENT: '#20C997', // 内容：清新绿
  TOOL: '#4096FF', // 工具：稳定蓝
  FINANCE: '#00B42A', // 金融：安全绿
  SOCIAL: '#FF7D00', // 社交：活力橙
  EDUCATION: '#165DFF', // 教育：科技蓝
  OTHER: '#86909C', // 其他：中性灰
  DEFAULT: '#A8B1C1', // 默认：浅灰（更柔和）
} as const;

export function applicationTypeToColor(type?: Application_Type) {
  return (
    APPLICATION_TYPE_COLOR_MAP[
      type as keyof typeof APPLICATION_TYPE_COLOR_MAP
    ] || APPLICATION_TYPE_COLOR_MAP.DEFAULT
  );
}

export function applicationTypeToName(type?: Application_Type) {
  const values = applicationTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : '';
}
