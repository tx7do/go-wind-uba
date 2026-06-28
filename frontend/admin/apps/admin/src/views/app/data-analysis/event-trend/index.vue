<script lang="ts" setup>
import type { AnalyticsGranularity } from '#/api';
import type { ubaservicev1_EventTrendResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import dayjs from 'dayjs';

import { fetchEventTrend, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const granularity = ref<AnalyticsGranularity>(
  'ANALYTICS_GRANULARITY_UNSPECIFIED',
);
const loading = ref(false);
const data = ref<ubaservicev1_EventTrendResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const trendOption = computed(() => {
  const points = data.value?.points ?? [];
  return {
    grid: {
      bottom: 40,
      containLabel: true,
      left: '3%',
      right: '3%',
      top: '5%',
    },
    series: [
      {
        areaStyle: { opacity: 0.2 },
        data: points.map((p) => Number(p.value ?? 0)),
        itemStyle: { color: '#5ab1ef' },
        name: $t('page.analytics.eventCount'),
        smooth: true,
        type: 'line',
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: {
      boundaryGap: false,
      data: points.map((p) => dayjs(p.timestamp ?? 0).format('MM-DD HH:mm')),
      type: 'category',
    },
    yAxis: { type: 'value' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchEventTrend({
      timeRange: range.value,
      granularity: granularity.value,
    });
  } catch {
    data.value = undefined;
  } finally {
    loading.value = false;
  }
}

function onToolbarChange(payload: {
  endMs: number;
  granularity: AnalyticsGranularity;
  startMs: number;
}) {
  range.value = { endMs: payload.endMs, startMs: payload.startMs };
  granularity.value = payload.granularity;
  load();
}

void load();

// option 变化时（数据加载完成 / 切换粒度）触发渲染
watch(trendOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.eventTrend') }}
      </h3>
      <AnalyticsToolbar
        :end-ms="range.endMs"
        :granularity="granularity"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <div class="text-muted-foreground mb-2">
      {{ $t('page.analytics.totalEvents') }}:
      <span class="text-foreground font-semibold">
        {{ data?.total ?? 0 }}
      </span>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="400px" />
    </div>
  </Page>
</template>
