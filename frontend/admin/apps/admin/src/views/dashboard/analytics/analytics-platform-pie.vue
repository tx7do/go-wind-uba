<script lang="ts" setup>
import type { ubaservicev1_GroupByResponse } from '#/generated/api/admin/service/v1';

import { onMounted, ref, watch } from 'vue';

import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

interface Props {
  data?: ubaservicev1_GroupByResponse;
}

const props = defineProps<Props>();

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

function render() {
  const buckets = props.data?.buckets ?? [];
  const data = buckets.map((b) => ({
    name: b.label || 'unknown',
    value: Number(b.value ?? 0),
  }));

  renderEcharts({
    legend: { bottom: '2%', left: 'center' },
    series: [
      {
        animationDelay() {
          return Math.random() * 100;
        },
        animationEasing: 'exponentialInOut',
        animationType: 'scale',
        avoidLabelOverlap: false,
        color: [
          '#5ab1ef',
          '#b6a2de',
          '#67e0e3',
          '#2ec7c9',
          '#fa8c16',
          '#13c2c2',
        ],
        data: data.length > 0 ? data : [{ name: '暂无数据', value: 1 }],
        emphasis: { label: { fontSize: '12', fontWeight: 'bold', show: true } },
        itemStyle: { borderRadius: 10, borderWidth: 2 },
        label: { position: 'center', show: false },
        labelLine: { show: false },
        name: props.data?.dimension ?? 'platform',
        radius: ['40%', '65%'],
        type: 'pie',
      },
    ],
    tooltip: { trigger: 'item' },
  });
}

onMounted(render);
watch(() => props.data, render, { deep: true });
</script>

<template>
  <EchartsUI ref="chartRef" />
</template>
