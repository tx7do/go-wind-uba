<script lang="ts" setup>
import type { ubaservicev1_AttributionResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchAttribution, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 转化事件（默认示例）
const conversionEvent = ref('pay_success');
// 归因维度：channel / referer
const dimension = ref<'channel' | 'referer'>('channel');
// 归因模型：last_touch / first_touch
const model = ref<'last_touch' | 'first_touch'>('last_touch');

const data = ref<ubaservicev1_AttributionResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const buckets = data.value?.buckets ?? [];
  return {
    series: [
      {
        data: buckets.map((b) => ({
          name: b.label,
          value: Number(b.converterUv ?? 0),
        })),
        itemStyle: { color: '#5ab1ef' },
        label: { position: 'top', show: true },
        type: 'bar',
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: {
      axisLabel: { rotate: 30 },
      data: buckets.map((b) => b.label),
      type: 'category',
    },
    yAxis: { type: 'value' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchAttribution({
      timeRange: range.value,
      conversionEvent: conversionEvent.value,
      dimension: dimension.value,
      model: model.value,
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
        {{ $t('page.analytics.attribution') }}
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
            $t('page.analytics.conversionEvent')
          }}</span>
          <a-input
            v-model:value="conversionEvent"
            class="w-48"
            :placeholder="$t('page.analytics.eventName')"
          />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.dimension')
          }}</span>
          <a-select v-model:value="dimension" class="w-32">
            <a-select-option value="channel">
              {{ $t('page.analytics.channel') }}
            </a-select-option>
            <a-select-option value="referer">
              {{ $t('page.analytics.referer') }}
            </a-select-option>
          </a-select>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.attributionModel')
          }}</span>
          <a-select v-model:value="model" class="w-36">
            <a-select-option value="last_touch">
              {{ $t('page.analytics.lastTouch') }}
            </a-select-option>
            <a-select-option value="first_touch">
              {{ $t('page.analytics.firstTouch') }}
            </a-select-option>
          </a-select>
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 概览 -->
    <div v-if="data" class="mb-4 grid grid-cols-2 gap-4">
      <a-statistic
        :title="$t('page.analytics.totalConverters')"
        :value="Number(data.totalConverters ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.touchpointCount')"
        :value="Number(data.buckets?.length ?? 0)"
      />
    </div>

    <!-- 归因柱状图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="360px" />

      <!-- 明细表 -->
      <a-table
        v-if="data"
        :data-source="data.buckets"
        :pagination="false"
        row-key="label"
        size="small"
        class="mt-3"
      >
        <a-table-column
          :title="$t('page.analytics.dimension')"
          data-index="label"
        />
        <a-table-column
          :title="$t('page.analytics.converterUv')"
          data-index="converterUv"
          :width="140"
        />
        <a-table-column :title="$t('page.analytics.percentage')" :width="140">
          <template #default="{ record }">
            {{ ((record.percentage ?? 0) * 100).toFixed(2) }}%
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
