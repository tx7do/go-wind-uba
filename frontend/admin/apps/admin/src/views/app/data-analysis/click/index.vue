<script lang="ts" setup>
import type { ubaservicev1_ClickResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchClick, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 页面 URL（必填）
const pageUrl = ref('');
// 网格大小（像素）
const gridSize = ref(20);

const data = ref<ubaservicev1_ClickResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

// Echarts heatmap 需要 [x, y, value] 三元组 + visualMap
const chartOption = computed(() => {
  const points = data.value?.points ?? [];
  const heatData = points.map((p) => [p.x, p.y, Number(p.count ?? 0)]);
  const maxCount = points.reduce(
    (m, p) => Math.max(m, Number(p.count ?? 0)),
    0,
  );
  return {
    tooltip: {
      formatter: (params: any) =>
        `(${params.value[0]}, ${params.value[1]})<br/>${$t(
          'page.analytics.clickCount',
        )}: ${params.value[2]}`,
    },
    visualMap: {
      max: maxCount || 1,
      min: 0,
      calculable: true,
      inRange: { color: ['#50a3ba', '#eac736', '#d94e5d'] },
      orient: 'horizontal',
      left: 'center',
      top: 0,
    },
    grid: { bottom: '8%', containLabel: true, left: '3%', right: '4%', top: '15%' },
    xAxis: {
      max: 'dataMax',
      name: 'X (px)',
      splitArea: { show: true },
      type: 'value',
    },
    yAxis: {
      max: 'dataMax',
      name: 'Y (px)',
      splitArea: { show: true },
      type: 'value',
    },
    series: [
      {
        data: heatData,
        emphasis: {
          itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0, 0, 0, 0.5)' },
        },
        pointSize: Number(gridSize.value),
        type: 'heatmap',
      },
    ],
  } as any;
});

async function load() {
  if (!pageUrl.value) {
    data.value = undefined;
    return;
  }
  loading.value = true;
  try {
    data.value = await fetchClick({
      timeRange: range.value,
      pageUrl: pageUrl.value,
      gridSize: gridSize.value,
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
        {{ $t('page.analytics.clickHeatmap') }}
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
        <div class="flex flex-1 items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.pageUrl')
          }}</span>
          <a-input
            v-model:value="pageUrl"
            class="min-w-[320px] flex-1"
            :placeholder="$t('page.analytics.pageUrlPlaceholder')"
            allow-clear
            @press-enter="load"
          />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.gridSize')
          }}</span>
          <a-input-number
            v-model:value="gridSize"
            class="w-28"
            :min="5"
            :max="100"
          />
          <span class="text-muted-foreground text-sm">px</span>
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 概览 -->
    <div v-if="data" class="mb-4 grid grid-cols-2 gap-4">
      <a-statistic
        :title="$t('page.analytics.totalClicks')"
        :value="Number(data.totalClicks ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.heatmapPoints')"
        :value="Number(data.points?.length ?? 0)"
      />
    </div>

    <!-- 热力图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI v-if="data" ref="chartRef" height="420px" />
      <a-empty v-else :description="$t('page.analytics.inputPageUrlFirst')" />
    </div>

    <!-- 元素点击 TOP -->
    <div
      v-if="data && (data.topElements?.length ?? 0) > 0"
      class="bg-background rounded-lg p-4"
    >
      <h4 class="mb-3 font-semibold">
        {{ $t('page.analytics.topClickedElements') }}
      </h4>
      <a-table
        :data-source="data.topElements"
        :pagination="{ pageSize: 10 }"
        row-key="elementXpath"
        size="small"
      >
        <a-table-column
          :title="$t('page.analytics.elementXpath')"
          data-index="elementXpath"
        />
        <a-table-column
          :title="$t('page.analytics.clickCount')"
          data-index="count"
          :width="120"
        />
        <a-table-column :title="$t('page.analytics.percentage')" :width="120">
          <template #default="{ record }">
            {{ ((record.percentage ?? 0) * 100).toFixed(2) }}%
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
