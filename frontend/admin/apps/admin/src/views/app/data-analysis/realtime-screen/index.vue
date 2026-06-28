<script lang="ts" setup>
import type {
  ubaservicev1_BehaviorEvent,
  ubaservicev1_GroupByResponse,
  ubaservicev1_RiskEvent,
} from '#/generated/api/admin/service/v1';

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import dayjs from 'dayjs';

import {
  fetchEventTrend,
  fetchGroupBy,
  fetchListBehaviorEvents,
  fetchListRiskEvents,
  lastDaysRange,
  PaginationQuery,
} from '#/api';
import { $t } from '#/locales';

// 大屏采用 5 秒轮询（列表/KPI），图表数据 30 秒刷新一次（避免高频聚合查询）。
// TODO: 后端 SSE 推送 risk_event / behavior_event 就绪后切换为订阅。
const POLL_INTERVAL = 5000;
const CHART_INTERVAL = 30_000;

const riskLevelToColor: Record<string, string> = {
  HIGH: '#F53F3F',
  LOW: '#86909C',
  MEDIUM: '#FF7D00',
};

// ============ 列表 + KPI 数据 ============
const recentEvents = ref<ubaservicev1_BehaviorEvent[]>([]);
const recentRisks = ref<ubaservicev1_RiskEvent[]>([]);
const todayEventCount = ref(0);
const onlineSessions = ref(0);
const riskAlerts = ref(0);
const eventsPerMinute = ref(0);

// ============ 图表数据 ============
const platformResp = ref<ubaservicev1_GroupByResponse>();
const channelResp = ref<ubaservicev1_GroupByResponse>();
const trendPoints = ref<{ ts: number; value: number }[]>([]);
const highRiskCount = ref(0);

// ============ ECharts 实例 ============
const trendRef = ref<EchartsUIType>();
const platformRef = ref<EchartsUIType>();
const channelRef = ref<EchartsUIType>();
const { renderEcharts: renderTrend } = useEcharts(trendRef);
const { renderEcharts: renderPlatform } = useEcharts(platformRef);
const { renderEcharts: renderChannel } = useEcharts(channelRef);

let pollTimer: null | ReturnType<typeof setInterval> = null;
let chartTimer: null | ReturnType<typeof setInterval> = null;

// ============ 列表 + KPI 刷新（高频）============
async function refreshLists() {
  try {
    const evResp = await fetchListBehaviorEvents(
      new PaginationQuery({
        paging: { page: 1, pageSize: 50 },
        orderBy: ['-event_ts'],
      }),
    );
    recentEvents.value = (evResp.items ?? []).slice(0, 30);
    todayEventCount.value = Number(evResp.total ?? 0);

    // 每分钟事件速率：近 1 分钟事件数
    const oneMinAgo = Date.now() - 60 * 1000;
    eventsPerMinute.value = recentEvents.value.filter(
      (e) => Number(e.eventTs ?? 0) >= oneMinAgo,
    ).length;

    // 在线会话：近 30 分钟去重 session
    const recent = recentEvents.value.filter(
      (e) => Number(e.eventTs ?? 0) >= Date.now() - 30 * 60 * 1000,
    );
    onlineSessions.value = new Set(
      recent.map((e) => e.sessionId).filter(Boolean),
    ).size;

    const riskResp = await fetchListRiskEvents(
      new PaginationQuery({
        paging: { page: 1, pageSize: 50 },
        orderBy: ['-occur_time'],
      }),
    );
    recentRisks.value = (riskResp.items ?? []).slice(0, 20);
    riskAlerts.value = Number(riskResp.total ?? 0);
    // 高危事件计数（前端 reduce）
    highRiskCount.value = recentRisks.value.filter(
      (r) => r.riskLevel === 'HIGH',
    ).length;
  } catch {
    // 轮询失败静默，下次重试
  }
}

