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
  const xData = points.map((p) =>
    dayjs(p.timestamp ?? 0).format('MM-DD HH:mm'),
  );
  const yData = points.map((p) => Number(p.value ?? 0));

  renderEcharts({
    grid: { bottom: 0, containLabel: true, left: '1%', right: '1%', top: '2%' },
    series: [
      {
        areaStyle: {},
        data: yData,
        itemStyle: { color: '#5ab1ef' },
        smooth: true,
        type: 'line',
      },
    ],
    tooltip: {
      axisPointer: { lineStyle: { color: '#019680', width: 1 } },
      trigger: 'axis',
    },
    xAxis: {
      axisTick: { show: false },
      boundaryGap: false,
      data: xData,
      splitLine: { lineStyle: { type: 'solid', width: 1 }, show: true },
      type: 'category',
    },
    yAxis: [
      {
        axisTick: { show: false },
        splitArea: { show: true },
        splitNumber: 4,
        type: 'value',
      },
    ],
  });
}

onMounted(render);
watch(
  () => props.data,
  render,
  { deep: true },
);
</script>

<template>
  <EchartsUI ref="chartRef" />
</template>
