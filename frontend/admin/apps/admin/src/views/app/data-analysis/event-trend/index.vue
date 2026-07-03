<script lang="ts" setup>
import type {
  AnalyticsDimension,
  AnalyticsGranularity,
  AnalyticsMetric,
} from '#/api';
import type {
  ubaservicev1_EventSeries,
  ubaservicev1_EventTrendRequest,
  ubaservicev1_EventTrendResponse,
  ubaservicev1_PropertyFilter,
  ubaservicev1_PropertyFilter_FieldScope,
  ubaservicev1_PropertyFilter_Operator,
} from '#/generated/api/admin/service/v1';

import { computed, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import { fetchEventTrend, lastDaysRange } from '#/api';
import { useListEventSchemas } from '#/api/composables/event-schema';
import { $t } from '#/locales';
import { PaginationQuery } from '#/transport/rest';

import dayjs from 'dayjs';

import AnalyticsToolbar from '../components/analytics-toolbar.vue';

// ==============================
// 查询条件状态
// ==============================
const range = ref(lastDaysRange(7));
const granularity = ref<AnalyticsGranularity>(
  'ANALYTICS_GRANULARITY_UNSPECIFIED',
);
const selectedEvents = ref<string[]>([]);
const metric = ref<AnalyticsMetric>('COUNT');
const dimension = ref<string>(''); // 空=不拆分
const chartType = ref<'bar' | 'line' | 'pie' | 'table'>('line');

interface FilterRow {
  field: string;
  op: ubaservicev1_PropertyFilter_Operator;
  scope: ubaservicev1_PropertyFilter_FieldScope;
  values: string;
}
const filterRows = ref<FilterRow[]>([]);

const loading = ref(false);
const data = ref<ubaservicev1_EventTrendResponse>();

// ==============================
// 事件 schema 下拉数据源
// ==============================
const schemaResp = useListEventSchemas(
  new PaginationQuery({ paging: { page: 1, pageSize: 1000 } }),
);
const eventOptions = computed(() => {
  const items = schemaResp.data?.value?.items ?? [];
  return items
    .filter((s) => s.status === 'ENABLED' || s.status === undefined)
    .map((s) => ({
      label: s.displayName || s.eventName || '',
      value: s.eventName || '',
    }));
});

// ==============================
// 下拉选项
// ==============================
const metricOptions: { label: string; value: AnalyticsMetric }[] = [
  { label: $t('page.analytics.count'), value: 'COUNT' },
  { label: $t('page.analytics.activeUsers'), value: 'UNIQUE_USER' },
  { label: $t('page.analytics.metricSumAmount'), value: 'SUM_AMOUNT' },
  { label: $t('page.analytics.metricAvgAmount'), value: 'AVG_AMOUNT' },
  { label: $t('page.analytics.metricPerUser'), value: 'PER_USER' },
];

const dimensionOptions: { label: string; value: AnalyticsDimension }[] = [
  { label: $t('page.analytics.dimPlatform'), value: 'platform' },
  { label: $t('page.analytics.accessSource'), value: 'channel' },
  { label: $t('page.analytics.dimCountry'), value: 'country' },
  { label: $t('page.analytics.dimAppVersion'), value: 'app_version' },
  { label: $t('page.analytics.eventName'), value: 'event_name' },
  { label: $t('page.analytics.dimOs'), value: 'os' },
];

const scopeOptions: {
  label: string;
  value: ubaservicev1_PropertyFilter_FieldScope;
}[] = [
  { label: $t('page.analytics.scopeDimension'), value: 'DIMENSION' },
  { label: $t('page.analytics.scopeContext'), value: 'EVENT_CONTEXT' },
  { label: $t('page.analytics.scopeProperty'), value: 'EVENT_PROPERTY' },
  { label: $t('page.analytics.scopeMetric'), value: 'EVENT_METRIC' },
];

const opOptions: {
  label: string;
  value: ubaservicev1_PropertyFilter_Operator;
}[] = [
  { label: $t('page.analytics.opEq'), value: 'EQ' },
  { label: $t('page.analytics.opNeq'), value: 'NEQ' },
  { label: $t('page.analytics.opContains'), value: 'CONTAINS' },
  { label: $t('page.analytics.opIn'), value: 'IN' },
  { label: $t('page.analytics.opGt'), value: 'GT' },
  { label: $t('page.analytics.opGte'), value: 'GTE' },
  { label: $t('page.analytics.opLt'), value: 'LT' },
  { label: $t('page.analytics.opLte'), value: 'LTE' },
];

const chartTypeOptions = computed(() => [
  { label: $t('page.analytics.lineChart'), value: 'line' },
  { label: $t('page.analytics.barChart'), value: 'bar' },
  { label: $t('page.analytics.pieChart'), value: 'pie' },
  { label: $t('page.analytics.tableView'), value: 'table' },
]);

function addFilterRow() {
  filterRows.value.push({
    field: '',
    op: 'EQ',
    scope: 'DIMENSION',
    values: '',
  });
}
function removeFilterRow(idx: number) {
  filterRows.value.splice(idx, 1);
}

// ==============================
// 构造请求并加载
// ==============================
function buildRequest(): ubaservicev1_EventTrendRequest {
  // 多事件：每个事件一条 query；空选=单条查全部事件
  const queries =
    selectedEvents.value.length > 0
      ? selectedEvents.value.map((e) => ({
          eventName: e,
          metric: metric.value,
        }))
      : [{ metric: metric.value }];

  // 过滤条件 → FilterGroup（IN 操作按逗号拆多值）
  const filters: ubaservicev1_PropertyFilter[] = filterRows.value
    .filter((r) => r.field.trim() !== '')
    .map((r) => ({
      scope: r.scope,
      field: r.field.trim(),
      op: r.op,
      values:
        r.op === 'IN'
          ? r.values
              .split(',')
              .map((v) => v.trim())
              .filter(Boolean)
          : [r.values],
    }));

  return {
    timeRange: range.value,
    granularity: granularity.value,
    queries,
    dimension: dimension.value || undefined,
    globalFilter:
      filters.length > 0 ? { filters } : undefined,
  };
}

async function load() {
  loading.value = true;
  try {
    data.value = await fetchEventTrend(buildRequest());
  } catch {
    data.value = undefined;
  } finally {
    loading.value = false;
  }
}

function onToolbarChange(payload: {
  endMs: number;
  granularity: AnalyticsGranularity;
  startMs: number;
}) {
  range.value = { endMs: payload.endMs, startMs: payload.startMs };
  granularity.value = payload.granularity;
  load();
}

// 条件变化时自动重新查询
watch([selectedEvents, metric, dimension, filterRows], load, { deep: true });
void load();

// ==============================
// 图表渲染
// ==============================
const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

// 所有时间桶（横轴并集，按时间排序）
const allTimestamps = computed(() => {
  const set = new Set<number>();
  for (const s of data.value?.series ?? []) {
    for (const p of s.points ?? []) {
      if (p.timestamp) set.add(p.timestamp);
    }
  }
  return [...set].sort((a, b) => a - b);
});

const seriesData = computed(() => {
  const series = data.value?.series ?? [];
  return series.map((s: ubaservicev1_EventSeries) => {
    const m = new Map<number, number>();
    for (const p of s.points ?? []) {
      if (p.timestamp) m.set(p.timestamp, Number(p.value ?? 0));
    }
    return {
      data: allTimestamps.value.map((ts) => m.get(ts) ?? 0),
      name: s.name || '',
      total: Number(s.total ?? 0),
      type: chartType.value === 'bar' ? 'bar' : 'line',
    };
  });
});

const lineBarOption = computed(() => {
  const fmt =
    granularity.value === 'HOUR' ? 'MM-DD HH:mm' : 'MM-DD';
  return {
    color: ['#5ab1ef', '#fa8c16', '#52c41a', '#eb2f96', '#722ed1', '#13c2c2'],
    grid: { bottom: 40, containLabel: true, left: '3%', right: '3%', top: '12%' },
    legend: { bottom: 0, left: 'center', type: 'scroll' },
    series: seriesData.value.map((s) => ({
      areaStyle: { opacity: 0.15 },
      barMaxWidth: 40,
      data: s.data,
      name: s.name,
      smooth: true,
      type: s.type,
    })),
    tooltip: { trigger: 'axis' },
    xAxis: {
      boundaryGap: chartType.value === 'bar',
      data: allTimestamps.value.map((ts) => dayjs(ts).format(fmt)),
      type: 'category',
    },
    yAxis: { type: 'value' },
  } as any;
});

const pieOption = computed(() => {
  const series = data.value?.series ?? [];
  return {
    legend: { bottom: 0, left: 'center', type: 'scroll' },
    series: [
      {
        avoidLabelOverlap: true,
        color: ['#5ab1ef', '#fa8c16', '#52c41a', '#eb2f96', '#722ed1'],
        data: series.map((s) => ({
          name: s.name || '',
          value: Number(s.total ?? 0),
        })),
        emphasis: {
          label: { fontSize: 16, fontWeight: 'bold', show: true },
        },
        label: { formatter: '{b}: {d}%', show: true },
        radius: ['40%', '70%'],
        type: 'pie',
      },
    ],
    tooltip: { formatter: '{b}: {c} ({d}%)', trigger: 'item' },
  } as any;
});

const chartOption = computed(() => {
  if (chartType.value === 'pie') return pieOption.value;
  return lineBarOption.value;
});

// 表格视图数据：时间 × series 的宽表
const tableColumns = computed(() => {
  const cols: any[] = [
    {
      dataIndex: 'time',
      key: 'time',
      title: $t('page.analytics.granularity'),
      width: 160,
    },
  ];
  for (const s of data.value?.series ?? []) {
    cols.push({
      dataIndex: s.name,
      key: s.name,
      title: s.name,
    });
  }
  return cols;
});
const tableDataSource = computed(() => {
  const series = data.value?.series ?? [];
  const fmt = granularity.value === 'HOUR' ? 'YYYY-MM-DD HH:mm' : 'YYYY-MM-DD';
  return allTimestamps.value.map((ts) => {
    const row: Record<string, any> = { key: ts, time: dayjs(ts).format(fmt) };
    series.forEach((s) => {
      const p = (s.points ?? []).find((pt) => pt.timestamp === ts);
      row[s.name || ''] = p ? Number(p.value ?? 0) : 0;
    });
    return row;
  });
});

// 汇总指标
const totalSum = computed(() =>
  (data.value?.series ?? []).reduce((acc, s) => acc + Number(s.total ?? 0), 0),
);

// option 变化时触发渲染（表格类型不渲染 ECharts）
watch(chartOption, (opt) => {
  if (chartType.value !== 'table') renderEcharts(opt as any);
});
watch(chartType, () => {
  if (chartType.value !== 'table') renderEcharts(chartOption.value as any);
});
</script>

<template>
  <Page auto-content-height>
    <!-- 顶部：标题 + 时间/粒度 -->
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-lg font-semibold">
        {{ $t('page.analytics.eventAnalysis') }}
      </h3>
      <AnalyticsToolbar
        :end-ms="range.endMs"
        :granularity="granularity"
        :start-ms="range.startMs"
        @change="onToolbarChange"
      />
    </div>

    <!-- 查询条件卡片 -->
    <div class="bg-background mb-4 rounded-lg p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground w-16 shrink-0">{{
            $t('page.analytics.selectEvent')
          }}</span>
          <a-select
            v-model:value="selectedEvents"
            :max-tag-count="3"
            :options="eventOptions"
            :placeholder="$t('page.analytics.allEvents')"
            allow-clear
            class="w-72"
            mode="multiple"
            show-search
          />
        </div>

        <div class="flex items-center gap-2">
          <span class="text-muted-foreground w-10 shrink-0">{{
            $t('page.analytics.metric')
          }}</span>
          <a-select v-model:value="metric" class="w-32">
            <a-select-option
              v-for="opt in metricOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </a-select-option>
          </a-select>
        </div>

        <div class="flex items-center gap-2">
          <span class="text-muted-foreground w-16 shrink-0">{{
            $t('page.analytics.splitDimension')
          }}</span>
          <a-select
            v-model:value="dimension"
            :placeholder="$t('page.analytics.noSplit')"
            allow-clear
            class="w-40"
          >
            <a-select-option value="">{{ $t('page.analytics.noSplit') }}</a-select-option>
            <a-select-option
              v-for="opt in dimensionOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </a-select-option>
          </a-select>
        </div>

        <div class="flex items-center gap-2">
          <span class="text-muted-foreground w-16 shrink-0">{{
            $t('page.analytics.chartType')
          }}</span>
          <a-radio-group v-model:value="chartType">
            <a-radio-button
              v-for="opt in chartTypeOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </a-radio-button>
          </a-radio-group>
        </div>
      </div>

      <!-- 过滤条件构建器 -->
      <div class="mt-4 border-t pt-3">
        <div class="text-muted-foreground mb-2 text-sm">
          {{ $t('page.analytics.filterCondition') }}
        </div>
        <div class="space-y-2">
          <div
            v-for="(row, idx) in filterRows"
            :key="idx"
            class="flex flex-wrap items-center gap-2"
          >
            <a-select v-model:value="row.scope" class="w-32">
              <a-select-option
                v-for="opt in scopeOptions"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </a-select-option>
            </a-select>
            <a-input
              v-model:value="row.field"
              :placeholder="$t('page.analytics.filterField')"
              class="w-40"
            />
            <a-select v-model:value="row.op" class="w-28">
              <a-select-option
                v-for="opt in opOptions"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </a-select-option>
            </a-select>
            <a-input
              v-model:value="row.values"
              :placeholder="
                row.op === 'IN'
                  ? `${$t('page.analytics.filterValue')} (v1,v2)`
                  : $t('page.analytics.filterValue')
              "
              class="w-48"
            />
            <a-button danger size="small" type="link" @click="removeFilterRow(idx)">
              ×
            </a-button>
          </div>
          <a-button size="small" type="dashed" @click="addFilterRow">
            + {{ $t('page.analytics.addCondition') }}
          </a-button>
        </div>
      </div>
    </div>

    <!-- 汇总数字 -->
    <div class="mb-4 flex flex-wrap items-center gap-6">
      <div class="text-muted-foreground">
        {{ $t('page.analytics.totalEvents') }}:
        <span class="text-foreground text-lg font-semibold">
          {{ totalSum.toLocaleString() }}
        </span>
      </div>
      <div class="text-muted-foreground">
        {{ $t('page.analytics.seriesCount') }}:
        <span class="text-foreground text-lg font-semibold">
          {{ data?.series?.length ?? 0 }}
        </span>
      </div>
    </div>

    <!-- 图表/表格区 -->
    <div class="bg-background relative rounded-lg p-4">
      <a-spin
        v-if="loading"
        class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-white/60"
      />
      <a-table
        v-if="chartType === 'table'"
        :columns="tableColumns"
        :data-source="tableDataSource"
        :pagination="{ pageSize: 20 }"
        size="small"
      />
      <EchartsUI v-else ref="chartRef" height="450px" />
    </div>
  </Page>
</template>
</content>
</invoke>