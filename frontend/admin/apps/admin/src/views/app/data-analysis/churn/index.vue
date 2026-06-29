<script lang="ts" setup>
import type { ubaservicev1_ChurnResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchChurn, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

const churnDays = ref(30);
const reactivationDays = ref(7);

const data = ref<ubaservicev1_ChurnResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const chartOption = computed(() => {
  const buckets = data.value?.churnBuckets ?? [];
  return {
    series: [
      {
        type: 'bar',
        data: buckets.map((b) => ({ name: b.bucket, value: Number(b.userCount ?? 0) })),
        itemStyle: { color: '#d94e5d' },
        label: { position: 'top', show: true },
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: { data: buckets.map((b) => b.bucket), type: 'category' },
    yAxis: { type: 'value' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchChurn({
      timeRange: range.value,
      churnDays: churnDays.value,
      reactivationDays: reactivationDays.value,
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
        {{ $t('page.analytics.churn') }}
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
            $t('page.analytics.churnDays')
          }}</span>
          <a-input-number v-model:value="churnDays" class="w-28" :min="1" />
          {{ $t('page.analytics.days') }}
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.reactivationDays')
          }}</span>
          <a-input-number v-model:value="reactivationDays" class="w-28" :min="1" />
          {{ $t('page.analytics.days') }}
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 指标卡 -->
    <div v-if="data" class="mb-4 grid grid-cols-3 gap-4">
      <a-statistic
        :title="$t('page.analytics.churnedUsers')"
        :value="Number(data.churnedUsers ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.reactivatedUsers')"
        :value="Number(data.reactivatedUsers ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.reactivationRate')"
        :precision="2"
        :value="Number(data.reactivationRate ?? 0) * 100"
        suffix="%"
      />
    </div>

    <!-- 流失时长分布柱状图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <h4 class="mb-2 font-semibold">
        {{ $t('page.analytics.churnDurationDistribution') }}
      </h4>
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI v-if="data" ref="chartRef" height="320px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>

    <!-- 回流触发事件 TOP -->
    <div
      v-if="data && (data.triggers?.length ?? 0) > 0"
      class="bg-background rounded-lg p-4"
    >
      <h4 class="mb-3 font-semibold">
        {{ $t('page.analytics.reactivationTriggers') }}
      </h4>
      <a-table
        :data-source="data.triggers"
        :pagination="{ pageSize: 10 }"
        row-key="eventName"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.eventName')" data-index="eventName" />
        <a-table-column :title="$t('page.analytics.count')" data-index="count" :width="120" />
        <a-table-column :title="$t('page.analytics.percentage')" :width="120">
          <template #default="{ record }">
            {{ ((record.percentage ?? 0) * 100).toFixed(2) }}%
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
