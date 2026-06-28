<script lang="ts" setup>
import type { ubaservicev1_EventTrendResponse } from '#/generated/api/admin/service/v1';

import { onMounted, ref, watch } from 'vue';

import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import dayjs from 'dayjs';

interface Props {
  data?: ubaservicev1_EventTrendResponse;
}

const props = defineProps<Props>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

function render() {
  const points = props.data?.points ?? [];
  const xData = points.map((p) => dayjs(p.timestamp ?? 0).format('MM-DD'));
  const yData = points.map((p) => Number(p.value ?? 0));

  renderEcharts({
    series: [
      {
        data: yData,
        itemStyle: { color: '#67e0e3' },
        type: 'bar',
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: { data: xData, type: 'category' },
    yAxis: { type: 'value' },
  });
}

onMounted(render);
watch(() => props.data, render, { deep: true });
</script>

<template>
  <EchartsUI ref="chartRef" />
</template>
