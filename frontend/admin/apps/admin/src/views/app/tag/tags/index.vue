<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_TagDefinition as TagDefinition } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page, useVbenDrawer } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2 } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  enableBoolToColor,
  enableBoolToName,
  tagCategoryList,
  tagCategoryToColor,
  tagCategoryToName,
  tagTypeList,
  tagTypeToColor,
  tagTypeToName,
  useTagDefinitionListStore,
} from '#/stores';

import TagDrawer from './tag-drawer.vue';

const tagDefinitionListStore = useTagDefinitionListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('page.tagDefinition.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'code',
      label: $t('page.tagDefinition.code'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'category',
      label: $t('page.tagDefinition.category'),
      componentProps: {
        options: tagCategoryList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'tagType',
      label: $t('page.tagDefinition.tagType'),
      componentProps: {
        options: tagTypeList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
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
    {
      title: $t('page.tagDefinition.name'),
      field: 'name',
      minWidth: 150,
      fixed: 'left',
      align: 'left',
    },
    { title: $t('page.tagDefinition.code'), field: 'code', minWidth: 150 },
    {
      title: $t('page.tagDefinition.description'),
      field: 'description',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.tagDefinition.category'),
      field: 'category',
      minWidth: 120,
      slots: { default: 'category' },
    },
    {
      title: $t('page.tagDefinition.tagType'),
      field: 'tagType',
      minWidth: 120,
      slots: { default: 'type' },
    },
    {
      title: $t('page.tagDefinition.isSystem'),
      field: 'isSystem',
      minWidth: 120,
      slots: { default: 'isSystem' },
    },
    {
      title: $t('page.tagDefinition.isDynamic'),
      field: 'isDynamic',
      minWidth: 120,
      slots: { default: 'isDynamic' },
    },
    {
      title: $t('page.tagDefinition.refreshIntervalSeconds'),
      field: 'refreshIntervalSeconds',
      minWidth: 120,
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

const [Drawer, drawerApi] = useVbenDrawer({
  // 连接抽离的组件
  connectedComponent: TagDrawer,

  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      // 关闭时，重载表格数据
      gridApi.reload();
    }
  },
});

function openDrawer(create: boolean, row?: any) {
  drawerApi.setData({
    create,
    row,
  });
  drawerApi.open();
}

/* 创建 */
function handleCreate() {
  console.log('创建');

  openDrawer(true);
}

/* 编辑 */
function handleEdit(row: any) {
  console.log('编辑', row);
  openDrawer(false, row);
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.tag.tags')">
      <template #toolbar-tools>
        <a-button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('page.tagDefinition.button.create') }}
        </a-button>
      </template>
      <template #category="{ row }">
        <a-tag :color="tagCategoryToColor(row.category)">
          {{ tagCategoryToName(row.category) }}
        </a-tag>
      </template>
      <template #type="{ row }">
        <a-tag :color="tagTypeToColor(row.tagType)">
          {{ tagTypeToName(row.tagType) }}
        </a-tag>
      </template>
      <template #isSystem="{ row }">
        <a-tag :color="enableBoolToColor(row.isSystem)">
          {{ enableBoolToName(row.isSystem) }}
        </a-tag>
      </template>
      <template #isDynamic="{ row }">
        <a-tag :color="enableBoolToColor(row.isDynamic)">
          {{ enableBoolToName(row.isDynamic) }}
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
              moduleName: $t('menu.tag.tags'),
            })
          "
          @confirm="handleDelete(row)"
        >
          <a-button danger type="link" :icon="h(LucideTrash2)" />
        </a-popconfirm>
      </template>
    </Grid>
    <Drawer />
  </Page>
</template>

<style scoped></style>
