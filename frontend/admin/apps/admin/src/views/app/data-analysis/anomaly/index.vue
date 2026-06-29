<script lang="ts" setup>
import type { ubaservicev1_AnomalyResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchAnomaly, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

// 取近 14 天，覆盖 7 日基线
const range = ref(lastDaysRange(14));
const loading = ref(false);
const eventName = ref('');

const data = ref<ubaservicev1_AnomalyResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchAnomaly({
      timeRange: range.value,
      eventName: eventName.value || undefined,
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.anomaly') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.eventName') }}</span>
          <a-input v-model:value="eventName" class="w-48" :placeholder="$t('page.analytics.eventNameOptional')" allow-clear @press-enter="load" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div v-if="data" class="mb-4">
      <a-statistic :title="$t('page.analytics.anomalyCount')" :value="Number(data.anomalyCount ?? 0)" />
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.points?.length ?? 0) > 0"
        :data-source="data.points"
        :pagination="{ pageSize: 20 }"
        row-key="statDate"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.eventName')" data-index="eventName" />
        <a-table-column :title="$t('page.analytics.statDate')" :width="140">
          <template #default="{ record }">
            {{ record.statDate ? new Date(Number(record.statDate)).toISOString().slice(0, 10) : '-' }}
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.pv')" data-index="pv" :width="100" />
        <a-table-column :title="$t('page.analytics.baseline')" :width="120">
          <template #default="{ record }">{{ Number(record.baseline ?? 0).toFixed(0) }}</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.wowChange')" :width="120">
          <template #default="{ record }">
            <span :style="{ color: Number(record.wowChange ?? 0) >= 0 ? '#019680' : '#d94e5d' }">
              {{ (Number(record.wowChange ?? 0) * 100).toFixed(2) }}%
            </span>
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.isAnomaly')" :width="100">
          <template #default="{ record }">
            <a-tag v-if="record.isAnomaly" color="red">{{ $t('page.analytics.anomaly') }}</a-tag>
            <span v-else class="text-muted-foreground">-</span>
          </template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
