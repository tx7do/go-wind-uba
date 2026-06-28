<script lang="ts" setup>
import type { AnalyticsDimension, AnalyticsMetric } from '#/api';
import type { ubaservicev1_GroupByResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchGroupBy, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const dimension = ref<AnalyticsDimension>('platform');
const metric = ref<AnalyticsMetric>('COUNT');
const loading = ref(false);
const data = ref<ubaservicev1_GroupByResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const buckets = data.value?.buckets ?? [];
  return {
    grid: {
      bottom: 40,
      containLabel: true,
      left: '3%',
      right: '4%',
      top: '5%',
    },
    series: [
      {
        barMaxWidth: 50,
        data: buckets.map((b) => ({
          name: b.label || 'unknown',
          value: Number(b.value ?? 0),
        })),
        itemStyle: { borderRadius: [4, 4, 0, 0], color: '#5ab1ef' },
        label: { position: 'top', show: true },
        type: 'bar',
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: {
      data: buckets.map((b) => b.label || 'unknown'),
      type: 'category',
    },
    yAxis: { type: 'value' },
  } as any;
});

const dimensionOptions: { label: string; value: AnalyticsDimension }[] = [
  { label: $t('page.analytics.dimPlatform'), value: 'platform' },
  { label: $t('page.analytics.accessSource'), value: 'channel' },
  { label: $t('page.analytics.dimCountry'), value: 'country' },
  { label: $t('page.analytics.dimAppVersion'), value: 'app_version' },
  { label: $t('page.analytics.eventName'), value: 'event_name' },
  { label: $t('page.analytics.dimOs'), value: 'os' },
];

const metricOptions: { label: string; value: AnalyticsMetric }[] = [
  { label: $t('page.analytics.count'), value: 'COUNT' },
  { label: $t('page.analytics.activeUsers'), value: 'UNIQUE_USER' },
  { label: $t('page.analytics.metricSumAmount'), value: 'SUM_AMOUNT' },
];

async function load() {
  loading.value = true;
  try {
    data.value = await fetchGroupBy({
      timeRange: range.value,
      dimension: dimension.value,
      metric: metric.value,
      topN: 15,
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

watch([dimension, metric], load);
void load();

// option 变化时触发渲染
watch(chartOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.dimensionCompare') }}
      </h3>
      <AnalyticsToolbar
        :show-granularity="false"
        :end-ms="range.endMs"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-4">
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground">{{
          $t('page.analytics.dimension')
        }}</span>
        <a-select v-model:value="dimension" class="w-40">
          <a-select-option
            v-for="opt in dimensionOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </a-select-option>
        </a-select>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground">{{
          $t('page.analytics.compareMetric')
        }}</span>
        <a-select v-model:value="metric" class="w-40">
          <a-select-option
            v-for="opt in metricOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </a-select-option>
        </a-select>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="420px" />
    </div>
  </Page>
</template>
