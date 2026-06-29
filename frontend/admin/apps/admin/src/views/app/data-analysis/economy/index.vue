<script lang="ts" setup>
import type { ubaservicev1_EconomyResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchEconomy, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);
const currency = ref('');

const data = ref<ubaservicev1_EconomyResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchEconomy({
      timeRange: range.value,
      currency: currency.value || undefined,
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.economy') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.currencyFilter') }}</span>
          <a-input v-model:value="currency" class="w-40" :placeholder="$t('page.analytics.currencyPlaceholder')" allow-clear @press-enter="load" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.currencies?.length ?? 0) > 0"
        :data-source="data.currencies"
        :pagination="false"
        row-key="currency"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.currencyCol')" data-index="currency" />
        <a-table-column :title="$t('page.analytics.source')" :width="160">
          <template #default="{ record }">{{ Number(record.source ?? 0).toFixed(2) }}</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.sink')" :width="160">
          <template #default="{ record }">{{ Number(record.sink ?? 0).toFixed(2) }}</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.net')" :width="160">
          <template #default="{ record }">
            <span :style="{ color: Number(record.net ?? 0) > 0 ? '#d94e5d' : '#019680', fontWeight: 'bold' }">
              {{ Number(record.net ?? 0).toFixed(2) }}
              {{ Number(record.net ?? 0) > 0 ? $t('page.analytics.inflation') : $t('page.analytics.deflation') }}
            </span>
          </template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
