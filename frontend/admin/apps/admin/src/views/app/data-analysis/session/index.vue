<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_Session as Session } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useSessionListStore } from '#/stores';

const sessionListStore = useSessionListStore();

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
    { title: $t('ui.field.sessionId'), field: 'id', width: 200 },
    { title: $t('ui.field.userId'), field: 'userId', width: 100 },
    { title: $t('ui.field.username'), field: 'username', width: 120 },
    { title: $t('ui.field.ipAddress'), field: 'ip', width: 140 },
    { title: $t('ui.field.userAgent'), field: 'userAgent', width: 200 },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      width: 160,
    },
    {
      title: $t('ui.field.expiresAt'),
      field: 'expiresAt',
      formatter: 'formatDateTime',
      width: 160,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 120,
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
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.page.session'),
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
