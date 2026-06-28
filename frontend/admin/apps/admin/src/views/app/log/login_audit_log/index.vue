<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, type VbenFormProps } from '@vben/common-ui';

import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  fetchListLoginAuditLogs,
  getLoginAuditLogActionTypeColor,
  getLoginAuditLogRiskLevelColor,
  getLoginAuditLogStatusColor,
  loginAuditLogActionTypeList,
  loginAuditLogActionTypeToName,
  loginAuditLogRiskLevelList,
  loginAuditLogRiskLevelToName,
  loginAuditLogStatusList,
  loginAuditLogStatusToName,
  PaginationQuery,
} from '#/api';
import { type auditservicev1_LoginAuditLog as LoginAuditLog } from '#/generated/api/admin/service/v1';
import { $t } from '#/locales';

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
      label: $t('page.loginAuditLog.username'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'ipAddress',
      label: $t('page.loginAuditLog.ipAddress'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'actionType',
      label: $t('page.loginAuditLog.actionType'),
      componentProps: {
        options: loginAuditLogActionTypeList,
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
      label: $t('page.loginAuditLog.riskLevel'),
      componentProps: {
        options: loginAuditLogRiskLevelList,
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('page.loginAuditLog.status'),
      componentProps: {
        options: loginAuditLogStatusList,
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
      label: $t('page.loginAuditLog.createdAt'),
      componentProps: {
        showTime: true,
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<LoginAuditLog> = {
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

        return await fetchListLoginAuditLogs(
          new PaginationQuery({
            paging: { page: page.currentPage, pageSize: page.pageSize },
            formValues: {
              username: formValues.username,
              ipAddress: formValues.ipAddress,
              status: formValues.status,
              actionType: formValues.actionType,
              riskLevel: formValues.riskLevel,
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
      title: $t('page.loginAuditLog.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      width: 140,
    },
    {
      title: $t('page.loginAuditLog.status'),
      field: 'status',
      width: 80,
      slots: { default: 'status' },
    },
    { title: $t('page.loginAuditLog.username'), field: 'username' },
    {
      title: $t('page.loginAuditLog.actionType'),
      field: 'actionType',
      slots: { default: 'actionType' },
    },
    {
      title: $t('page.loginAuditLog.riskLevel'),
      field: 'riskLevel',
      slots: { default: 'riskLevel' },
    },
    {
      title: $t('page.loginAuditLog.platform'),
      field: 'deviceInfo.platform',
      slots: { default: 'platform' },
    },
    {
      title: $t('page.loginAuditLog.geoLocation'),
      field: 'geoLocation',
      slots: { default: 'geoLocation' },
    },
    {
      title: $t('page.loginAuditLog.ipAddress'),
      field: 'ipAddress',
      width: 140,
    },
  ],
};

const [Grid] = useVbenVxeGrid({ gridOptions, formOptions });
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.log.loginAuditLog')">
      <template #status="{ row }">
        <a-tag :color="getLoginAuditLogStatusColor(row.status)">
          {{ loginAuditLogStatusToName(row.status) }}
        </a-tag>
      </template>
      <template #actionType="{ row }">
        <a-tag :color="getLoginAuditLogActionTypeColor(row.actionType)">
          {{ loginAuditLogActionTypeToName(row.actionType) }}
        </a-tag>
      </template>
      <template #riskLevel="{ row }">
        <a-tag :color="getLoginAuditLogRiskLevelColor(row.riskLevel)">
          {{ loginAuditLogRiskLevelToName(row.riskLevel) }}
        </a-tag>
      </template>
      <template #geoLocation="{ row }">
        {{ row.geoLocation?.province || '-' }} {{ row.geoLocation?.city }}
      </template>
      <template #platform="{ row }">
        {{ row.deviceInfo?.osName || '-' }} {{ row.deviceInfo?.browserName }}
      </template>
    </Grid>
  </Page>
</template>
