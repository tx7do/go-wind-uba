<script lang="ts" setup>
import type { ubaservicev1_BehaviorEvent } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import dayjs from 'dayjs';

import { fetchListBehaviorEvents, PaginationQuery } from '#/api';
import { $t } from '#/locales';

const loading = ref(false);
const events = ref<ubaservicev1_BehaviorEvent[]>([]);
const total = ref(0);

const form = ref({
  user_id: '',
  session_id: '',
  global_user_id: '',
  event_name: '',
});

async function search() {
  const hasQuery = Object.values(form.value).some((v) => v?.trim());
  if (!hasQuery) return;

  loading.value = true;
  try {
    const cleaned = Object.fromEntries(
      Object.entries(form.value).filter(([, v]) => v?.trim()),
    );
    const resp = await fetchListBehaviorEvents(
      new PaginationQuery({
        paging: { page: 1, pageSize: 200 },
        formValues: cleaned,
        orderBy: ['-event_ts'],
      }),
    );
    events.value = resp.items ?? [];
    total.value = Number(resp.total ?? 0);
  } catch {
    events.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function formatTime(ts?: number | string) {
  if (!ts) return '';
  return dayjs(typeof ts === 'number' ? ts : Number(ts)).format(
    'YYYY-MM-DD HH:mm:ss',
  );
}

function formatProps(props?: Record<string, string>) {
  if (!props || Object.keys(props).length === 0) return '';
  return Object.entries(props)
    .map(([k, v]) => `${k}=${v}`)
    .join('；');
}
</script>

<template>
  <Page auto-content-height>
    <h3 class="mb-4 text-lg font-semibold">
      {{ $t('page.analytics.behaviorTimeline') }}
    </h3>

    <!-- 搜索栏 -->
    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-end gap-3">
        <div class="flex flex-col gap-1">
          <span class="text-muted-foreground text-sm">{{
            $t('page.session.userId')
          }}</span>
          <a-input
            v-model:value="form.user_id"
            allow-clear
            placeholder="user_id"
            class="w-48"
            @press-enter="search"
          />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-muted-foreground text-sm">{{
            $t('page.session.sessionId')
          }}</span>
          <a-input
            v-model:value="form.session_id"
            allow-clear
            placeholder="session_id"
            class="w-48"
            @press-enter="search"
          />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-muted-foreground text-sm">{{
            $t('page.session.globalUserId')
          }}</span>
          <a-input
            v-model:value="form.global_user_id"
            allow-clear
            placeholder="global_user_id"
            class="w-48"
            @press-enter="search"
          />
        </div>
        <div class="flex flex-col gap-1">
          <span class="text-muted-foreground text-sm">{{
            $t('page.analytics.eventName')
          }}</span>
          <a-input
            v-model:value="form.event_name"
            allow-clear
            placeholder="event_name"
            class="w-48"
            @press-enter="search"
          />
        </div>
        <a-button type="primary" :loading="loading" @click="search">
          {{ $t('ui.button.ok') }}
        </a-button>
      </div>
    </div>

    <div
      v-if="total > 0"
      class="text-muted-foreground mb-2 flex items-center gap-2"
    >
      <span>
        {{ $t('page.analytics.totalEvents') }}:
        <span class="text-foreground font-semibold">{{ total }}</span>
      </span>
      <a-tag v-if="total > events.length" color="orange">
        {{
          $t('page.analytics.showingPartial', { shown: events.length, total })
        }}
      </a-tag>
    </div>

    <!-- 时间轴 -->
    <a-spin v-if="loading" class="flex justify-center py-20" />
    <a-timeline v-else-if="events.length > 0" class="px-2">
      <a-timeline-item v-for="(ev, idx) in events" :key="idx">
        <template #dot>
          <a-badge status="processing" />
        </template>
        <div class="border-border rounded-md border p-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <a-tag color="blue">{{ ev.eventName }}</a-tag>
              <span
                v-if="ev.eventCategory"
                class="text-muted-foreground text-xs"
              >
                {{ ev.eventCategory }}
              </span>
              <span v-if="ev.objectName" class="text-sm">
                → {{ ev.objectType }}: {{ ev.objectName }}
              </span>
            </div>
            <span class="text-muted-foreground text-xs">
              {{ formatTime(ev.eventTs) }}
            </span>
          </div>
          <div class="text-muted-foreground mt-2 flex flex-wrap gap-4 text-xs">
            <span v-if="ev.platform">platform: {{ ev.platform }}</span>
            <span v-if="ev.durationMs">duration: {{ ev.durationMs }}ms</span>
            <span v-if="ev.amount">amount: {{ ev.amount }}</span>
            <span v-if="ev.sessionId">session: {{ ev.sessionId }}</span>
          </div>
          <div
            v-if="ev.properties || ev.context"
            class="text-muted-foreground mt-2 text-xs"
          >
            {{ formatProps(ev.properties || ev.context) }}
          </div>
        </div>
      </a-timeline-item>
    </a-timeline>
    <a-empty v-else :description="$t('page.analytics.noData')" class="py-20" />
  </Page>
</template>
