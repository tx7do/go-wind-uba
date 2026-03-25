<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_RiskEvent as RiskEvent } from '#/generated/api/admin/service/v1';

import { h } from 'vue';

import { Page } from '@vben/common-ui';
import { LucideTrash2 } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  riskEventStatusToColor,
  riskEventStatusToName,
  riskEventTypeToColor,
  riskEventTypeToName,
  riskLevelToColor,
  riskLevelToName,
  useRiskEventListStore,
} from '#/stores';

const riskEventListStore = useRiskEventListStore();

const formOptions = {
  collapsed: true,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('ui.formLabel.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: $t('enum.riskEvent.type.title'),
      componentProps: {
        options: [
          {
            value: 'RISK_TYPE_LOGIN_ABNORMAL',
            label: $t('ui.riskEventType.RISK_TYPE_LOGIN_ABNORMAL'),
          },
          {
            value: 'RISK_TYPE_PERMISSION_CHANGE',
            label: $t('ui.riskEventType.RISK_TYPE_PERMISSION_CHANGE'),
          },
          {
            value: 'RISK_TYPE_DATA_ACCESS',
            label: $t('ui.riskEventType.RISK_TYPE_DATA_ACCESS'),
          },
        ],
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'level',
      label: $t('enum.riskEvent.level.title'),
      componentProps: {
        options: [
          { value: 'LEVEL_LOW', label: $t('ui.riskLevel.LEVEL_LOW') },
          { value: 'LEVEL_MEDIUM', label: $t('ui.riskLevel.LEVEL_MEDIUM') },
          { value: 'LEVEL_HIGH', label: $t('ui.riskLevel.LEVEL_HIGH') },
        ],
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
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
    { title: $t('ui.field.id'), field: 'id', width: 100 },
    { title: $t('ui.field.userId'), field: 'userId', width: 100 },
    {
      title: $t('enum.riskEvent.type.title'),
      field: 'type',
      width: 140,
      slots: { default: 'type' },
    },
    {
      title: $t('enum.riskEvent.level.title'),
      field: 'level',
      width: 100,
      slots: { default: 'level' },
    },
    {
      title: $t('enum.riskEvent.status.title'),
      field: 'status',
      width: 100,
      slots: { default: 'status' },
    },
    { title: $t('ui.field.description'), field: 'description', width: 250 },
    {
      title: $t('ui.field.occurredAt'),
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
      <template #type="{ row }">
        <a-tag :color="riskEventTypeToColor(row.type)">
          {{ riskEventTypeToName(row.type) }}
        </a-tag>
      </template>
      <template #level="{ row }">
        <a-tag :color="riskLevelToColor(row.level)">
          {{ riskLevelToName(row.level) }}
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
              moduleName: $t('menu.page.riskEvent'),
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
