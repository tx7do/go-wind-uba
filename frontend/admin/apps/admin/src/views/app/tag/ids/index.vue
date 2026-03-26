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
  idTypeList,
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
      fieldName: 'id_value',
      label: $t('page.idMapping.idValue'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'idType',
      label: $t('page.idMapping.idType'),
      componentProps: {
        options: idTypeList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
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
    {
      title: $t('page.idMapping.globalUserId'),
      field: 'globalUserId',
      minWidth: 200,
      fixed: 'left',
    },
    {
      title: $t('page.idMapping.idType'),
      field: 'idType',
      minWidth: 120,
      slots: { default: 'idType' },
    },
    { title: $t('page.idMapping.idValue'), field: 'idValue', minWidth: 120 },
    {
      title: $t('page.idMapping.confidence'),
      field: 'confidence',
      minWidth: 150,
    },
    {
      title: $t('page.idMapping.linkSource'),
      field: 'linkSource',
      minWidth: 150,
      formatter: 'formatDateTime',
    },
    {
      title: $t('page.idMapping.firstSeen'),
      field: 'firstSeen',
      minWidth: 150,
      formatter: 'formatDateTime',
    },
    {
      title: $t('page.idMapping.lastSeen'),
      field: 'lastSeen',
      minWidth: 150,
      formatter: 'formatDateTime',
    },
    { title: $t('page.idMapping.isActive'), field: 'isActive', minWidth: 150 },

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
    <Grid :table-title="$t('menu.tag.ids')">
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
              moduleName: $t('menu.tag.ids'),
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
