<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_UserTag as UserTag } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page, useVbenDrawer } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2 } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  useDictStore,
  userTagSourceToColor,
  userTagSourceToName,
  useUserTagListStore,
} from '#/stores';

import UserTagDrawer from './user-tag-drawer.vue';

const userTagListStore = useUserTagListStore();
const dictStore = useDictStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.userTag.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'tag_id',
      label: $t('page.userTag.tagId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'value_label',
      label: $t('page.userTag.valueLabel'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'source',
      label: $t('page.userTag.source'),
      componentProps: {
        options: dictStore.getDictEntriesOptionsByTypeCode('TAG_SOURCE'),
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<UserTag> = {
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
        return await userTagListStore.listUserTag(
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
      title: $t('page.userTag.valueLabel'),
      field: 'valueLabel',
      minWidth: 150,
      align: 'left',
      fixed: 'left',
    },
    {
      title: $t('page.userTag.value'),
      field: 'value',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.userTag.userId'),
      field: 'userId',
      minWidth: 100,
    },
    { title: $t('page.userTag.tagId'), field: 'tagId', minWidth: 100 },

    {
      title: $t('page.userTag.confidence'),
      field: 'confidence',
      minWidth: 100,
    },
    {
      title: $t('page.userTag.source'),
      field: 'source',
      minWidth: 160,
      align: 'left',
      slots: {
        default: 'source',
      },
    },
    {
      title: $t('page.userTag.effectiveTime'),
      field: 'effectiveTime',
      formatter: 'formatDateTime',
      minWidth: 120,
    },
    {
      title: $t('page.userTag.expireTime'),
      field: 'expireTime',
      formatter: 'formatDateTime',
      minWidth: 120,
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      minWidth: 120,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 90,
    },
  ],
};

const gridEvents: VxeGridListeners<UserTag> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

const [Drawer, drawerApi] = useVbenDrawer({
  // 连接抽离的组件
  connectedComponent: UserTagDrawer,

  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      // 关闭时，重载表格数据
      gridApi.reload();
    }
  },
});

/* 打开模态窗口 */
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

async function handleDelete(row: any) {
  try {
    await userTagListStore.deleteUserTag(row.id);
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
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.tag.userTags')">
      <template #toolbar-tools>
        <a-button type="primary" class="mr-2" @click="handleCreate">
          {{ $t('page.userTag.button.create') }}
        </a-button>
      </template>
      <template #source="{ row }">
        <a-tag :color="userTagSourceToColor(row.source)">
          {{ userTagSourceToName(row.source) }}
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
              moduleName: $t('page.loginPolicy.moduleName'),
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
