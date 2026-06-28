<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type { ubaservicev1_EventSchema as EventSchema } from '#/generated/api/admin/service/v1';

import { Page, useVbenDrawer } from '@vben/common-ui';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  eventSchemaStatusToColor,
  eventSchemaStatusToName,
  fetchListEventSchemas,
  PaginationQuery,
  useDeleteEventSchema,
} from '#/api';
import { $t } from '#/locales';

import EventSchemaDrawer from './event-schema-drawer.vue';

const { mutateAsync: deleteEventSchema } = useDeleteEventSchema();

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'event_name',
      label: $t('page.analytics.eventName'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'category',
      label: $t('page.eventSchema.category'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<EventSchema> = {
  height: 'auto',
  stripe: true,
  autoResize: true,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
  pagerConfig: {},
  rowConfig: { isHover: true, resizable: true },
  tooltipConfig: { showAll: true, enterable: true },
  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        return await fetchListEventSchemas(
          new PaginationQuery({
            paging: { page: page.currentPage, pageSize: page.pageSize },
            formValues,
          }),
        );
      },
    },
  },
  columns: [
    {
      title: 'ID',
      field: 'id',
      minWidth: 80,
      fixed: 'left',
    },
    {
      title: $t('page.analytics.eventName'),
      field: 'eventName',
      minWidth: 180,
      fixed: 'left',
    },
    {
      title: $t('page.eventSchema.displayName'),
      field: 'displayName',
      minWidth: 140,
    },
    {
      title: $t('page.eventSchema.category'),
      field: 'category',
      minWidth: 120,
    },
    {
      title: $t('page.analytics.schemaProperty'),
      field: 'properties',
      minWidth: 100,
      slots: { default: 'propertyCount' },
    },
    {
      title: $t('page.eventSchema.status'),
      field: 'status',
      minWidth: 100,
      slots: { default: 'status' },
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      minWidth: 140,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
};

const gridEvents: VxeGridListeners<EventSchema> = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

const [Drawer, drawerApi] = useVbenDrawer({
  connectedComponent: EventSchemaDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.reload();
    }
  },
});

function openDrawer(create: boolean, row?: EventSchema) {
  drawerApi.setData({ create, row });
  drawerApi.open();
}

function handleCreate() {
  openDrawer(true);
}

function handleEdit(row: EventSchema) {
  openDrawer(false, row);
}

async function handleDelete(row: EventSchema) {
  try {
    await deleteEventSchema({ id: row.id! });
    notification.success({ message: $t('ui.notification.delete_success') });
    await gridApi.reload();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.developer.eventSchema')">
      <template #toolbar-tools>
        <a-button type="primary" @click="handleCreate">
          {{ $t('ui.button.create') }}
        </a-button>
      </template>
      <template #propertyCount="{ row }">
        <a-tag>{{ (row.properties ?? []).length }}</a-tag>
      </template>
      <template #status="{ row }">
        <a-tag :color="eventSchemaStatusToColor(row.status)">
          {{ eventSchemaStatusToName(row.status) }}
        </a-tag>
      </template>
      <template #action="{ row }">
        <a-button type="link" size="small" @click="handleEdit(row)">
          {{ $t('ui.button.edit') }}
        </a-button>
        <a-popconfirm
          :title="$t('ui.notification.delete_confirm')"
          @confirm="handleDelete(row)"
        >
          <a-button type="link" danger size="small">
            {{ $t('ui.button.delete') }}
          </a-button>
        </a-popconfirm>
      </template>
    </Grid>
    <Drawer />
  </Page>
</template>
