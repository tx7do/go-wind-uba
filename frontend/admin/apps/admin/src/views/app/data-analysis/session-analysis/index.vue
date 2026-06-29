<script lang="ts" setup>
import type { ubaservicev1_SessionAnalysisResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchSessionAnalysis, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);
const data = ref<ubaservicev1_SessionAnalysisResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchSessionAnalysis({ timeRange: range.value });
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
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.sessionAnalysis') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background relative mb-4 rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <div v-if="data" class="grid grid-cols-2 gap-4 md:grid-cols-4">
        <a-statistic :title="$t('page.analytics.sessionCount')" :value="Number(data.sessionCount ?? 0)" />
        <a-statistic :title="$t('page.analytics.uniqueUsers')" :value="Number(data.uniqueUsers ?? 0)" />
        <a-statistic :title="$t('page.analytics.avgDurationSec')" :value="Number(data.avgDurationSec ?? 0)" :precision="2" suffix="s" />
        <a-statistic :title="$t('page.analytics.p50DurationSec')" :value="Number(data.p50DurationSec ?? 0)" :precision="2" suffix="s" />
        <a-statistic :title="$t('page.analytics.p90DurationSec')" :value="Number(data.p90DurationSec ?? 0)" :precision="2" suffix="s" />
        <a-statistic :title="$t('page.analytics.bounceRate')" :value="Number(data.bounceRate ?? 0) * 100" :precision="2" suffix="%" />
        <a-statistic :title="$t('page.analytics.avgDepth')" :value="Number(data.avgDepth ?? 0)" :precision="2" />
      </div>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
