<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type {
  ubaservicev1_EventPath as EventPath,
  ubaservicev1_PathNode,
} from '#/generated/api/admin/service/v1';

import { Page } from '@vben/common-ui';

import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  enableBoolToColor,
  enableBoolToName,
  fetchListEventPaths,
  PaginationQuery,
} from '#/api';
import { $t } from '#/locales';

const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'path_id',
      label: $t('page.eventPath.pathId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.eventPath.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'session_id',
      label: $t('page.eventPath.sessionId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'is_converted',
      label: $t('page.eventPath.isConverted'),
      componentProps: {
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
        options: [
          { label: enableBoolToName('true'), value: 'true' },
          { label: enableBoolToName('false'), value: 'false' },
        ],
      },
    },
  ],
};

const gridOptions: VxeGridProps<EventPath> = {
  height: 'auto',
  stripe: true,
  autoResize: true,
  // 行展开配置：点击行展开路径节点序列
  expandConfig: {
    trigger: 'row',
    accordion: true,
  },
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
        return await fetchListEventPaths(
          new PaginationQuery({
            paging: { page: page.currentPage, pageSize: page.pageSize },
            formValues,
            orderBy: ['-start_time'],
          }),
        );
      },
    },
  },
  columns: [
    {
      type: 'expand',
      width: 50,
      slots: { content: 'expand_content' },
    },
    {
      title: $t('page.eventPath.pathId'),
      field: 'pathId',
      minWidth: 200,
      fixed: 'left',
      align: 'left',
    },
    {
      title: $t('page.eventPath.userId'),
      field: 'userId',
      minWidth: 100,
    },
    {
      title: $t('page.eventPath.stepCount'),
      field: 'stepCount',
      minWidth: 90,
    },
    // 路径摘要：前3事件 → 后3事件，一眼看出流转走向
    {
      title: $t('page.eventPath.flowSummary'),
      field: 'first3Events',
      minWidth: 320,
      slots: { default: 'flowSummary' },
    },
    {
      title: $t('page.eventPath.totalDurationMs'),
      field: 'totalDurationMs',
      minWidth: 140,
    },
    {
      title: $t('page.eventPath.conversionEvent'),
      field: 'conversionEvent',
      minWidth: 140,
    },
    {
      title: $t('page.eventPath.isConverted'),
      field: 'isConverted',
      minWidth: 110,
      slots: { default: 'isConverted' },
    },
    {
      title: $t('page.eventPath.conversionTime'),
      field: 'conversionTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.eventPath.startTime'),
      field: 'startTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<EventPath> = {};

const [Grid] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

// 把前3 + 后3事件拼成流转链路（中间用省略号衔接）
function buildFlowSummary(row: EventPath): string[] {
  const first3 = row.first3Events ?? [];
  const last3 = row.last3Events ?? [];
  if (first3.length === 0 && last3.length === 0) {
    return [row.firstEvent ?? '-', row.lastEvent ?? '-'];
  }
  // 前3 和 后3 有重叠时合并
  const all = [...first3];
  if (last3.length > 0) {
    const overlap = first3.filter((e) => last3.includes(e)).length;
    all.push('...', ...last3.slice(Math.max(0, overlap)));
  }
  return all.filter((v, i, a) => v !== '...' || a[i - 1] !== '...');
}

function formatNodeTime(node: ubaservicev1_PathNode): string {
  const t = node.eventTime as { seconds?: number | string } | undefined;
  if (!t?.seconds) return '';
  return dayjs(Number(t.seconds) * 1000).format('HH:mm:ss');
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('menu.dataAnalysis.eventPath')">
      <!-- 路径摘要：事件流转链路标签 -->
      <template #flowSummary="{ row }">
        <div class="flex flex-wrap items-center gap-1">
          <template v-for="(ev, idx) in buildFlowSummary(row)" :key="idx">
            <a-tag v-if="ev === '...'" color="default">…</a-tag>
            <a-tag v-else color="blue">{{ ev }}</a-tag>
            <span
              v-if="idx < buildFlowSummary(row).length - 1"
              class="text-muted-foreground"
            >
              →
            </span>
          </template>
        </div>
      </template>

      <template #isConverted="{ row }">
        <a-tag :color="enableBoolToColor(row.isConverted)">
          {{ enableBoolToName(row.isConverted) }}
        </a-tag>
      </template>

      <!-- 行展开：完整路径节点序列 -->
      <template #expand_content="{ row }">
        <div class="bg-muted/40 rounded-md p-4">
          <div class="text-muted-foreground mb-3 text-sm">
            {{ $t('page.eventPath.fullPath') }}（{{ row.stepCount ?? 0 }}
            {{ $t('page.eventPath.steps') }}）
          </div>
          <template v-if="(row.nodes ?? []).length > 0">
            <div class="flex flex-wrap items-center gap-2">
              <template v-for="(node, idx) in row.nodes" :key="idx">
                <div
                  class="border-border bg-background flex flex-col rounded-lg border px-3 py-2"
                >
                  <div class="flex items-center gap-2">
                    <span
                      class="bg-primary/10 text-primary inline-flex size-5 items-center justify-center rounded-full text-xs font-semibold"
                    >
                      {{ node.stepIndex ?? idx }}
                    </span>
                    <span class="text-foreground text-sm font-medium">{{
                      node.eventName
                    }}</span>
                  </div>
                  <div
                    class="text-muted-foreground mt-1 flex flex-wrap gap-3 text-xs"
                  >
                    <span v-if="node.objectType">
                      {{ node.objectType }}: {{ node.objectId }}
                    </span>
                    <span>{{ formatNodeTime(node) }}</span>
                  </div>
                </div>
                <span
                  v-if="idx < (row.nodes ?? []).length - 1"
                  class="text-muted-foreground text-lg"
                >
                  →
                </span>
              </template>
            </div>
          </template>
          <div v-else class="text-muted-foreground py-4 text-center text-sm">
            {{ $t('page.eventPath.noNodes') }}
          </div>
        </div>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
