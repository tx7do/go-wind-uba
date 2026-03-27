<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_Session as Session } from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  enableBoolToColor,
  enableBoolToName,
  platformToColor,
  platformToName,
  riskLevelToColor,
  riskLevelToName,
  useSessionListStore,
} from '#/stores';

const sessionListStore = useSessionListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'session_id',
      label: $t('page.session.sessionId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.session.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'device_id',
      label: $t('page.session.deviceId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'global_user_id',
      label: $t('page.session.globalUserId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Session> = {
  height: 'auto',
  stripe: true,
  autoResize: true,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
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
        return await sessionListStore.listSession(
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
      title: $t('page.session.sessionId'),
      field: 'sessionId',
      minWidth: 200,
      fixed: 'left',
    },
    { title: $t('page.session.userId'), field: 'userId', minWidth: 100 },
    {
      title: $t('page.session.deviceId'),
      field: 'deviceId',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.session.globalUserId'),
      field: 'globalUserId',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.session.durationMs'),
      field: 'durationMs',
      minWidth: 100,
    },
    {
      title: $t('page.session.eventCount'),
      field: 'eventCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.pageViewCount'),
      field: 'pageViewCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.actionCount'),
      field: 'actionCount',
      minWidth: 100,
    },
    { title: $t('page.session.entryPage'), field: 'entryPage', minWidth: 120 },
    { title: $t('page.session.exitPage'), field: 'exitPage', minWidth: 120 },
    {
      title: $t('page.session.isBounce'),
      field: 'isBounce',
      minWidth: 100,
      slots: { default: 'isBounce' },
    },
    {
      title: $t('page.session.platform'),
      field: 'platform',
      minWidth: 100,
    },
    { title: $t('page.session.os'), field: 'os', minWidth: 100 },
    {
      title: $t('page.session.appVersion'),
      field: 'appVersion',
      minWidth: 100,
    },
    { title: $t('page.session.ipCity'), field: 'ipCity', minWidth: 100 },
    { title: $t('page.session.country'), field: 'country', minWidth: 100 },
    {
      title: $t('page.session.totalAmount'),
      field: 'totalAmount',
      minWidth: 100,
    },
    {
      title: $t('page.session.payEventCount'),
      field: 'payEventCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.riskLevel'),
      field: 'riskLevel',
      minWidth: 100,
    },
    {
      title: $t('page.session.startTime'),
      field: 'startTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.session.endTime'),
      field: 'endTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<Session> = {};

const [Grid] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.dataAnalysis.session')">
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
      <template #isBounce="{ row }">
        <a-tag :color="enableBoolToColor(row.isBounce)">
          {{ enableBoolToName(row.isBounce) }}
        </a-tag>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
