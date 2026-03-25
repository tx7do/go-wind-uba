<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_TagDefinition as TagDefinition } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2 } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  tagCategoryToColor,
  tagCategoryToName,
  tagTypeToColor,
  tagTypeToName,
  useTagDefinitionListStore,
} from '#/stores';

const tagDefinitionListStore = useTagDefinitionListStore();

const formOptions = {
  collapsed: true,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('ui.formLabel.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'category',
      label: $t('ui.field.category'),
      componentProps: {
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<TagDefinition> = {
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
        return await tagDefinitionListStore.listTagDefinition(
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
    { title: $t('ui.field.name'), field: 'name', width: 150 },
    { title: $t('ui.field.code'), field: 'code', width: 150 },
    {
      title: $t('ui.field.category'),
      field: 'category',
      width: 120,
      slots: { default: 'category' },
    },
    {
      title: $t('ui.field.type'),
      field: 'type',
      width: 120,
      slots: { default: 'type' },
    },
    { title: $t('ui.field.defaultValue'), field: 'defaultValue', width: 120 },
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

const gridEvents: VxeGridListeners<TagDefinition> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('Delete', row);
  try {
    await tagDefinitionListStore.deleteTagDefinition(row.id);
    notification.success({
      message: $t('ui.notification.delete_success'),
    });
    await gridApi.reload();
  } catch {
    notification.error({
      message: $t('ui.notification.delete_failed'),
    });
  }
}

function handleEdit(row: any) {
  console.log('Edit', row);
}
</script>

<template>
  <Page auto-content-height>
    <Grid :title="$t('menu.tag.tags')">
      <template #category="{ row }">
        <a-tag :color="tagCategoryToColor(row.category)">
          {{ tagCategoryToName(row.category) }}
        </a-tag>
      </template>
      <template #type="{ row }">
        <a-tag :color="tagTypeToColor(row.type)">
          {{ tagTypeToName(row.type) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <a-button
          type="link"
          :icon="h(LucideFilePenLine)"
          @click.stop="handleEdit(row)"
        />
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.page.tagDefinition'),
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
