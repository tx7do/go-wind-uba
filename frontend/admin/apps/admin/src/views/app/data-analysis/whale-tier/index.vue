<script lang="ts" setup>
import type { ubaservicev1_WhaleTierResponse } from '#/generated/api/admin/service/v1';

import { ref } from 'vue';

import { Page } from '@vben/common-ui';

import { fetchWhaleTier } from '#/api';
import { $t } from '@vben/locales';

const loading = ref(false);
const data = ref<ubaservicev1_WhaleTierResponse>();

async function load() {
  loading.value = true;
  try {
    data.value = await fetchWhaleTier({});
  } catch {
    data.value = undefined;
  } finally {
    loading.value = false;
  }
}

void load();
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-semibold">{{ $t('page.analytics.whaleTier') }}</h3>
      <a-button type="primary" :loading="loading" @click="load">{{ $t('ui.button.ok') }}</a-button>
    </div>

    <div v-if="data" class="mb-4 grid grid-cols-2 gap-4">
      <a-statistic :title="$t('page.analytics.totalUsers')" :value="Number(data.totalUsers ?? 0)" />
      <a-statistic :title="$t('page.analytics.totalRevenue')" :value="Number(data.totalRevenue ?? 0)" :precision="2" />
    </div>

    <div class="bg-background relative rounded-lg p-4">
      <a-spin v-if="loading" class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60" />
      <a-table
        v-if="data && (data.segments?.length ?? 0) > 0"
        :data-source="data.segments"
        :pagination="false"
        row-key="tier"
        size="small"
      >
        <a-table-column :title="$t('page.analytics.tier')" :width="120">
          <template #default="{ record }">
            <a-tag :color="record.tier === 'whale' ? 'red' : record.tier === 'dolphin' ? 'orange' : record.tier === 'minnow' ? 'blue' : 'default'">
              {{ record.tierLabel || record.tier }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.userCount')" data-index="userCount" />
        <a-table-column :title="$t('page.analytics.percentage')" :width="100">
          <template #default="{ record }">{{ ((record.percentage ?? 0) * 100).toFixed(2) }}%</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.totalAmount')" :width="140">
          <template #default="{ record }">{{ Number(record.totalAmount ?? 0).toFixed(2) }}</template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.revenueShare')" :width="120">
          <template #default="{ record }">
            <span :style="{ color: Number(record.revenueShare ?? 0) > 0.5 ? '#d94e5d' : 'inherit', fontWeight: Number(record.revenueShare ?? 0) > 0.5 ? 'bold' : 'normal' }">
              {{ ((record.revenueShare ?? 0) * 100).toFixed(2) }}%
            </span>
          </template>
        </a-table-column>
        <a-table-column :title="$t('page.analytics.arppu')" :width="120">
          <template #default="{ record }">{{ Number(record.arppu ?? 0).toFixed(2) }}</template>
        </a-table-column>
      </a-table>
      <a-empty v-else :description="$t('page.analytics.noData')" />
    </div>
  </Page>
</template>
