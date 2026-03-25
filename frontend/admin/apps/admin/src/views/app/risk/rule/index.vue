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
  riskActionTypeToColor,
  riskActionTypeToName,
  useRiskRuleListStore,
} from '#/stores';

const riskRuleListStore = useRiskRuleListStore();

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
      fieldName: 'action_type',
      label: $t('enum.riskRule.actionType.title'),
      componentProps: {
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
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
    { title: $t('ui.field.id'), field: 'id', width: 100 },
    { title: $t('ui.field.name'), field: 'name', width: 150 },
    { title: $t('ui.field.description'), field: 'description', width: 200 },
    {
      title: $t('enum.riskRule.actionType.title'),
      field: 'actionType',
      width: 120,
      slots: { default: 'actionType' },
    },
    { title: $t('ui.field.priority'), field: 'priority', width: 80 },
    {
      title: $t('ui.field.enabled'),
      field: 'enabled',
      width: 90,
      slots: { default: 'enabled' },
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

const gridEvents: VxeGridListeners<RiskRule> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

const [Drawer, drawerApi] = useVbenDrawer();

function openDrawer(create: boolean, row?: any) {
  drawerApi.setData({
    create,
    row,
  });
  drawerApi.open();
}

function handleCreate() {
  openDrawer(true);
}

function handleEdit(row: any) {
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
    <Grid :title="$t('menu.risk.rule')">
      <template #toolbar-tools>
        <a-button type="primary" @click="handleCreate">
          {{ $t('ui.button.create', { moduleName: $t('menu.page.riskRule') }) }}
        </a-button>
      </template>
      <template #actionType="{ row }">
        <a-tag :color="riskActionTypeToColor(row.actionType)">
          {{ riskActionTypeToName(row.actionType) }}
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
              moduleName: $t('menu.page.riskRule'),
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
