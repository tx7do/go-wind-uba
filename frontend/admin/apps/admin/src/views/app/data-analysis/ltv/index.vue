<script lang="ts" setup>
import type { ubaservicev1_LTVResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchLTV, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(30));
const loading = ref(false);
const dimension = ref<'none' | 'channel'>('none');

const data = ref<ubaservicev1_LTVResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const points = data.value?.points ?? [];
  // 按维度分组（label），x 轴是 dayN
  const dayAxis = [0, 1, 3, 7, 14, 30, 60, 90];
  const labels = [...new Set(points.map((p) => p.label || $t('page.analytics.overall')))];
  const series = labels.map((label) => {
    const seriesData = dayAxis.map((dn) => {
      const pt = points.find((p) => (p.label || $t('page.analytics.overall')) === label && p.dayN === dn);
      return pt ? Number(pt.ltv ?? 0) : null;
    });
    return {
      name: label,
      type: 'line',
      smooth: true,
      data: seriesData,
      connectNulls: true,
    };
  });
  return {
    legend: { top: 0 },
    series,
    tooltip: { trigger: 'axis' },
    xAxis: { data: dayAxis.map(String), type: 'category', name: $t('page.analytics.dayN') },
    yAxis: { type: 'value', name: $t('page.analytics.ltv') },
    grid: { top: '12%', left: '3%', right: '4%', containLabel: true },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchLTV({
      timeRange: range.value,
      dimension: dimension.value === 'channel' ? 'channel' : undefined,
    });
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.ltv') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.groupBy') }}</span>
          <a-select v-model:value="dimension" class="w-32">
            <a-select-option value="none">{{ $t('page.analytics.overall') }}</a-select-option>
            <a-select-option value="channel">{{ $t('page.analytics.channel') }}</a-select-option>
          </a-select>
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <EchartsUI v-if="data" ref="chartRef" height="400px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
