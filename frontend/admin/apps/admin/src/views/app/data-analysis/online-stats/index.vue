<script lang="ts" setup>
import type { ubaservicev1_OnlineStatsResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchOnlineStats, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(1));
const loading = ref(false);
const serverId = ref('');

const data = ref<ubaservicev1_OnlineStatsResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchOnlineStats({
      timeRange: range.value,
      serverId: serverId.value || undefined,
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
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.onlineStats') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.serverId') }}</span>
          <a-input v-model:value="serverId" class="w-40" :placeholder="$t('page.analytics.serverIdOptional')" allow-clear @press-enter="load" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <div v-if="data" class="grid grid-cols-2 gap-4 md:grid-cols-4">
        <a-statistic :title="$t('page.analytics.pcu')" :value="Number(data.pcu ?? 0)" />
        <a-statistic :title="$t('page.analytics.acu')" :value="Number(data.acu ?? 0)" />
        <a-statistic :title="$t('page.analytics.totalSessions')" :value="Number(data.totalSessions ?? 0)" />
        <a-statistic :title="$t('page.analytics.durationMinutes')" :value="Number(data.durationMinutes ?? 0)" suffix="min" />
      </div>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>

    <div class="mt-4 rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
      {{ $t('page.analytics.pcuNote') }}
    </div>
  </Page>
</template>
