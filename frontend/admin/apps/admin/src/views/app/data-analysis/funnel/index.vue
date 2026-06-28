<script lang="ts" setup>
import type { ubaservicev1_FunnelResponse } from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchFunnel, lastDaysRange } from '#/api';
import { $t } from '#/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 漏斗步骤（默认示例，可增删）
const steps = ref<string[]>([
  'app_launch',
  'view_home',
  'add_to_cart',
  'submit_order',
  'pay_success',
]);

const data = ref<ubaservicev1_FunnelResponse>();

// 转化窗口（毫秒），默认 30 分钟
const windowMs = ref(30 * 60 * 1000);
const windowOptions = [
  { label: '15分钟', value: 15 * 60 * 1000 },
  { label: '30分钟', value: 30 * 60 * 1000 },
  { label: '1小时', value: 60 * 60 * 1000 },
  { label: '1天', value: 24 * 60 * 60 * 1000 },
];

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const funnelOption = computed(() => {
  const stepsData = data.value?.steps ?? [];
  return {
    series: [
      {
        data: stepsData.map((s) => ({
          name: `${s.stepIndex}. ${s.eventName}`,
          value: s.count,
        })),
        gap: 2,
        label: {
          formatter: '{b} {c}',
          position: 'inside',
          show: true,
        },
        sort: 'descending',
        type: 'funnel',
        width: '70%',
      },
    ],
    tooltip: { formatter: '{b}: {c}', trigger: 'item' },
  } as any;
});

async function load() {
  loading.value = true;
  try {
    data.value = await fetchFunnel({
      timeRange: range.value,
      steps: steps.value,
      windowMs: windowMs.value,
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

function addStep() {
  steps.value.push('new_event');
}

function removeStep(index: number) {
  if (steps.value.length > 2) {
    steps.value.splice(index, 1);
  }
}

// option 变化时触发渲染
watch(funnelOption, (opt) => renderEcharts(opt as any));
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.funnel') }}
      </h3>
      <AnalyticsToolbar
        :show-granularity="false"
        :end-ms="range.endMs"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <!-- 步骤编辑器 -->
    <div class="bg-background mb-4 rounded-lg p-4">
      <div
        v-for="(_step, index) in steps"
        :key="index"
        class="mb-2 flex items-center gap-2"
      >
        <span class="text-muted-foreground w-8">{{ index + 1 }}</span>
        <a-input
          v-model:value="steps[index]"
          :placeholder="$t('page.analytics.eventName')"
          class="flex-1"
        />
        <a-button
          v-if="steps.length > 2"
          danger
          size="small"
          @click="removeStep(index)"
        >
          ×
        </a-button>
      </div>
      <a-button block type="dashed" @click="addStep">
        + {{ $t('page.analytics.addStep') }}
      </a-button>
      <div class="mt-3 flex items-center justify-center gap-2">
        <span class="text-muted-foreground text-sm">{{
          $t('page.analytics.conversionWindow')
        }}</span>
        <a-select v-model:value="windowMs" class="w-32" @change="load">
          <a-select-option
            v-for="opt in windowOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </a-select-option>
        </a-select>
      </div>
      <div class="mt-3 text-center">
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 概览 -->
    <div v-if="data" class="mb-4 grid grid-cols-3 gap-4">
      <a-statistic
        :title="$t('page.analytics.enteredUsers')"
        :value="Number(data.enteredUsers ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.completedUsers')"
        :value="Number(data.completedUsers ?? 0)"
      />
      <a-statistic
        :title="$t('page.analytics.overallConversion')"
        :precision="2"
        :value="Number(data.overallConversion ?? 0) * 100"
        suffix="%"
      />
    </div>

    <!-- 漏斗图 -->
    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <EchartsUI ref="chartRef" height="360px" />

      <!-- 步骤明细表 -->
      <a-table
        v-if="data"
        :data-source="data.steps"
        :pagination="false"
        row-key="stepIndex"
        size="small"
      >
        <a-table-column
          :title="$t('page.analytics.funnelStep')"
          data-index="stepIndex"
          :width="80"
        />
        <a-table-column
          :title="$t('page.analytics.eventName')"
          data-index="eventName"
        />
        <a-table-column
          :title="$t('page.analytics.count')"
          data-index="count"
          :width="120"
        />
        <a-table-column
          :title="$t('page.analytics.conversionRate')"
          :width="140"
        >
          <template #default="{ record }">
            {{ ((record.conversionRate ?? 0) * 100).toFixed(2) }}%
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Page>
</template>
