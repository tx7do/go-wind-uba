<script lang="ts" setup>
import type { ubaservicev1_NewVsOldResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchNewVsOld, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);
const newUserDays = ref(7);

const data = ref<ubaservicev1_NewVsOldResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchNewVsOld({
      timeRange: range.value,
      newUserDays: newUserDays.value,
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.newVsOld') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.newUserDays') }}</span>
          <a-input-number v-model:value="newUserDays" class="w-28" :min="1" />
          {{ $t('page.analytics.days') }}
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.segments?.length ?? 0) > 0"
        :data-source="data.segments"
        :pagination="false"
        row-key="userType"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.userType')" :width="120">
          <template #default="{ record }">
            <a-tag :color="record.userType === 'new' ? 'green' : 'blue'">
              {{ record.userType === 'new' ? $t('page.analytics.newUser') : $t('page.analytics.oldUser') }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.userCount')" data-index="userCount" />
        <a-table-column :title="$t('page.analytics.eventCount')" data-index="eventCount" />
        <a-table-column :title="$t('page.analytics.payUsers')" data-index="payUsers" />
        <a-table-column :title="$t('page.analytics.payRate')" :width="120">
          <template #default="{ record }">{{ ((record.payRate ?? 0) * 100).toFixed(2) }}%</template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
