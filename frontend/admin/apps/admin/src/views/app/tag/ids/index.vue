<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_IDMapping as IDMapping } from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  enableBoolToColor,
  enableBoolToName,
  idMappingIdTypeToColor,
  idTypeDict,
  idTypeToName,
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
        options: idTypeDict(),
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
      align: 'left',
    },
    {
      title: $t('page.idMapping.idType'),
      field: 'idType',
      minWidth: 120,
      slots: { default: 'idType' },
    },
    {
      title: $t('page.idMapping.idValue'),
      field: 'idValue',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.idMapping.confidence'),
      field: 'confidence',
      minWidth: 100,
    },
    {
      title: $t('page.idMapping.linkSource'),
      field: 'linkSource',
      minWidth: 150,
      align: 'left',
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
    {
      title: $t('page.idMapping.isActive'),
      field: 'isActive',
      minWidth: 150,
      slots: { default: 'isActive' },
    },

    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<IDMapping> = {};

const [Grid] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.tag.ids')">
      <template #idType="{ row }">
        <a-tag :color="idMappingIdTypeToColor(row.idType)">
          {{ idTypeToName(row.idType) }}
        </a-tag>
      </template>
      <template #isActive="{ row }">
        <a-tag :color="enableBoolToColor(row.isActive)">
          {{ enableBoolToName(row.isActive) }}
        </a-tag>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