// ============ 图表刷新（低频聚合）============
async function refreshCharts() {
  const now = Date.now();
  const tasks: Promise<unknown>[] = [
    // 事件趋势：近 1 小时，按小时粒度（接口最小粒度）
    fetchEventTrend({
      timeRange: { startMs: now - 60 * 60 * 1000, endMs: now },
      granularity: 'HOUR',
    })
      .then((resp) => {
        trendPoints.value = (resp.points ?? []).map((p) => ({
          ts: Number(p.timestamp ?? 0),
          value: Number(p.value ?? 0),
        }));
      })
      .catch(() => {}),
    // 平台分布
    fetchGroupBy({
      timeRange: { startMs: now - 24 * 60 * 60 * 1000, endMs: now },
      dimension: 'platform',
      metric: 'COUNT',
      topN: 8,
    })
      .then((resp) => {
        platformResp.value = resp;
      })
      .catch(() => {}),
    // 渠道分布
    fetchGroupBy({
      timeRange: { startMs: now - 24 * 60 * 60 * 1000, endMs: now },
      dimension: 'channel',
      metric: 'COUNT',
      topN: 8,
    })
      .then((resp) => {
        channelResp.value = resp;
      })
      .catch(() => {}),
  ];
  await Promise.allSettled(tasks);
}

// ============ 图表 option ============
const trendOption = computed(() => ({
  grid: { bottom: 24, left: '3%', right: '3%', top: '8%' },
  series: [
    {
      areaStyle: { opacity: 0.25 },
      data: trendPoints.value.map((p) => p.value),
      itemStyle: { color: '#06b6d4' },
      smooth: true,
      type: 'line',
    },
  ],
  tooltip: { trigger: 'axis' },
  xAxis: {
    boundaryGap: false,
    data: trendPoints.value.map((p) => dayjs(p.ts).format('HH:mm')),
    type: 'category',
  },
  yAxis: { type: 'value' },
}));

function pieOption(resp: typeof platformResp.value) {
  const buckets = resp?.buckets ?? [];
  return {
    legend: { bottom: 0, left: 'center', type: 'scroll' },
    series: [
      {
        color: [
          '#06b6d4',
          '#8b5cf6',
          '#10b981',
          '#f59e0b',
          '#ef4444',
          '#3b82f6',
          '#ec4899',
          '#14b8a6',
        ],
        data:
          buckets.length > 0
            ? buckets.map((b) => ({
                name: b.label || 'unknown',
                value: Number(b.value ?? 0),
              }))
            : [{ name: $t('page.analytics.noData'), value: 1 }],
        label: { show: false },
        radius: ['40%', '68%'],
        tooltip: { trigger: 'item' },
        type: 'pie',
      },
    ],
  };
}

const platformOption = computed(() => pieOption(platformResp.value));
const channelOption = computed(() => pieOption(channelResp.value));

// 风险等级分布（前端 reduce recentRisks）
const riskLevelDist = computed(() => {
  const map: Record<string, number> = { HIGH: 0, MEDIUM: 0, LOW: 0 };
  for (const r of recentRisks.value) {
    const lv = r.riskLevel ?? 'LOW';
    map[lv] = (map[lv] ?? 0) + 1;
  }
  return map;
});

// 风险类型 TOP（前端 reduce）
const riskTypeTop = computed(() => {
  const map: Record<string, number> = {};
  for (const r of recentRisks.value) {
    const t = r.riskType || 'unknown';
    map[t] = (map[t] ?? 0) + 1;
  }
  return Object.entries(map)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5);
});

// 图表 option 变化时触发渲染（数据刷新后自动重绘）
watch(trendOption, (opt) => renderTrend(opt as any));
watch(platformOption, (opt) => renderPlatform(opt as any));
watch(channelOption, (opt) => renderChannel(opt as any));

onMounted(() => {
  refreshLists();
  refreshCharts();
  pollTimer = setInterval(refreshLists, POLL_INTERVAL);
  chartTimer = setInterval(refreshCharts, CHART_INTERVAL);
});

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (chartTimer) clearInterval(chartTimer);
});

function formatTime(ts?: number | string) {
  if (!ts) return '';
  return dayjs(typeof ts === 'number' ? ts : Number(ts)).format('HH:mm:ss');
}

// 兼容 lastDaysRange 未使用告警
void lastDaysRange;
</script>

