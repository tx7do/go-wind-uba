<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';
import type {
  ubaservicev1_Session as Session,
  ubaservicev1_GroupByResponse,
} from '#/generated/api/admin/service/v1';

import { computed, markRaw, onMounted, ref, watch } from 'vue';

import {
  AnalysisChartCard,
  AnalysisOverview,
  type AnalysisOverviewItem,
  Page,
} from '@vben/common-ui';
import {
  SvgBellIcon,
  SvgCakeIcon,
  SvgCardIcon,
  SvgDownloadIcon,
} from '@vben/icons';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  appPlatformToName,
  enableBoolToColor,
  enableBoolToName,
  fetchGroupBy,
  fetchListRiskEvents,
  fetchListSessions,
  lastDaysRange,
  PaginationQuery,
  platformToColor,
  riskLevelToColor,
  riskLevelToName,
} from '#/api';
import { $t } from '#/locales';

// ============ 顶部 KPI + 平台分布（真实聚合数据） ============
const range = lastDaysRange(7);
const platformResp = ref<ubaservicev1_GroupByResponse>();

const overviewItems = ref<AnalysisOverviewItem[]>([
  {
    icon: markRaw(SvgCardIcon),
    title: $t('page.analytics.sessionCount'),
    totalTitle: $t('page.analytics.bounceRate'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgDownloadIcon),
    title: $t('page.analytics.avgDuration'),
    totalTitle: $t('page.analytics.avgEvents'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgCakeIcon),
    title: $t('page.analytics.totalEvents'),
    totalTitle: $t('page.analytics.platformCount'),
    totalValue: 0,
    value: 0,
  },
  {
    icon: markRaw(SvgBellIcon),
    title: $t('page.analytics.riskEventCount'),
    totalTitle: $t('page.analytics.highRisk'),
    totalValue: 0,
    value: 0,
  },
]);

const pieRef = ref<EchartsUIType>();
const { renderEcharts: renderPie } = useEcharts(pieRef);

const pieOption = computed(() => {
  const buckets = platformResp.value?.buckets ?? [];
  return {
    legend: { bottom: '2%', left: 'center' },
    series: [
      {
        color: [
          '#5ab1ef',
          '#b6a2de',
          '#67e0e3',
          '#2ec7c9',
          '#fa8c16',
          '#13c2c2',
        ],
        data:
          buckets.length > 0
            ? buckets.map((b) => ({
                name: b.label || 'unknown',
                value: Number(b.value ?? 0),
              }))
            : [{ name: $t('page.analytics.noData'), value: 1 }],
        label: { show: false },
        radius: ['40%', '65%'],
        tooltip: { trigger: 'item' },
        type: 'pie',
      },
    ],
  };
});

// 加载 KPI：拉会话样本前端 reduce + 平台分布聚合
async function loadKpi() {
  // 会话样本（首页取较大样本做统计）
  try {
    const sessionResp = await fetchListSessions(
      new PaginationQuery({
        paging: { page: 1, pageSize: 500 },
        orderBy: ['-start_time'],
      }),
    );
    const sessions = sessionResp.items ?? [];
    const totalSessions = Number(sessionResp.total ?? sessions.length);
    const sampleSize = sessions.length;
    const bounceCount = sessions.filter((s) => s.isBounce).length;
    const durations = sessions.map((s) => Number(s.durationMs ?? 0));
    const eventCounts = sessions.map((s) => Number(s.eventCount ?? 0));
    const avgDuration =
      durations.length > 0
        ? Math.round(durations.reduce((a, b) => a + b, 0) / durations.length)
        : 0;
    const avgEvents =
      eventCounts.length > 0
        ? Math.round(
            eventCounts.reduce((a, b) => a + b, 0) / eventCounts.length,
          )
        : 0;
    const totalEvents = eventCounts.reduce((a, b) => a + b, 0);
    // 高风险会话数（大小写归一化，兼容 HIGH/high）
    const highRiskSessions = sessions.filter(
      (s) => String(s.riskLevel ?? '').toLowerCase() === 'high',
    ).length;

    // 卡1：总会话数 / 跳出率(%)
    // 跳出率基于样本计算（分子分母同源，避免 total 远大于样本时系统性偏低）
    overviewItems.value[0]!.value = totalSessions;
    overviewItems.value[0]!.totalValue =
      sampleSize > 0
        ? Number(((bounceCount / sampleSize) * 100).toFixed(1))
        : 0;
    // 卡2：平均时长(ms) / 平均事件数
    overviewItems.value[1]!.value = avgDuration;
    overviewItems.value[1]!.totalValue = avgEvents;
    // 卡3：总事件数 / 平台数（平台数稍后由 GroupBy 覆盖）
    overviewItems.value[2]!.value = totalEvents;
    // 卡4：高风险会话数（副值，主值稍后由风险接口或样本补充）
    overviewItems.value[3]!.totalValue = highRiskSessions;
  } catch {
    // 接口失败保持 0
  }

  // 平台分布（饼图 + 卡3副值）
  try {
    const resp = await fetchGroupBy({
      timeRange: range,
      dimension: 'platform',
      metric: 'COUNT',
      topN: 20,
    });
    platformResp.value = resp;
    // 卡3副值：平台分布数
    overviewItems.value[2]!.totalValue = Number(resp.buckets?.length ?? 0);
  } catch {
    // 接口失败保持 0
  }

  // 风险事件总数（卡4主值）
  try {
    const riskResp = await fetchListRiskEvents(
      new PaginationQuery({
        paging: { page: 1, pageSize: 1 },
        orderBy: ['-occur_time'],
      }),
    );
    overviewItems.value[3]!.value = Number(riskResp.total ?? 0);
  } catch {
    // 接口失败保持 0
  }
}

