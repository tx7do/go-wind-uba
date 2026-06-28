<script lang="ts" setup>
import type { ubaservicev1_RetentionResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import dayjs from 'dayjs';

import { fetchRetention, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(30));
const loading = ref(false);
const data = ref<ubaservicev1_RetentionResponse>();

// 留存配置：最大偏移天数、留存类型
const maxOffsetDays = ref(7);
const offsetOptions = [3, 7, 14, 30];
const retentionType = ref<'ACTIVE' | 'EVENT'>('ACTIVE');
const retentionEventName = ref('');

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

// 热力图数据：x = offsetDays, y = cohortDate, value = rate
const heatmapOption = computed(() => {
  const cohorts = data.value?.cohorts ?? [];
  const offsets = data.value?.offsetDays ?? [];
  const yLabels = cohorts.map((c) => dayjs(c.cohortDate ?? 0).format('MM-DD'));
  const points: [number, number, number][] = [];
  let maxRate = 0;
  cohorts.forEach((c, yi) => {
    (c.cells ?? []).forEach((cell) => {
      const xi = offsets.indexOf(cell.offsetDays ?? 0);
      const rate = Number(cell.rate ?? 0);
      if (rate > maxRate) maxRate = rate;
      if (xi !== -1) points.push([xi, yi, rate]);
    });
  });
  return {
    grid: { bottom: 60, left: '15%', right: '5%', top: '5%' },
    series: [
      {
        data: points,
        emphasis: {
          itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,.3)' },
        },
        label: {
          formatter: (p: any) => `${(p.value[2] * 100).toFixed(0)}%`,
          show: true,
        },
        type: 'heatmap',
      },
    ],
    tooltip: {
      formatter: (p: any) =>
        `${yLabels[p.value[1]]} +${offsets[p.value[0]]}d: ${(p.value[2] * 100).toFixed(1)}%`,
      position: 'top',
    },
    visualMap: {
      calculable: true,
      inRange: { color: ['#e0eaff', '#5ab1ef', '#165DFF'] },
      max: maxRate || 1,
      min: 0,
      orient: 'horizontal',
      right: 0,
      top: 0,
    },
    xAxis: {
      data: offsets.map((o) => `+${o}d`),
      splitArea: { show: true },
      type: 'category',
    },
    yAxis: {
      data: yLabels,
      splitArea: { show: true },
      type: 'category',
    },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchRetention({
      timeRange: range.value,
      maxOffsetDays: maxOffsetDays.value,
      retentionType: retentionType.value,
      eventName:
        retentionType.value === 'EVENT' ? retentionEventName.value : undefined,
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

void load();

// option 变化时触发渲染
watch(heatmapOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.retention') }}
      </h3>
      <AnalyticsToolbar
        :show-granularity="false"
        :end-ms="range.endMs"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <!-- 留存配置 -->
    <div class="mb-4 flex flex-wrap items-center gap-4">
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground text-sm">{{
          $t('page.analytics.offsetDays')
        }}</span>
        <a-select v-model:value="maxOffsetDays" class="w-28" @change="load">
          <a-select-option v-for="d in offsetOptions" :key="d" :value="d">
            {{ d }} {{ $t('page.analytics.retentionUnit') }}
          </a-select-option>
        </a-select>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground text-sm">{{
          $t('page.analytics.retentionTypeLabel')
        }}</span>
        <a-select v-model:value="retentionType" class="w-28" @change="load">
          <a-select-option value="ACTIVE">
            {{ $t('page.analytics.retentionActive') }}
          </a-select-option>
          <a-select-option value="EVENT">
            {{ $t('page.analytics.retentionByEvent') }}
          </a-select-option>
        </a-select>
      </div>
      <div v-if="retentionType === 'EVENT'" class="flex items-center gap-2">
        <span class="text-muted-foreground text-sm">{{
          $t('page.analytics.eventName')
        }}</span>
        <a-input
          v-model:value="retentionEventName"
          allow-clear
          class="w-40"
          :placeholder="$t('ui.placeholder.input')"
          @press-enter="load"
        />
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="500px" />
    </div>
  </Page>
</template>
