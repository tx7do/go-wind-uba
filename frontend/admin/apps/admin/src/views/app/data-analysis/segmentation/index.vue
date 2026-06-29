<script lang="ts" setup>
import type {
  ubaservicev1_SegmentCondition,
  ubaservicev1_SegmentationResponse,
} from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchSegmentation, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

// 做过的事件（include[0]）
const includeEvent = ref('view_home');
const includeMinTimes = ref(1);
// 未做过的事件（exclude[0]，可空）
const excludeEvent = ref('');

const data = ref<ubaservicev1_SegmentationResponse>();

async function load() {
  loading.value = true;
  try {
    const include: ubaservicev1_SegmentCondition[] = [
      { eventName: includeEvent.value, minTimes: includeMinTimes.value },
    ];
    const exclude: ubaservicev1_SegmentCondition[] = excludeEvent.value
      ? [{ eventName: excludeEvent.value }]
      : [];

    data.value = await fetchSegmentation({
      timeRange: range.value,
      include,
      exclude,
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

// 用户表格数据源
function tableRows() {
  const ids = data.value?.userIds ?? [];
  return ids.map((id, index) => ({ index: index + 1, userId: id }));
}
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.segmentation') }}
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
            $t('page.analytics.didEvent')
          }}</span>
          <a-input
            v-model:value="includeEvent"
            class="w-44"
            :placeholder="$t('page.analytics.eventName')"
          />
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.atLeast')
          }}</span>
          <a-input-number
            v-model:value="includeMinTimes"
            class="w-24"
            :min="1"
          />
          {{ $t('page.analytics.times') }}
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.didNotEvent')
          }}</span>
          <a-input
            v-model:value="excludeEvent"
            class="w-44"
            :placeholder="$t('page.analytics.eventNameOptional')"
            allow-clear
          />
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 命中数 -->
    <div v-if="data" class="mb-4">
      <a-statistic
        :title="$t('page.analytics.matchedUsers')"
        :value="Number(data.total ?? 0)"
      />
    </div>

    <!-- 用户列表 -->
    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <a-table
        v-if="data && (data.userIds?.length ?? 0) > 0"
        :data-source="tableRows()"
        :pagination="{ pageSize: 20 }"
        row-key="userId"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.index')" :width="80">
          <template #default="{ record }">
            {{ record.index }}
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.userId')" :width="160">
          <template #default="{ record }">
            {{ record.userId }}
          </template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
