<script lang="ts" setup>
import type { ubaservicev1_LifecycleResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchLifecycle, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 阈值（可配）
const newUserDays = ref(7);
const churnDays = ref(30);

const data = ref<ubaservicev1_LifecycleResponse>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const colors: Record<string, string> = {
  new_user: '#5ab1ef',
  active: '#019680',
  retained: '#fa8c16',
  reactivated: '#722ed1',
  churned: '#d94e5d',
};

const chartOption = computed(() => {
  const stages = data.value?.stages ?? [];
  return {
    legend: { orient: 'vertical', left: 'left', top: 'middle' },
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['60%', '50%'],
        avoidLabelOverlap: false,
        label: {
          formatter: '{b}: {d}%',
          show: true,
        },
        data: stages.map((s) => ({
          name: s.stageLabel || s.stage,
          value: Number(s.userCount ?? 0),
          itemStyle: { color: colors[s.stage ?? ''] },
        })),
      },
    ],
    tooltip: { trigger: 'item' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchLifecycle({
      timeRange: range.value,
      newUserDays: newUserDays.value,
      churnDays: churnDays.value,
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
        {{ $t('page.analytics.lifecycle') }}
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
            $t('page.analytics.newUserDays')
          }}</span>
          <a-input-number v-model:value="newUserDays" class="w-28" :min="1" />
          {{ $t('page.analytics.days') }}
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.churnDays')
          }}</span>
          <a-input-number v-model:value="churnDays" class="w-28" :min="1" />
          {{ $t('page.analytics.days') }}
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 总用户数 -->
    <div v-if="data" class="mb-4">
      <a-statistic
        :title="$t('page.analytics.totalUsers')"
        :value="Number(data.totalUsers ?? 0)"
      />
    </div>

    <!-- 饼图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI v-if="data" ref="chartRef" height="360px" />
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>

    <!-- 阶段明细表 -->
    <div
      v-if="data && (data.stages?.length ?? 0) > 0"
      class="bg-background rounded-lg p-4"
    >
      <a-table
        :data-source="data.stages"
        :pagination="false"
        row-key="stage"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.stage')" data-index="stageLabel" />
        <a-table-column :title="$t('page.analytics.userCount')" data-index="userCount" />
        <a-table-column :title="$t('page.analytics.percentage')" :width="140">
          <template #default="{ record }">
            {{ ((record.percentage ?? 0) * 100).toFixed(2) }}%
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
