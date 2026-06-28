<script lang="ts" setup>
import type { AnalyticsGranularity } from '#/api';

import { computed } from 'vue';

import dayjs from 'dayjs';

import { $t } from '#/locales';

interface Props {
  /** 毫秒时间戳范围 */
  endMs?: number;
  /** 选中的时间粒度 */
  granularity?: AnalyticsGranularity;
  /** 是否显示粒度选择器 */
  showGranularity?: boolean;
  /** 快捷范围（天数），默认 7 */
  startMs?: number;
}

const props = withDefaults(defineProps<Props>(), {
  endMs: undefined,
  granularity: 'ANALYTICS_GRANULARITY_UNSPECIFIED',
  showGranularity: true,
  startMs: undefined,
});

const emit = defineEmits<{
  change: [
    payload: {
      endMs: number;
      granularity: AnalyticsGranularity;
      startMs: number;
    },
  ];
}>();

// antd 4.x RangePicker 用 presets（数组），ranges 已废弃
const presets = computed(() => [
  {
    label: _('today'),
    value: [dayjs().startOf('day'), dayjs()] as [dayjs.Dayjs, dayjs.Dayjs],
  },
  {
    label: _('last7'),
    value: [dayjs().subtract(7, 'day'), dayjs()] as [dayjs.Dayjs, dayjs.Dayjs],
  },
  {
    label: _('last30'),
    value: [dayjs().subtract(30, 'day'), dayjs()] as [dayjs.Dayjs, dayjs.Dayjs],
  },
  {
    label: _('last90'),
    value: [dayjs().subtract(90, 'day'), dayjs()] as [dayjs.Dayjs, dayjs.Dayjs],
  },
]);

const rangeValue = computed(() => {
  const start = props.startMs
    ? dayjs(props.startMs)
    : dayjs().subtract(7, 'day');
  const end = props.endMs ? dayjs(props.endMs) : dayjs();
  return [start, end] as [dayjs.Dayjs, dayjs.Dayjs];
});

function _(key: string): string {
  const map: Record<string, string> = {
    last30: $t('page.analytics.last30Days'),
    last7: $t('page.analytics.last7Days'),
    last90: $t('page.analytics.last90Days'),
    today: $t('page.analytics.today'),
  };
  return map[key] ?? key;
}

function handleRangeChange(_values: unknown, dateStrings: [string, string]) {
  const start = dateStrings[0]
    ? dayjs(dateStrings[0]).startOf('day').valueOf()
    : 0;
  const end = dateStrings[1]
    ? dayjs(dateStrings[1]).endOf('day').valueOf()
    : Date.now();
  emit('change', {
    endMs: end,
    granularity: props.granularity,
    startMs: start,
  });
}

function handleGranularityChange(value: AnalyticsGranularity) {
  emit('change', {
    endMs: props.endMs ?? Date.now(),
    granularity: value,
    startMs: props.startMs ?? dayjs().subtract(7, 'day').valueOf(),
  });
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-3">
    <a-range-picker
      :presets="presets"
      :value="rangeValue"
      allow-clear
      show-time
      @change="handleRangeChange"
    />
    <template v-if="showGranularity">
      <span class="text-muted-foreground">{{
        $t('page.analytics.granularity')
      }}</span>
      <a-select
        :value="granularity"
        class="w-32"
        @change="handleGranularityChange"
      >
        <a-select-option value="ANALYTICS_GRANULARITY_UNSPECIFIED">
          {{ $t('page.analytics.auto') }}
        </a-select-option>
        <a-select-option value="HOUR">
          {{ $t('page.analytics.hour') }}
        </a-select-option>
        <a-select-option value="DAY">
          {{ $t('page.analytics.day') }}
        </a-select-option>
        <a-select-option value="WEEK">
          {{ $t('page.analytics.week') }}
        </a-select-option>
        <a-select-option value="MONTH">
          {{ $t('page.analytics.month') }}
        </a-select-option>
      </a-select>
    </template>
  </div>
</template>
