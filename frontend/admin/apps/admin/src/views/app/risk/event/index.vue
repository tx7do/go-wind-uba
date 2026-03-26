<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_RiskEvent as RiskEvent } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  riskEventStatusList,
  riskEventStatusToColor,
  riskEventStatusToName,
  riskEventTypeList,
  riskEventTypeToColor,
  riskEventTypeToName,
  riskLevelList,
  riskLevelToColor,
  riskLevelToName,
  useRiskEventListStore,
} from '#/stores';

const riskEventListStore = useRiskEventListStore();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.riskEvent.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('page.riskEvent.status'),
      componentProps: {
        options: riskEventStatusList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'riskType',
      label: $t('page.riskEvent.riskType'),
      componentProps: {
        options: riskEventTypeList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'riskLevel',
      label: $t('page.riskEvent.riskLevel'),
      componentProps: {
        options: riskLevelList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<RiskEvent> = {
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
        return await riskEventListStore.listRiskEvent(
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
      title: $t('page.riskEvent.userId'),
      field: 'userId',
      minWidth: 100,
      fixed: 'left',
    },
    {
      title: $t('page.riskEvent.deviceId'),
      field: 'deviceId',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.riskEvent.globalUserId'),
      field: 'globalUserId',
      minWidth: 150,
      align: 'left',
    },
    {
      title: $t('page.riskEvent.riskType'),
      field: 'riskType',
      minWidth: 140,
      slots: { default: 'riskType' },
    },
    {
      title: $t('page.riskEvent.riskLevel'),
      field: 'riskLevel',
      minWidth: 100,
      slots: { default: 'riskLevel' },
    },
    {
      title: $t('page.riskEvent.status'),
      field: 'status',
      minWidth: 100,
      slots: { default: 'status' },
    },
    {
      title: $t('page.riskEvent.riskScore'),
      field: 'riskScore',
      minWidth: 100,
    },
    {
      title: $t('page.riskEvent.ruleId'),
      field: 'ruleId',
      minWidth: 100,
    },
    {
      title: $t('page.riskEvent.ruleName'),
      field: 'ruleName',
      minWidth: 120,
      align: 'left',
    },
    {
      title: $t('page.riskEvent.description'),
      field: 'description',
      minWidth: 250,
      align: 'left',
    },
    {
      title: $t('page.riskEvent.occurTime'),
      field: 'occurTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.riskEvent.reportTime'),
      field: 'reportTime',
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

const gridEvents: VxeGridListeners<RiskEvent> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

async function handleDelete(row: any) {
  console.log('handleDelete', row);
  // try {
  //   await riskEventListStore.deleteRiskEvent(row.id);
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
    <Grid :title="$t('menu.risk.event')">
      <template #riskType="{ row }">
        <a-tag :color="riskEventTypeToColor(row.riskType)">
          {{ riskEventTypeToName(row.riskType) }}
        </a-tag>
      </template>
      <template #riskLevel="{ row }">
        <a-tag :color="riskLevelToColor(row.riskLevel)">
          {{ riskLevelToName(row.riskLevel) }}
        </a-tag>
      </template>
      <template #status="{ row }">
        <a-tag :color="riskEventStatusToColor(row.status)">
          {{ riskEventStatusToName(row.status) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <a-popconfirm
          :cancel-text="$t('ui.button.cancel')"
          :ok-text="$t('ui.button.ok')"
          :title="
            $t('ui.text.do_you_want_delete', {
              moduleName: $t('menu.risk.event'),
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
