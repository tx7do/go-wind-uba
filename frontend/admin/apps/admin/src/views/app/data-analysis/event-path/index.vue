<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_EventPath as EventPath } from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useEventPathListStore } from '#/stores';

const eventPathListStore = useEventPathListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'id',
      label: $t('page.eventPath.id'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.eventPath.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'session_id',
      label: $t('page.eventPath.sessionId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<EventPath> = {
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
        return await eventPathListStore.listEventPath(
          {
            page: page.currentPage,
            pageSize: page.pageSize,
          },
          formValues,
          undefined,
          ['-start_time'],
        );
      },
    },
  },
  columns: [
    {
      title: $t('page.eventPath.id'),
      field: 'id',
      minWidth: 200,
      fixed: 'left',
    },
    { title: $t('page.eventPath.userId'), field: 'userId', minWidth: 100 },
    {
      title: $t('page.eventPath.sessionId'),
      field: 'sessionId',
      minWidth: 100,
    },
    {
      title: $t('page.eventPath.totalDurationMs'),
      field: 'totalDurationMs',
      minWidth: 150,
    },
    {
      title: $t('page.eventPath.stepCount'),
      field: 'stepCount',
      minWidth: 200,
    },
    {
      title: $t('page.eventPath.firstEvent'),
      field: 'firstEvent',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.lastEvent'),
      field: 'lastEvent',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.conversionEvent'),
      field: 'conversionEvent',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.isConverted'),
      field: 'isConverted',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.conversionTime'),
      field: 'conversionTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.startTime'),
      field: 'startTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.endTime'),
      field: 'endTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<EventPath> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});
</script>

<template>
  <Page auto-content-height>
    <Grid :title="$t('menu.dataAnalysis.eventPath')" />
  </Page>
</template>

<style scoped></style>
