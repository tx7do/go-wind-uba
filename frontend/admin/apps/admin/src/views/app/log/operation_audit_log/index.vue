<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { h, ref } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from '@vben/common-ui';
import { LucideEye } from '@vben/icons';

import dayjs from 'dayjs';

import { message } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  fetchListOperationAuditLogs,
  operationAuditLogActionList,
  operationAuditLogActionToColor,
  operationAuditLogActionToName,
  PaginationQuery,
  successStatusList,
  successToColor,
  successToNameWithStatusCode,
  serverExportFile,
} from '#/api';
import { type auditservicev1_OperationAuditLog as OperationAuditLog } from '#/generated/api/admin/service/v1';
import { $t } from '#/locales';
import TableActions from '#/components/TableActions.vue';
import type { TableAction } from '#/components/TableActions.vue';
import { downloadTableFile } from '#/utils/csv';

import OperationAuditLogDetailDrawer from './operation-audit-log-detail-drawer.vue';

const formOptions: VbenFormProps = {
  // 默认展开
  collapsed: false,
  // 控制表单是否显示折叠按钮
  showCollapseButton: false,
  // 按下回车时是否提交表单
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'username',
      label: $t('page.operationAuditLog.username'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'resourceType',
      label: $t('page.operationAuditLog.resourceType'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'action',
      label: $t('page.operationAuditLog.action'),
      componentProps: {
        options: operationAuditLogActionList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'ipAddress',
      label: $t('page.operationAuditLog.ipAddress'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'success',
      label: $t('page.operationAuditLog.success'),
      componentProps: {
        options: successStatusList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'RangePicker',
      fieldName: 'createdAt',
      label: $t('page.operationAuditLog.createdAt'),
      componentProps: {
        showTime: true,
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<OperationAuditLog> = {
  toolbarConfig: {
    custom: true,
    export: true,
    // import: true,
    refresh: true,
    zoom: true,
  },
  height: 'auto',
  exportConfig: {},
  pagerConfig: {},
  rowConfig: {
    isHover: true,
  },
  stripe: true,

  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        let startTime: any;
        let endTime: any;
        if (
          formValues.createdAt !== undefined &&
          formValues.createdAt.length === 2
        ) {
          startTime = dayjs(formValues.createdAt[0]).format(
            'YYYY-MM-DD HH:mm:ss',
          );
          endTime = dayjs(formValues.createdAt[1]).format(
            'YYYY-MM-DD HH:mm:ss',
          );
        }

        return await fetchListOperationAuditLogs(
          new PaginationQuery({
            paging: { page: page.currentPage, pageSize: page.pageSize },
            formValues: {
              username: formValues.username,
              resourceType: formValues.resourceType,
              action: formValues.action,
              ipAddress: formValues.ipAddress,
              success: formValues.success,
              created_at__gte: startTime,
              created_at__lte: endTime,
            },
            orderBy: ['-created_at'],
          }),
        );
      },
    },
  },

  columns: [
    {
      title: $t('page.operationAuditLog.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      width: 140,
    },
    {
      title: $t('page.operationAuditLog.success'),
      field: 'success',
      slots: { default: 'success' },
      width: 80,
    },
    {
      title: $t('page.operationAuditLog.action'),
      field: 'action',
      slots: { default: 'action' },
      width: 80,
    },
    { title: $t('page.operationAuditLog.resourceType'), field: 'resourceType' },
    { title: $t('page.operationAuditLog.resourceId'), field: 'resourceId' },
    {
      title: $t('page.operationAuditLog.requestId'),
      field: 'requestId',
    },
    { title: $t('page.operationAuditLog.username'), field: 'username' },
    {
      title: $t('page.operationAuditLog.geoLocation'),
      field: 'geoLocation',
      slots: { default: 'geoLocation' },
    },
    {
      title: $t('page.operationAuditLog.ipAddress'),
      field: 'ipAddress',
      width: 140,
    },
    {
      title: $t('ui.table.action'),
      field: 'operation',
      fixed: 'right',
      slots: { default: 'action' },
      width: 100,
    },
  ],
};

// 当前搜索条件（服务端导出透传用，与 Grid proxy 查询同源——"导出的就是当前搜索看到的"）
const latestFormValuesRef = ref<Record<string, unknown>>({});

// 服务端全量导出（后端 :export 手动锚点；权限/租户隔离与列表接口同语义）。
async function handleServerExport() {
  try {
    await serverExportFile(
      'admin/v1/operation-audit-logs:export',
      new PaginationQuery({ formValues: latestFormValuesRef.value }),
    );
    message.success($t('page.auditExport.serverSuccess'));
  } catch (error: any) {
    // 原始错误必须留在控制台：用户可见的那句翻译不包含服务端的原因
    console.error('[export] server-side export failed', error);
    message.error(error?.message || $t('page.auditExport.serverFailed'));
  }
}


const exporting = ref(false);

const [Grid] = useVbenVxeGrid({ gridOptions, formOptions });

const [Drawer, drawerApi] = useVbenDrawer({
  connectedComponent: OperationAuditLogDetailDrawer,
});

function handleView(row: OperationAuditLog) {
  drawerApi.setData({ row });
  drawerApi.open();
}

async function handleExportAll(info: { key: string | number }) {
  const format = String(info.key);
  if (format === 'server-xlsx') {
    await handleServerExport();
    return;
  }
  if (format !== 'csv' && format !== 'xlsx') return;
  exporting.value = true;
  try {
    const pageSize = 1000;
    const maxRows = 10000;
    const rows: any[] = [];
    for (let p = 1; rows.length < maxRows; p++) {
      const resp = await fetchListOperationAuditLogs(
        new PaginationQuery({
          paging: { page: p, pageSize },
          // 客户端聚合与服务端导出同口径：透传当前搜索条件——导出的即当前搜索看到的
          formValues: latestFormValuesRef.value,
        }),
      );
      const items = (resp.items ?? []) as any[];
      rows.push(...items);
      if (items.length < pageSize) break;
    }
    const gridColumns = (gridOptions.columns ?? []) as any[];
    const cols = gridColumns
      .filter((c) => c.field)
      .map((c) => ({ title: c.title as string, key: c.field as string }));
    const sliced = rows.slice(0, maxRows);
    await downloadTableFile(format, `operation-audit-logs-${Date.now()}`, cols, sliced);
    message.success($t('ui.export.success', { count: sliced.length }));
  } catch (error: any) {
    // 原始错误必须留在控制台：翻译文案不含原因
    console.error('[export] client-side export failed', error);
    message.error(error?.message || $t('ui.export.failed'));
  } finally {
    exporting.value = false;
  }
}


/* 操作列描述符：单详情钮（定式由 TableActions 统一执行） */
function buildRowActions(): TableAction[] {
  return [
    {
      key: 'detail',
      label: $t('ui.button.detail'),
      icon: h(LucideEye),
      onClick: handleView,
    },
  ];
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.log.operationAuditLog')">
      <template #toolbar-tools>
        <a-dropdown class="mr-2">
          <a-button :loading="exporting">{{ $t('ui.button.exportAll') }}</a-button>
          <template #overlay>
            <a-menu @click="handleExportAll">
              <a-menu-item key="csv">{{ $t('ui.button.exportFormatCsv') }}</a-menu-item>
              <a-menu-item key="xlsx">{{ $t('ui.button.exportFormatXlsx') }}</a-menu-item>
              <a-menu-divider />
              <a-menu-item key="server-xlsx">{{ $t('page.auditExport.serverFull') }}</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </template>
      <template #success="{ row }">
        <a-tag :color="successToColor(row.success)">
          {{ successToNameWithStatusCode(row.success, row.statusCode) }}
        </a-tag>
      </template>
      <template #geoLocation="{ row }">
        {{ row.geoLocation?.province || '-' }} {{ row.geoLocation?.city }}
      </template>
      <template #action="{ row }">
        <a-tag :color="operationAuditLogActionToColor(row.action)">
          {{ operationAuditLogActionToName(row.action) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <TableActions :row="row" :actions="buildRowActions()" />
      </template>
    </Grid>
  <Drawer />
  </Page>
</template>
