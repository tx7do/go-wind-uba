<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_UserBehaviorProfile as UserBehaviorProfile } from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  platformList,
  platformToColor,
  platformToName,
  riskLevelToColor,
  riskLevelToName, statusList,
  useUserBehaviorProfileListStore,
} from '#/stores';

const userBehaviorProfileListStore = useUserBehaviorProfileListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.userBehaviorProfile.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'platform',
      label: $t('page.userBehaviorProfile.platform'),
      componentProps: {
        options: platformList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<UserBehaviorProfile> = {
  height: 'auto',
  stripe: true,
  autoResize: true,
  toolbarConfig: {
    custom: true,
    export: true,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
  pagerConfig: {},
  rowConfig: {
    isHover: true,
    resizable: true,
  },
  tooltipConfig: {
    showAll: true,
    enterable: true,
  },
  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        return await userBehaviorProfileListStore.listUserBehaviorProfile(
          {
            page: page.currentPage,
            pageSize: page.pageSize,
          },
          formValues,
        );
      },
    },
  },
  columns: [
    {
      title: $t('page.userBehaviorProfile.userId'),
      field: 'userId',
      minWidth: 100,
      fixed: 'left',
    },
    {
      title: $t('page.userBehaviorProfile.registerTime'),
      field: 'registerTime',
      minWidth: 180,
      formatter: 'formatDateTime',
    },
    {
      title: $t('page.userBehaviorProfile.registerChannel'),
      field: 'registerChannel',
      minWidth: 120,
    },
    {
      title: $t('page.userBehaviorProfile.firstActiveDate'),
      field: 'firstActiveDate',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.userBehaviorProfile.lastActiveDate'),
      field: 'lastActiveDate',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.userBehaviorProfile.userLevel'),
      field: 'userLevel',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.vipLevel'),
      field: 'vipLevel',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.totalEvents'),
      field: 'totalEvents',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.totalSessions'),
      field: 'totalSessions',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.totalPayAmount'),
      field: 'totalPayAmount',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.lastPayTime'),
      field: 'lastPayTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.userBehaviorProfile.riskLevel'),
      field: 'riskLevel',
      minWidth: 100,
      slots: {
        default: 'riskLevel',
      },
    },
    {
      title: $t('page.userBehaviorProfile.deviceType'),
      field: 'deviceType',
      minWidth: 100,
    },
    {
      title: $t('page.userBehaviorProfile.platform'),
      field: 'platform',
      minWidth: 100,
      slots: {
        default: 'platform',
      },
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      fixed: 'right',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<UserBehaviorProfile> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});
</script>

<template>
  <Page auto-content-height>
    <Grid :title="$t('menu.dataAnalysis.userBehaviorProfile')">
      <template #platform="{ row }">
        <a-tag :color="platformToColor(row.platform)">
          {{ platformToName(row.platform) }}
        </a-tag>
      </template>
      <template #riskLevel="{ row }">
        <a-tag :color="riskLevelToColor(row.riskLevel)">
          {{ riskLevelToName(row.riskLevel) }}
        </a-tag>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
