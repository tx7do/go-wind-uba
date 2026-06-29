<script lang="ts" setup>
import type { ubaservicev1_DistributionResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchDistribution, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 目标事件（默认示例）
const eventName = ref('page_view');

const data = ref<ubaservicev1_DistributionResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const buckets = data.value?.buckets ?? [];
  return {
    series: [
      {
        data: buckets.map((b) => ({
          name: b.bucket,
          value: Number(b.count ?? 0),
        })),
        itemStyle: { color: '#019680' },
        label: { position: 'top', show: true },
        type: 'bar',
      },
    ],
    tooltip: {
      formatter: (params: any) => {
        const idx = buckets.findIndex((b) => b.bucket === params.name);
        const pct = idx >= 0 ? (buckets[idx]?.percentage ?? 0) * 100 : 0;
        return `${params.name}<br/>${$t('page.analytics.count')}: ${
          params.value
        }<br/>${$t('page.analytics.percentage')}: ${pct.toFixed(2)}%`;
      },
      trigger: 'axis',
    },
    xAxis: {
      data: buckets.map((b) => b.bucket),
      type: 'category',
    },
    yAxis: { type: 'value' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchDistribution({
      timeRange: range.value,
      eventName: eventName.value,
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
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.distribution') }}
      </h3>
      <AnalyticsToolbar
        :show-granularity="false"
        :end-ms="range.endMs"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <!-- 参数 -->
    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.eventName')
          }}</span>
          <a-input
            v-model:value="eventName"
            class="w-48"
            :placeholder="$t('page.analytics.eventName')"
          />
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 分位数摘要 -->
    <div
      v-if="data?.summary"
      class="mb-4 grid grid-cols-2 gap-4 md:grid-cols-5"
    >
      <a-statistic
        :title="$t('page.analytics.count')"
        :value="Number(data.summary.count ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.avgSec')"
        :precision="2"
        :value="Number(data.summary.avgSec ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.p50Sec')"
        :precision="2"
        :value="Number(data.summary.p50Sec ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.p90Sec')"
        :precision="2"
        :value="Number(data.summary.p90Sec ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.maxSec')"
        :precision="2"
        :value="Number(data.summary.maxSec ?? 0)"
      />
    </div>

    <!-- 分桶柱状图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="360px" />
    </div>
  </Page>
</template>
