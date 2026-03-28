<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_RiskRule as RiskRule } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page, useVbenDrawer } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2 } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  riskEventTypeToColor,
  riskLevelToColor,
  riskLevelToName, riskTypeDict,
  riskTypeToName,
  useRiskRuleListStore,
} from '#/stores';

import RiskRuleDrawer from './risk-rule-drawer.vue';

const riskRuleListStore = useRiskRuleListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('page.riskRule.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'code',
      label: $t('page.riskRule.code'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'riskType',
      label: $t('page.riskRule.riskType'),
      componentProps: {
        options: riskTypeDict(),
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<RiskRule> = {
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
        return await riskRuleListStore.listRiskRule(
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
      title: $t('page.riskRule.name'),
      field: 'name',
      minWidth: 150,
      align: 'left',
      fixed: 'left',
    },
    {
      title: $t('page.riskRule.description'),
      field: 'description',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.riskRule.code'),
      field: 'code',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.riskRule.riskType'),
      field: 'riskType',
      minWidth: 160,
      align: 'left',
      slots: { default: 'riskType' },
    },
    {
      title: $t('page.riskRule.defaultLevel'),
      field: 'defaultLevel',
      minWidth: 160,
      align: 'left',
      slots: { default: 'riskLevel' },
    },
    { title: $t('page.riskRule.priority'), field: 'priority', minWidth: 80 },
    {
      title: $t('page.riskRule.enabled'),
      field: 'enabled',
      minWidth: 90,
      slots: { default: 'enabled' },
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

const gridEvents: VxeGridListeners<RiskRule> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

const [Drawer, drawerApi] = useVbenDrawer({
  // 连接抽离的组件
  connectedComponent: RiskRuleDrawer,

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

async function handleDelete(row: any) {
  try {
    await riskRuleListStore.deleteRiskRule(row.id);
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
    <Grid :table-title="$t('menu.risk.rule')">
      <template #toolbar-tools>
        <a-button type="primary" @click="handleCreate">
          {{ $t('ui.button.create', { moduleName: $t('menu.risk.rule') }) }}
        </a-button>
      </template>
      <template #riskType="{ row }">
        <a-tag :color="riskEventTypeToColor(row.riskType)">
          {{ riskTypeToName(row.riskType) }}
        </a-tag>
      </template>
      <template #riskLevel="{ row }">
        <a-tag :color="riskLevelToColor(row.defaultLevel)">
          {{ riskLevelToName(row.defaultLevel) }}
        </a-tag>
      </template>
      <template #enabled="{ row }">
        <a-tag :color="row.enabled ? '#52C41A' : '#8C8C8C'">
          {{ row.enabled ? $t('ui.switch.enable') : $t('ui.switch.disable') }}
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
              moduleName: $t('menu.risk.rule'),
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
