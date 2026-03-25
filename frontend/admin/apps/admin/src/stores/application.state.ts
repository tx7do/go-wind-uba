import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  type ubaservicev1_Application_Status as Application_Status,
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
    return await service.ListApplication({
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
    return await service.GetApplication({ id });
  }

  /**
   * 创建应用
   */
  async function createApplication(values: Record<string, any> = {}) {
    return await service.CreateApplication({
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

    return await service.UpdateApplication({
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
    return await service.DeleteApplication({ id });
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

export const applicationStatusList = computed(() => [
  {
    value: 'STATUS_UNSPECIFIED',
    label: $t('enum.application.status.UNSPECIFIED'),
  },
  { value: 'STATUS_ACTIVE', label: $t('enum.application.status.ACTIVE') },
  { value: 'STATUS_INACTIVE', label: $t('enum.application.status.INACTIVE') },
  { value: 'STATUS_DISABLED', label: $t('enum.application.status.DISABLED') },
]);

const APPLICATION_STATUS_COLOR_MAP = {
  STATUS_UNSPECIFIED: '#86909C',
  STATUS_ACTIVE: '#4096FF',
  STATUS_INACTIVE: '#C9CDD4',
  STATUS_DISABLED: '#F53F3F',
  DEFAULT: '#86909C',
} as const;

export function applicationStatusToColor(status: Application_Status) {
  return (
    APPLICATION_STATUS_COLOR_MAP[
      status as keyof typeof APPLICATION_STATUS_COLOR_MAP
    ] || APPLICATION_STATUS_COLOR_MAP.DEFAULT
  );
}

export function applicationStatusToName(status?: Application_Status) {
  const values = applicationStatusList.value;
  const matchedItem = values.find((item) => item.value === status);
  return matchedItem ? matchedItem.label : '';
}

export const platformList = computed(() => [
  { value: 'PLATFORM_UNSPECIFIED', label: $t('enum.platform.UNSPECIFIED') },
  { value: 'PLATFORM_WEB', label: $t('enum.platform.WEB') },
  { value: 'PLATFORM_IOS', label: $t('enum.platform.IOS') },
  { value: 'PLATFORM_ANDROID', label: $t('enum.platform.ANDROID') },
  { value: 'PLATFORM_WINDOWS', label: $t('enum.platform.WINDOWS') },
  { value: 'PLATFORM_MACOS', label: $t('enum.platform.MACOS') },
  { value: 'PLATFORM_LINUX', label: $t('enum.platform.LINUX') },
  { value: 'PLATFORM_MINI_PROGRAM', label: $t('enum.platform.MINI_PROGRAM') },
  {
    value: 'PLATFORM_OFFICIAL_ACCOUNT',
    label: $t('enum.platform.OFFICIAL_ACCOUNT'),
  },
]);

const PLATFORM_COLOR_MAP = {
  PLATFORM_UNSPECIFIED: '#86909C',
  PLATFORM_WEB: '#4096FF',
  PLATFORM_IOS: '#000000',
  PLATFORM_ANDROID: '#3DDC84',
  PLATFORM_WINDOWS: '#0078D4',
  PLATFORM_MACOS: '#A2AAAD',
  PLATFORM_LINUX: '#FCC624',
  PLATFORM_MINI_PROGRAM: '#07C160',
  PLATFORM_OFFICIAL_ACCOUNT: '#07C160',
  DEFAULT: '#86909C',
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
