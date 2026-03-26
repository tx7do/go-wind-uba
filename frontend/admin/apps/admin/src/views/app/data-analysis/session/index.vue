<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_Session as Session } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
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
    { title: $t('page.session.id'), field: 'id', minWidth: 200, fixed: 'left' },
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
    { title: $t('page.session.isBounce'), field: 'isBounce', minWidth: 100 },
    {
      title: $t('page.session.platform'),
      field: 'platform',
      minWidth: 100,
      slots: { default: 'platform' },
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
      slots: { default: 'riskLevel' },
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
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      minWidth: 120,
    },
  ],
};

const gridEvents: VxeGridListeners<Session> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('handleDelete', row);
  // try {
  //   await sessionListStore.deleteSession(row.id);
  //   notification.success({
  //     message: $t('ui.notification.delete_success'),
  //   });
  //   await gridApi.reload();
  // } catch {
  //   notification.error({
  //     message: $t('ui.notification.delete_failed'),
  //   });
  // }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :title="$t('menu.dataAnalysis.session')">
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
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.dataAnalysis.session'),
            })
          "
          @confirm="handleDelete(row)"
        >
          <a-button danger type="link" :icon="h(LucideTrash2)" />
        </a-popconfirm>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
