<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_ObjectDim as ObjectDim } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useObjectDimListStore } from '#/stores';

const objectDimListStore = useObjectDimListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'objectName',
      label: $t('page.object.objectName'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'objectType',
      label: $t('page.object.objectType'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'status',
      label: $t('page.object.status'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<ObjectDim> = {
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
        return await objectDimListStore.listObjectDim(
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
      title: $t('page.object.id'),
      field: 'id',
      minWidth: 100,
      fixed: 'left',
      align: 'left',
    },
    {
      title: $t('page.object.objectType'),
      field: 'objectType',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.object.objectName'),
      field: 'objectName',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.object.categoryPath'),
      field: 'categoryPath',
      minWidth: 200,
      align: 'left',
    },
    { title: $t('page.object.price'), field: 'price', minWidth: 100 },
    { title: $t('page.object.currency'), field: 'currency', minWidth: 100 },
    { title: $t('page.object.rarity'), field: 'rarity', minWidth: 100 },
    { title: $t('page.object.status'), field: 'status', minWidth: 100 },
    {
      title: $t('page.object.validFrom'),
      field: 'validFrom',
      minWidth: 160,
      formatter: 'formatDateTime',
    },
    {
      title: $t('page.object.validTo'),
      field: 'validTo',
      minWidth: 160,
      formatter: 'formatDateTime',
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

const gridEvents: VxeGridListeners<ObjectDim> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('handleDelete', row);
  // try {
  //   await objectDimListStore.deleteObjectDim(row.id);
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
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.page.object'),
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
