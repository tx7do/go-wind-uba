<script lang="ts" setup>
import type { ubaservicev1_PathSankeyResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchPathSankey, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);
const topN = ref(20);

const data = ref<ubaservicev1_PathSankeyResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

// 热门路径频次条形图（横向）
const chartOption = computed(() => {
  const paths = (data.value?.paths ?? []).slice(0, 15);
  return {
    series: [
      {
        type: 'bar',
        data: paths.map((p) => Number(p.supportCount ?? 0)),
        itemStyle: { color: '#5ab1ef' },
        label: { position: 'right', show: true },
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: paths.map((p) => p.eventSequence ?? ''),
      inverse: true,
      axisLabel: { width: 220, overflow: 'truncate' },
    },
    grid: { left: '30%', right: '8%', containLabel: false },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchPathSankey({ timeRange: range.value, topN: topN.value });
  } catch {
    data.value = undefined;
  } finally {
    loading.value = false;
  }
}

function onToolbarChange(payload: { endMs: number; startMs: number }) {
  range.value = { endMs: payload.endMs, startMs: payload.startMs };
  load();
}

watch(chartOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.pathSankey') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.topN') }}</span>
          <a-input-number v-model:value="topN" class="w-28" :min="1" :max="200" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <!-- 频次条形图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <EchartsUI v-if="data && (data.paths?.length ?? 0) > 0" ref="chartRef" height="480px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>

    <!-- 路径明细表 -->
    <div v-if="data && (data.paths?.length ?? 0) > 0" class="bg-background rounded-lg p-4">
      <a-table :data-source="data.paths" :pagination="{ pageSize: 20 }" row-key="eventSequence" size="small">
        <a-table-column :title="$t('page.analytics.eventSequence')" data-index="eventSequence" />
        <a-table-column :title="$t('page.analytics.supportCount')" data-index="supportCount" :width="120" />
        <a-table-column :title="$t('page.analytics.uniqueUsers')" data-index="uniqueUsers" :width="120" />
        <a-table-column :title="$t('page.analytics.conversionRate')" :width="120">
          <template #default="{ record }">{{ ((record.conversionRate ?? 0) * 100).toFixed(2) }}%</template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
