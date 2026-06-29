<script lang="ts" setup>
import type { ubaservicev1_IntervalResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchInterval, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 起始/结束事件（默认示例：注册→首付费）
const eventFrom = ref('register');
const eventTo = ref('pay_success');

const data = ref<ubaservicev1_IntervalResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const buckets = data.value?.buckets ?? [];
  return {
    series: [
      {
        type: 'bar',
        data: buckets.map((b) => ({ name: b.bucket, value: Number(b.count ?? 0) })),
        itemStyle: { color: '#019680' },
        label: { position: 'top', show: true },
      },
    ],
    tooltip: {
      formatter: (params: any) => {
        const idx = buckets.findIndex((b) => b.bucket === params.name);
        const pct = idx >= 0 ? (buckets[idx]?.percentage ?? 0) * 100 : 0;
        return `${params.name}<br/>${$t('page.analytics.count')}: ${params.value}<br/>${$t('page.analytics.percentage')}: ${pct.toFixed(2)}%`;
      },
      trigger: 'axis',
    },
    xAxis: { data: buckets.map((b) => b.bucket), type: 'category' },
    yAxis: { type: 'value' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchInterval({
      timeRange: range.value,
      eventFrom: eventFrom.value,
      eventTo: eventTo.value,
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
        {{ $t('page.analytics.interval') }}
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
            $t('page.analytics.eventFrom')
          }}</span>
          <a-input v-model:value="eventFrom" class="w-40" :placeholder="$t('page.analytics.eventName')" />
        </div>
        <span class="text-muted-foreground">→</span>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.eventTo')
          }}</span>
          <a-input v-model:value="eventTo" class="w-40" :placeholder="$t('page.analytics.eventName')" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 分位数摘要 -->
    <div v-if="data" class="mb-4 grid grid-cols-2 gap-4 md:grid-cols-4">
      <a-statistic :title="$t('page.analytics.count')" :value="Number(data.count ?? 0)" />
      <a-statistic
        :title="$t('page.analytics.avgHours')"
        :precision="2"
        :value="Number(data.avgHours ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.p50Hours')"
        :precision="2"
        :value="Number(data.p50Hours ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.p90Hours')"
        :precision="2"
        :value="Number(data.p90Hours ?? 0)"
      />
    </div>

    <!-- 间隔分布柱状图 -->
    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI v-if="data" ref="chartRef" height="360px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
