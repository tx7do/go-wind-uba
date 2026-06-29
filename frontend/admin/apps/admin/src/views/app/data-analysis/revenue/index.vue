<script lang="ts" setup>
import type { ubaservicev1_RevenueResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchRevenue, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

const data = ref<ubaservicev1_RevenueResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const points = data.value?.points ?? [];
  const xData = points.map((p) =>
    new Date(Number(p.timestamp ?? 0)).toISOString().slice(5, 10),
  );
  return {
    legend: { data: [$t('page.analytics.gmv'), $t('page.analytics.arpu'), $t('page.analytics.arppu')], top: 0 },
    series: [
      { name: $t('page.analytics.gmv'), type: 'line', smooth: true, data: points.map((p) => Number(p.gmv ?? 0)), itemStyle: { color: '#5ab1ef' } },
      { name: $t('page.analytics.arpu'), type: 'line', smooth: true, data: points.map((p) => Number(p.arpu ?? 0)), itemStyle: { color: '#019680' } },
      { name: $t('page.analytics.arppu'), type: 'line', smooth: true, data: points.map((p) => Number(p.arppu ?? 0)), itemStyle: { color: '#fa8c16' } },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: { data: xData, type: 'category' },
    yAxis: { type: 'value' },
    grid: { top: '12%', left: '3%', right: '4%', containLabel: true },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchRevenue({ timeRange: range.value, granularity: 'DAY' });
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.revenue') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div v-if="data" class="mb-4 grid grid-cols-2 gap-4 md:grid-cols-4">
      <a-statistic :title="$t('page.analytics.totalGmv')" :value="Number(data.totalGmv ?? 0)" :precision="2" />
      <a-statistic :title="$t('page.analytics.totalPayUsers')" :value="Number(data.totalPayUsers ?? 0)" />
      <a-statistic :title="$t('page.analytics.totalPayOrders')" :value="Number(data.totalPayOrders ?? 0)" />
      <a-statistic :title="$t('page.analytics.avgOrderValue')" :value="Number(data.avgOrderValue ?? 0)" :precision="2" />
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <EchartsUI v-if="data" ref="chartRef" height="380px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
