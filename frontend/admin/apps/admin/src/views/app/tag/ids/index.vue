<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_IDMapping as IDMapping } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  idMappingIdTypeToColor,
  idMappingIdTypeToName,
  useIdMappingListStore,
} from '#/stores';

const idMappingListStore = useIdMappingListStore();

const formOptions = {
  collapsed: true,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'source_id',
      label: $t('ui.formLabel.sourceId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'source_type',
      label: $t('ui.formLabel.sourceType'),
      componentProps: {
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<IDMapping> = {
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
        return await idMappingListStore.listIDMapping(
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
    { title: $t('ui.field.id'), field: 'id', width: 100 },
    { title: $t('ui.field.sourceId'), field: 'sourceId', width: 150 },
    { title: $t('ui.field.sourceType'), field: 'sourceType', width: 120 },
    { title: $t('ui.field.targetId'), field: 'targetId', width: 150 },
    { title: $t('ui.field.targetType'), field: 'targetType', width: 120 },
    {
      title: $t('ui.field.idType'),
      field: 'idType',
      width: 120,
      slots: { default: 'idType' },
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
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

const gridEvents: VxeGridListeners<IDMapping> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('handleDelete', row);
  // try {
  //   await idMappingListStore.deleteIDMapping(row.id);
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
    <Grid :title="$t('menu.tag.ids')">
      <template #idType="{ row }">
        <a-tag :color="idMappingIdTypeToColor(row.idType)">
          {{ idMappingIdTypeToName(row.idType) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.page.idMapping'),
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
