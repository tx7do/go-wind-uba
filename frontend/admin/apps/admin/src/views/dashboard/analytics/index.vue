<script lang="ts" setup>
import type { AnalysisOverviewItem } from '@vben/common-ui';
import type { TabOption } from '@vben/types';

import { computed, markRaw, ref } from 'vue';

import {
  AnalysisChartCard,
  AnalysisChartsTabs,
  AnalysisOverview,
} from '@vben/common-ui';
import {
  SvgBellIcon,
  SvgCakeIcon,
  SvgCardIcon,
  SvgDownloadIcon,
} from '@vben/icons';
import { $t } from '@vben/locales';

import {
  fetchActiveUsers,
  fetchEventTrend,
  fetchGroupBy,
  lastDaysRange,
} from '#/api';
import {
  type ubaservicev1_ActiveUsersResponse,
  type ubaservicev1_EventTrendResponse,
  type ubaservicev1_GroupByResponse,
} from '#/generated/api/admin/service/v1';

import AnalyticsChannelPie from './analytics-channel-pie.vue';
import AnalyticsPlatformPie from './analytics-platform-pie.vue';
import AnalyticsTrendBar from './analytics-trend-bar.vue';
import AnalyticsTrends from './analytics-trends.vue';
import AnalyticsVisits from './analytics-visits.vue';

// 最近 7 天范围（毫秒）
const range = lastDaysRange(7);

// KPI 概览：真实数据，部分接口失败时降级为 0
const overviewItems = ref<AnalysisOverviewItem[]>([
  {
    icon: markRaw(SvgCakeIcon),
    title: $t('page.analytics.latestDau'),
    totalTitle: $t('page.analytics.activeUsers'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgCardIcon),
    title: $t('page.analytics.eventCount'),
    totalTitle: $t('page.analytics.totalEvents'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgDownloadIcon),
    title: $t('page.analytics.activeUsers'),
    totalTitle: $t('page.analytics.platformCount'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgBellIcon),
    title: $t('page.analytics.eventCount'),
    totalTitle: $t('page.analytics.channelCount'),
    totalValue: 0,
    value: 0,
  },
]);

// 趋势 / 维度数据（透传给子组件）
const trendResp = ref<ubaservicev1_EventTrendResponse>();
const platformResp = ref<ubaservicev1_GroupByResponse>();
const channelResp = ref<ubaservicev1_GroupByResponse>();
const activeResp = ref<ubaservicev1_ActiveUsersResponse>();

const chartTabs: TabOption[] = [
  { label: $t('page.analytics.trafficTrend'), value: 'trends' },
  { label: $t('page.analytics.activeUsers'), value: 'active' },
];

const trendToday = computed(() => {
  // 今日事件量 = 趋势最后一个桶
  const points = trendResp.value?.points ?? [];
  if (points.length === 0) return 0;
  return Number(points[points.length - 1]?.value ?? 0);
});

const trendTotal = computed(() => Number(trendResp.value?.total ?? 0));

async function loadDashboard() {
  const tasks: Promise<unknown>[] = [
    // 活跃用户（DAU）
    fetchActiveUsers({ timeRange: range, granularity: 'DAY' })
      .then((resp) => {
        activeResp.value = resp;
        overviewItems.value[0]!.value = Number(resp.latestDau ?? 0);
        overviewItems.value[0]!.totalValue = Number(
          resp.points?.at(-1)?.mau ?? 0,
        );
      })
      .catch((error) =>
        console.error('[dashboard] ActiveUsers failed:', error),
      ),

    // 事件趋势
    fetchEventTrend({
      timeRange: range,
      granularity: 'ANALYTICS_GRANULARITY_UNSPECIFIED',
    })
      .then((resp) => {
        trendResp.value = resp;
        overviewItems.value[1]!.value = trendToday.value;
        overviewItems.value[1]!.totalValue = trendTotal.value;
      })
      .catch((error) => console.error('[dashboard] EventTrend failed:', error)),

    // 平台分布（用于会话/访问相关 KPI）
    fetchGroupBy({
      timeRange: range,
      dimension: 'platform',
      metric: 'UNIQUE_USER',
      topN: 1,
    })
      .then((resp) => {
        platformResp.value = resp;
        overviewItems.value[2]!.value = Math.round(Number(resp.total ?? 0));
        overviewItems.value[2]!.totalValue = Number(resp.buckets?.length ?? 0);
      })
      .catch((error) =>
        console.error('[dashboard] GroupBy(platform) failed:', error),
      ),

    // 渠道分布
    fetchGroupBy({
      timeRange: range,
      dimension: 'channel',
      metric: 'COUNT',
      topN: 1,
    })
      .then((resp) => {
        channelResp.value = resp;
        overviewItems.value[3]!.value = Math.round(Number(resp.total ?? 0));
        overviewItems.value[3]!.totalValue = Number(resp.buckets?.length ?? 0);
      })
      .catch((error) =>
        console.error('[dashboard] GroupBy(channel) failed:', error),
      ),
  ];

  await Promise.allSettled(tasks);
}

void loadDashboard();
</script>

<template>
  <div class="p-5">
    <AnalysisOverview :items="overviewItems" />
    <AnalysisChartsTabs :tabs="chartTabs" class="mt-5">
      <template #trends>
        <AnalyticsTrends :data="trendResp" />
      </template>
      <template #active>
        <AnalyticsVisits :data="activeResp" />
      </template>
    </AnalysisChartsTabs>

    <div class="mt-5 w-full md:flex">
      <AnalysisChartCard
        class="mt-5 md:mr-4 md:mt-0 md:w-1/3"
        :title="$t('page.analytics.eventTrend')"
      >
        <AnalyticsTrendBar :data="trendResp" />
      </AnalysisChartCard>
      <AnalysisChartCard
        class="mt-5 md:mr-4 md:mt-0 md:w-1/3"
        :title="$t('page.analytics.platformDistribution')"
      >
        <AnalyticsPlatformPie :data="platformResp" />
      </AnalysisChartCard>
      <AnalysisChartCard
        class="mt-5 md:mt-0 md:w-1/3"
        :title="$t('page.analytics.accessSource')"
      >
        <AnalyticsChannelPie :data="channelResp" />
      </AnalysisChartCard>
    </div>
  </div>
</template>
