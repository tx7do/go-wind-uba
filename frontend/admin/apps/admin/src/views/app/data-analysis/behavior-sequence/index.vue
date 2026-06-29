<script lang="ts" setup>
import type { ubaservicev1_BehaviorSequenceResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchBehaviorSequence, lastDaysRange } from '#/api';
import { $t } from '@vben/locales';

import dayjs from 'dayjs';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

const range = ref(lastDaysRange(7));
const loading = ref(false);

const userId = ref<number>(1);
const eventName = ref('');

const data = ref<ubaservicev1_BehaviorSequenceResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchBehaviorSequence({
      timeRange: range.value,
      userId: userId.value,
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

// 表格行需要稳定 key
function rowKey(record: any, index?: number) {
  return `${record?.timestamp ?? ''}-${index}`;
}
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.behaviorSequence') }}
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
            $t('page.analytics.userId')
          }}</span>
          <a-input-number v-model:value="userId" class="w-40" :min="1" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.eventName')
          }}</span>
          <a-input
            v-model:value="eventName"
            class="w-48"
            :placeholder="$t('page.analytics.eventNameOptional')"
            allow-clear
          />
        </div>
        <a-button type="primary" :loading="loading" @click="load">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <!-- 序列表（按时间升序） -->
    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <a-table
        v-if="data"
        :data-source="data.events"
        :pagination="{ pageSize: 50 }"
        :row-key="rowKey"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.eventTime')" :width="180">
          <template #default="{ record }">
            {{
              record.timestamp
                ? dayjs(record.timestamp).format('YYYY-MM-DD HH:mm:ss')
                : '-'
            }}
          </template>
        </a-table-column>
        <a-table-column
          :title="$t('page.analytics.eventName')"
          data-index="eventName"
        />
        <a-table-column
          :title="$t('page.analytics.sessionId')"
          data-index="sessionId"
          :width="160"
        />
        <a-table-column
          :title="$t('page.analytics.referer')"
          data-index="referer"
        />
        <a-table-column
          :title="$t('page.analytics.platform')"
          data-index="platform"
          :width="100"
        />
        <a-table-column
          :title="$t('page.analytics.channel')"
          data-index="channel"
          :width="100"
        />
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
