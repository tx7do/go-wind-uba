<script lang="ts" setup>
import type { ubaservicev1_MatrixResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchMatrix, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

const dimension = ref<'event_name' | 'event_category' | 'object_type' | 'platform'>('event_name');

const data = ref<ubaservicev1_MatrixResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

// 象限颜色
const quadrantColors: Record<string, string> = {
  core: '#d94e5d',
  star: '#fa8c16',
  niche: '#722ed1',
  edge: '#8c8c8c',
};

// 按象限分组散点
const chartOption = computed(() => {
  const points = data.value?.points ?? [];
  const groups: Record<string, [number, number, string][]> = {};
  for (const p of points) {
    const q = p.quadrant ?? 'edge';
    if (!groups[q]) groups[q] = [];
    groups[q].push([Number(p.x ?? 0), Number(p.y ?? 0), p.label ?? '']);
  }
  const quadrantLabels: Record<string, string> = {
    core: $t('page.analytics.quadrantCore'),
    star: $t('page.analytics.quadrantStar'),
    niche: $t('page.analytics.quadrantNiche'),
    edge: $t('page.analytics.quadrantEdge'),
  };
  return {
    series: Object.keys(groups).map((q) => ({
      name: quadrantLabels[q] || q,
      type: 'scatter',
      symbolSize: 12,
      data: groups[q],
      itemStyle: { color: quadrantColors[q] },
    })),
    legend: { top: 0 },
    tooltip: {
      formatter: (params: any) =>
        `${params.value[2]}<br/>${$t('page.analytics.users')}: ${params.value[0]}<br/>${$t('page.analytics.frequency')}: ${params.value[1]}`,
    },
    xAxis: {
      name: $t('page.analytics.xAxisUsers'),
      nameLocation: 'middle',
      nameGap: 30,
      type: 'value',
      splitLine: { lineStyle: { type: 'dashed' } },
    },
    yAxis: {
      name: $t('page.analytics.yAxisFrequency'),
      nameLocation: 'middle',
      nameGap: 40,
      type: 'value',
      splitLine: { lineStyle: { type: 'dashed' } },
    },
    // 象限分割线（中位数）
    markLine: undefined,
    series_markLine: undefined,
  } as any;
});

// 单独构造带 markLine 的 option（避免上面的 any 冲突）
const fullOption = computed(() => {
  const base: any = chartOption.value;
  if (!data.value || (data.value.points?.length ?? 0) === 0) return base;
  const xT = Number(data.value?.xThreshold ?? 0);
  const yT = Number(data.value?.yThreshold ?? 0);
  // 给每个 series 加 markLine（Echarts 要求 markLine 在 series 上）
  base.series = base.series.map((s: any, i: number) =>
    i === 0
      ? {
          ...s,
          markLine: {
            silent: true,
            symbol: 'none',
            lineStyle: { color: '#999', type: 'dashed' },
            data: [
              { xAxis: xT, label: { formatter: `${$t('page.analytics.median')}: ${xT}` } },
              { yAxis: yT, label: { formatter: `${$t('page.analytics.median')}: ${yT}` } },
            ],
          },
        }
      : s,
  );
  return base;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchMatrix({
      timeRange: range.value,
      dimension: dimension.value,
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

watch(fullOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.matrix') }}
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
            $t('page.analytics.dimension')
          }}</span>
          <a-select v-model:value="dimension" class="w-40">
            <a-select-option value="event_name">
              {{ $t('page.analytics.eventName') }}
            </a-select-option>
            <a-select-option value="event_category">
              {{ $t('page.analytics.eventCategory') }}
            </a-select-option>
            <a-select-option value="object_type">
              {{ $t('page.analytics.objectType') }}
            </a-select-option>
            <a-select-option value="platform">
              {{ $t('page.analytics.platform') }}
            </a-select-option>
          </a-select>
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 象限说明 -->
    <div class="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
      <div class="bg-background flex items-center gap-2 rounded p-2">
        <span class="h-3 w-3 rounded-full" style="background: #d94e5d" />
        <span class="text-sm">{{ $t('page.analytics.quadrantCore') }}</span>
      </div>
      <div class="bg-background flex items-center gap-2 rounded p-2">
        <span class="h-3 w-3 rounded-full" style="background: #fa8c16" />
        <span class="text-sm">{{ $t('page.analytics.quadrantStar') }}</span>
      </div>
      <div class="bg-background flex items-center gap-2 rounded p-2">
        <span class="h-3 w-3 rounded-full" style="background: #722ed1" />
        <span class="text-sm">{{ $t('page.analytics.quadrantNiche') }}</span>
      </div>
      <div class="bg-background flex items-center gap-2 rounded p-2">
        <span class="h-3 w-3 rounded-full" style="background: #8c8c8c" />
        <span class="text-sm">{{ $t('page.analytics.quadrantEdge') }}</span>
      </div>
    </div>

    <!-- 散点图 -->
    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI v-if="data" ref="chartRef" height="480px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
