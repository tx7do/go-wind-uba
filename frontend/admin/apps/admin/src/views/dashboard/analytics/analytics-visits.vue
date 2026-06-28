<script lang="ts" setup>
import type { ubaservicev1_ActiveUsersResponse } from '#/generated/api/admin/service/v1';

import { onMounted, ref, watch } from 'vue';

import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

import dayjs from 'dayjs';

interface Props {
  data?: ubaservicev1_ActiveUsersResponse;
}

const props = defineProps<Props>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

function render() {
  const points = props.data?.points ?? [];
  const xData = points.map((p) => dayjs(p.timestamp ?? 0).format('MM-DD'));
  const dau = points.map((p) => Number(p.dau ?? 0));

  renderEcharts({
    grid: { bottom: 0, containLabel: true, left: '1%', right: '1%', top: '2%' },
    series: [
      {
        barMaxWidth: 80,
        data: dau,
        itemStyle: { color: '#5ab1ef' },
        type: 'bar',
      },
    ],
    tooltip: { axisPointer: { lineStyle: { width: 1 } }, trigger: 'axis' },
    xAxis: { data: xData, type: 'category' },
    yAxis: { splitNumber: 4, type: 'value' },
  });
}

onMounted(render);
watch(() => props.data, render, { deep: true });
</script>

<template>
  <EchartsUI ref="chartRef" />
</template>
