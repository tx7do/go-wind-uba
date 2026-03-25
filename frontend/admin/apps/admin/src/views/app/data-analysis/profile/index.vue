<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_UserBehaviorProfile as UserBehaviorProfile } from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useUserBehaviorProfileListStore } from '#/stores';

const userBehaviorProfileListStore = useUserBehaviorProfileListStore();

const formOptions = {
  collapsed: true,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('ui.formLabel.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'username',
      label: $t('ui.formLabel.username'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
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
    { title: $t('ui.field.userId'), field: 'userId', width: 100 },
    { title: $t('ui.field.username'), field: 'username', width: 120 },
    { title: $t('ui.field.behaviorData'), field: 'behaviorData', width: 300 },
    { title: $t('ui.field.profileTags'), field: 'profileTags', width: 200 },
    {
      title: $t('ui.field.lastActiveAt'),
      field: 'lastActiveAt',
      formatter: 'formatDateTime',
      width: 160,
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      width: 160,
    },
    {
      title: $t('ui.table.updatedAt'),
      field: 'updatedAt',
      formatter: 'formatDateTime',
      width: 160,
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
    <Grid :title="$t('menu.dataAnalysis.userBehaviorProfile')" />
  </Page>
</template>

<style scoped></style>
