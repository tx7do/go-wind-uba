<script lang="ts" setup>
import type { ubaservicev1_ServerRetentionResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchServerRetention, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(14));
const loading = ref(false);
const serverId = ref('');
const maxOffsetDays = ref(7);

const data = ref<ubaservicev1_ServerRetentionResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchServerRetention({
      timeRange: range.value,
      serverId: serverId.value || undefined,
      maxOffsetDays: maxOffsetDays.value,
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.serverRetention') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.serverId') }}</span>
          <a-input v-model:value="serverId" class="w-40" :placeholder="$t('page.analytics.serverIdOptional')" allow-clear @press-enter="load" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.maxOffsetDays') }}</span>
          <a-input-number v-model:value="maxOffsetDays" class="w-24" :min="1" :max="30" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.rows?.length ?? 0) > 0"
        :data-source="data.rows"
        :pagination="{ pageSize: 20 }"
        row-key="serverId"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.serverId')" data-index="serverId" :width="120" />
        <a-table-column :title="$t('page.analytics.cohortSize')" data-index="cohortSize" :width="120" />
        <a-table-column
          v-for="d in data.offsetDays"
          :key="d"
          :title="`D${d}`"
          :width="90"
        >
          <template #default="{ record }">
            {{ ((record.retentionRates?.[String(d)] ?? 0) * 100).toFixed(1) }}%
          </template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