onMounted(() => {
  loadKpi();
  renderPie(pieOption.value as any);
});
watch(pieOption, (opt) => renderPie(opt as any), { deep: true });

// ============ 会话明细表格 ============
const formOptions = {
  collapsed: false,
  showCollapseButton: true,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'session_id',
      label: $t('page.session.sessionId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'user_id',
      label: $t('page.session.userId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'device_id',
      label: $t('page.session.deviceId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'global_user_id',
      label: $t('page.session.globalUserId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Session> = {
  height: 'auto',
  stripe: true,
  autoResize: true,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
  pagerConfig: {},
  rowConfig: {
    isHover: true,
    resizable: true,
  },
  tooltipConfig: {
    showAll: true,
    enterable: true,
  },
  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        return await fetchListSessions(
          new PaginationQuery({
            paging: {
              page: page.currentPage,
              pageSize: page.pageSize,
            },
            formValues,
          }),
        );
      },
    },
  },
  columns: [
    {
      title: $t('page.session.sessionId'),
      field: 'sessionId',
      minWidth: 200,
      fixed: 'left',
    },
    { title: $t('page.session.userId'), field: 'userId', minWidth: 100 },
    {
      title: $t('page.session.deviceId'),
      field: 'deviceId',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.session.globalUserId'),
      field: 'globalUserId',
      minWidth: 200,
      align: 'left',
    },
    {
      title: $t('page.session.durationMs'),
      field: 'durationMs',
      minWidth: 100,
    },
    {
      title: $t('page.session.eventCount'),
      field: 'eventCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.pageViewCount'),
      field: 'pageViewCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.actionCount'),
      field: 'actionCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.entryPage'),
      field: 'entryPage',
      minWidth: 120,
    },
    { title: $t('page.session.exitPage'), field: 'exitPage', minWidth: 120 },
    {
      title: $t('page.session.isBounce'),
      field: 'isBounce',
      minWidth: 100,
      slots: { default: 'isBounce' },
    },
    {
      title: $t('page.session.platform'),
      field: 'platform',
      minWidth: 100,
      slots: { default: 'platform' },
    },
    { title: $t('page.session.os'), field: 'os', minWidth: 100 },
    {
      title: $t('page.session.appVersion'),
      field: 'appVersion',
      minWidth: 100,
    },
    { title: $t('page.session.ipCity'), field: 'ipCity', minWidth: 100 },
    { title: $t('page.session.country'), field: 'country', minWidth: 100 },
    {
      title: $t('page.session.totalAmount'),
      field: 'totalAmount',
      minWidth: 100,
    },
    {
      title: $t('page.session.payEventCount'),
      field: 'payEventCount',
      minWidth: 100,
    },
    {
      title: $t('page.session.riskLevel'),
      field: 'riskLevel',
      minWidth: 100,
      slots: { default: 'riskLevel' },
    },
    {
      title: $t('page.session.startTime'),
      field: 'startTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
    {
      title: $t('page.session.endTime'),
      field: 'endTime',
      formatter: 'formatDateTime',
      minWidth: 160,
    },
  ],
};

const gridEvents: VxeGridListeners<Session> = {};

const [Grid] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});
</script>

<template>
  <Page auto-content-height>
    <AnalysisOverview :items="overviewItems" class="mb-4" />
    <AnalysisChartCard
      :title="$t('page.analytics.platformDistribution')"
      class="mb-4"
    >
      <EchartsUI ref="pieRef" height="280px" />
    </AnalysisChartCard>
    <Grid :table-title="$t('menu.dataAnalysis.session')">
      <template #platform="{ row }">
        <a-tag :color="platformToColor(row.platform)">
          {{ appPlatformToName(row.platform) }}
        </a-tag>
      </template>
      <template #riskLevel="{ row }">
        <a-tag :color="riskLevelToColor(row.riskLevel)">
          {{ riskLevelToName(row.riskLevel) }}
        </a-tag>
      </template>
      <template #isBounce="{ row }">
        <a-tag :color="enableBoolToColor(row.isBounce)">
          {{ enableBoolToName(row.isBounce) }}
        </a-tag>
      </template>
    </Grid>
  </Page>
</template>

<style scoped></style>
