<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_Application as Application } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  applicationTypeToColor,
  applicationTypeToName,
  platformList,
  platformToColor,
  platformToName,
  statusList,
  statusToColor,
  statusToName,
  useApplicationListStore,
} from '#/stores';

const applicationStore = useApplicationListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'appId',
      label: $t('page.application.appId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: $t('page.application.type'),
      componentProps: {
        options: platformList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('ui.table.status'),
      componentProps: {
        options: statusList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Application> = {
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
        return await applicationStore.listApplication(
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
      title: $t('page.application.appId'),
      field: 'appId',
      minWidth: 150,
      fixed: 'left',
      align: 'left',
    },
    {
      title: $t('page.application.appKey'),
      field: 'appKey',
      minWidth: 250,
      align: 'left',
    },
    {
      title: $t('page.application.type'),
      field: 'type',
      minWidth: 200,
      slots: {
        default: 'type',
      },
    },
    {
      title: $t('page.application.status'),
      field: 'status',
      minWidth: 100,
      slots: { default: 'status' },
    },
    {
      title: $t('page.application.platforms'),
      field: 'platforms',
      align: 'left',
      minWidth: 300,
      slots: {
        default: 'platforms',
      },
    },
    {
      title: $t('page.application.remark'),
      field: 'remark',
      minWidth: 200,
    },
    {
      title: $t('page.application.desensitize'),
      field: 'desensitize',
      minWidth: 160,
    },
    {
      title: $t('page.application.webhookUrl'),
      field: 'webhookUrl',
      minWidth: 160,
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
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

const gridEvents: VxeGridListeners<Application> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('handleDelete', row);
  // try {
  //   await applicationStore.deleteApplication(row.id);
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
    <Grid :title="$t('menu.object.objects')">
      <template #type="{ row }">
        <a-tag :color="applicationTypeToColor(row.type)">
          {{ applicationTypeToName(row.type) }}
        </a-tag>
      </template>
      <template #platforms="{ row }">
        <a-tag
          v-for="item in row.platforms"
          :key="item"
          :color="platformToColor(item)"
        >
          {{ platformToName(item) }}
        </a-tag>
      </template>
      <template #status="{ row }">
        <a-tag :color="statusToColor(row.status)">
          {{ statusToName(row.status) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.application.applications'),
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
