<script lang="ts" setup>
import type { ubaservicev1_ActiveUsersResponse } from '#/generated/api/admin/service/v1';

import { computed, onMounted, ref, watch } from 'vue';

import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';
import { $t } from '@vben/locales';

import dayjs from 'dayjs';

interface Props {
  data?: ubaservicev1_ActiveUsersResponse;
}

const props = defineProps<Props>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

const points = computed(() => props.data?.points ?? []);

function render() {
  const xData = points.value.map((p) => dayjs(p.timestamp ?? 0).format('MM-DD'));
  const dau = points.value.map((p) => Number(p.dau ?? 0));
  const wau = points.value.map((p) => Number(p.wau ?? 0));
  const mau = points.value.map((p) => Number(p.mau ?? 0));

  renderEcharts({
    grid: { bottom: 0, containLabel: true, left: '1%', right: '1%', top: '12%' },
    legend: {
      data: [
        $t('page.analytics.dau'),
        $t('page.analytics.wau'),
        $t('page.analytics.mau'),
      ],
      top: 0,
    },
    series: [
      {
        barMaxWidth: 80,
        data: dau,
        itemStyle: { color: '#5ab1ef' },
        name: $t('page.analytics.dau'),
        type: 'bar',
      },
      {
        data: wau,
        itemStyle: { color: '#019680' },
        name: $t('page.analytics.wau'),
        smooth: true,
        type: 'line',
      },
      {
        data: mau,
        itemStyle: { color: '#fa8c16' },
        name: $t('page.analytics.mau'),
        smooth: true,
        type: 'line',
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
