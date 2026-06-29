<script lang="ts" setup>
import type { ubaservicev1_LevelAnalysisResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchLevelAnalysis, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);
const levelId = ref('');

const data = ref<ubaservicev1_LevelAnalysisResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchLevelAnalysis({
      timeRange: range.value,
      levelId: levelId.value || undefined,
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
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.levelAnalysis') }}</h3>
      <AnalyticsToolbar :show-granularity="false" :end-ms="range.endMs" :start-ms="range.startMs" @change="onToolbarChange" />
    </div>

    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{ $t('page.analytics.levelId') }}</span>
          <a-input v-model:value="levelId" class="w-48" :placeholder="$t('page.analytics.levelIdOptional')" allow-clear @press-enter="load" />
        </div>
        <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
      </div>
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.levels?.length ?? 0) > 0"
        :data-source="data.levels"
        :pagination="{ pageSize: 20 }"
        row-key="levelId"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.levelId')" data-index="levelId" :width="100" />
        <a-table-column :title="$t('page.analytics.levelName')" data-index="levelName" />
        <a-table-column :title="$t('page.analytics.attemptCount')" data-index="attemptCount" :width="100" />
        <a-table-column :title="$t('page.analytics.playerCount')" data-index="playerCount" :width="100" />
        <a-table-column :title="$t('page.analytics.passRate')" :width="110">
          <template #default="{ record }">{{ ((record.passRate ?? 0) * 100).toFixed(1) }}%</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.stuckRate')" :width="110">
          <template #default="{ record }">
            <span :style="{ color: Number(record.stuckRate ?? 0) > 0.5 ? '#d94e5d' : 'inherit', fontWeight: Number(record.stuckRate ?? 0) > 0.5 ? 'bold' : 'normal' }">
              {{ ((record.stuckRate ?? 0) * 100).toFixed(1) }}%
            </span>
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.avgScore')" :width="100">
          <template #default="{ record }">{{ Number(record.avgScore ?? 0).toFixed(1) }}</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.star3Rate')" :width="100">
          <template #default="{ record }">{{ ((record.star3Rate ?? 0) * 100).toFixed(1) }}%</template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