<template>
  <Page auto-content-height>
    <div class="bg-card text-card-foreground min-h-full rounded-lg border p-6">
      <!-- 顶部标题 + 实时状态 -->
      <div class="mb-6 flex items-center justify-between">
        <h2 class="text-2xl font-bold tracking-wide">
          {{ $t('page.analytics.realtimeScreen') }}
        </h2>
        <span
          class="flex items-center gap-2 text-sm text-emerald-600 dark:text-emerald-400"
        >
          <span
            class="inline-block size-2 animate-pulse rounded-full bg-emerald-600 dark:bg-emerald-400"
          ></span>
          {{ dayjs().format('YYYY-MM-DD HH:mm:ss') }}
        </span>
      </div>

      <!-- KPI 区（4 项核心指标）-->
      <div class="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div class="bg-background rounded-lg border p-5">
          <div class="text-muted-foreground text-sm">
            {{ $t('page.analytics.eventCount') }}
          </div>
          <div class="mt-2 text-3xl font-bold text-cyan-600 dark:text-cyan-400">
            {{ todayEventCount.toLocaleString() }}
          </div>
          <div class="text-muted-foreground mt-1 text-xs">
            {{ eventsPerMinute }} {{ $t('page.analytics.eventsPerMinute') }}
          </div>
        </div>
        <div class="bg-background rounded-lg border p-5">
          <div class="text-muted-foreground text-sm">
            {{ $t('page.analytics.onlineSessions') }}
          </div>
          <div
            class="mt-2 text-3xl font-bold text-emerald-600 dark:text-emerald-400"
          >
            {{ onlineSessions.toLocaleString() }}
          </div>
          <div class="text-muted-foreground mt-1 text-xs">30min</div>
        </div>
        <div class="bg-background rounded-lg border p-5">
          <div class="text-muted-foreground text-sm">
            {{ $t('page.analytics.riskAlerts') }}
          </div>
          <div class="mt-2 text-3xl font-bold text-rose-600 dark:text-rose-400">
            {{ riskAlerts.toLocaleString() }}
          </div>
          <div class="text-muted-foreground mt-1 text-xs">
            {{ $t('page.analytics.highRisk') }}: {{ highRiskCount }}
          </div>
        </div>
        <div class="bg-background rounded-lg border p-5">
          <div class="text-muted-foreground text-sm">
            {{ $t('page.analytics.eventsPerMinute') }}
          </div>
          <div
            class="mt-2 text-3xl font-bold text-violet-600 dark:text-violet-400"
          >
            {{ eventsPerMinute.toLocaleString() }}
          </div>
          <div class="text-muted-foreground mt-1 text-xs">realtime</div>
        </div>
      </div>

      <!-- 事件趋势折线（近1小时）-->
      <div class="bg-background mb-6 rounded-lg border p-4">
        <h3 class="text-foreground mb-3 text-base font-semibold">
          {{ $t('page.analytics.eventTrend') }}（1h）
        </h3>
        <EchartsUI ref="trendRef" height="220px" />
      </div>

      <!-- 分布图 + 风控统计 -->
      <div class="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div class="bg-background rounded-lg border p-4">
          <h3 class="text-foreground mb-3 text-base font-semibold">
            {{ $t('page.analytics.platformDistribution') }}
          </h3>
          <EchartsUI ref="platformRef" height="240px" />
        </div>
        <div class="bg-background rounded-lg border p-4">
          <h3 class="text-foreground mb-3 text-base font-semibold">
            {{ $t('page.analytics.accessSource') }}
          </h3>
          <EchartsUI ref="channelRef" height="240px" />
        </div>
        <!-- 风险等级分布 -->
        <div class="bg-background rounded-lg border p-4">
          <h3 class="text-foreground mb-3 text-base font-semibold">
            {{ $t('page.analytics.riskLevelDist') }}
          </h3>
          <div class="space-y-3">
            <div
              v-for="(cnt, lv) in riskLevelDist"
              :key="lv"
              class="flex items-center gap-3"
            >
              <span
                class="inline-block size-3 rounded-full"
                :style="{ background: riskLevelToColor[lv] ?? '#86909C' }"
              ></span>
              <span class="text-muted-foreground w-16 text-sm">{{ lv }}</span>
              <div class="bg-muted h-2 flex-1 rounded-full">
                <div
                  class="h-2 rounded-full transition-all"
                  :style="{
                    width: `${riskAlerts > 0 ? (cnt / riskAlerts) * 100 : 0}%`,
                    background: riskLevelToColor[lv] ?? '#86909C',
                  }"
                ></div>
              </div>
              <span
                class="text-foreground w-8 text-right text-sm font-semibold"
              >
                {{ cnt }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 实时流（事件 + 风险 + 风险类型TOP）-->
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <!-- 实时事件流 -->
        <div class="bg-background rounded-lg border p-4">
          <h3 class="text-foreground mb-3 text-base font-semibold">
            {{ $t('page.analytics.recentEvents') }}
          </h3>
          <div class="max-h-80 space-y-2 overflow-y-auto">
            <div
              v-for="(ev, idx) in recentEvents"
              :key="idx"
              class="bg-muted flex items-center justify-between rounded px-3 py-2 text-sm"
            >
              <div class="flex items-center gap-2 overflow-hidden">
                <span class="truncate text-cyan-600 dark:text-cyan-400">{{
                  ev.eventName
                }}</span>
                <span
                  v-if="ev.userId"
                  class="text-muted-foreground shrink-0 text-xs"
                >
                  uid:{{ ev.userId }}
                </span>
              </div>
              <span class="text-muted-foreground shrink-0 text-xs">{{
                formatTime(ev.eventTs)
              }}</span>
            </div>
            <div
              v-if="recentEvents.length === 0"
              class="text-muted-foreground py-8 text-center"
            >
              {{ $t('page.analytics.noData') }}
            </div>
          </div>
        </div>

        <!-- 风险告警流（高危置顶高亮）-->
        <div class="bg-background rounded-lg border p-4">
          <h3
            class="mb-3 text-base font-semibold text-rose-600 dark:text-rose-400"
          >
            {{ $t('page.analytics.riskAlerts') }}
          </h3>
          <div class="max-h-80 space-y-2 overflow-y-auto">
            <div
              v-for="(risk, idx) in recentRisks"
              :key="idx"
              class="flex items-center justify-between rounded border-l-2 px-3 py-2 text-sm"
              :style="{
                borderLeftColor:
                  riskLevelToColor[risk.riskLevel ?? ''] ?? '#86909C',
                background:
                  risk.riskLevel === 'HIGH'
                    ? 'var(--destructive-foreground, rgba(244,63,94,0.08))'
                    : undefined,
              }"
            >
              <div class="flex items-center gap-2 overflow-hidden">
                <span
                  class="shrink-0 font-medium text-rose-600 dark:text-rose-400"
                >
                  {{ risk.riskType }}
                </span>
                <span
                  v-if="risk.ruleName"
                  class="text-muted-foreground truncate text-xs"
                >
                  {{ risk.ruleName }}
                </span>
              </div>
              <span class="text-muted-foreground shrink-0 text-xs">{{
                formatTime(
                  risk.occurTime
                    ? (risk.occurTime as any).seconds * 1000
                    : undefined,
                )
              }}</span>
            </div>
            <div
              v-if="recentRisks.length === 0"
              class="text-muted-foreground py-8 text-center"
            >
              {{ $t('page.analytics.noData') }}
            </div>
          </div>
        </div>

        <!-- 风险类型 TOP5 -->
        <div class="bg-background rounded-lg border p-4">
          <h3 class="text-foreground mb-3 text-base font-semibold">
            {{ $t('page.analytics.riskTypeTop') }}
          </h3>
          <div class="space-y-3">
            <div
              v-for="[type, cnt] in riskTypeTop"
              :key="type"
              class="flex items-center gap-3"
            >
              <span class="text-muted-foreground w-28 truncate text-sm">
                {{ type }}
              </span>
              <div class="bg-muted h-2 flex-1 rounded-full">
                <div
                  class="h-2 rounded-full bg-rose-500 transition-all"
                  :style="{
                    width: `${
                      riskTypeTop[0] && riskTypeTop[0][1] > 0
                        ? (cnt / riskTypeTop[0][1]) * 100
                        : 0
                    }%`,
                  }"
                ></div>
              </div>
              <span
                class="text-foreground w-8 text-right text-sm font-semibold"
              >
                {{ cnt }}
              </span>
            </div>
            <div
              v-if="riskTypeTop.length === 0"
              class="text-muted-foreground py-8 text-center"
            >
              {{ $t('page.analytics.noData') }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </Page>
</template>
