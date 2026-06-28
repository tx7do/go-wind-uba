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
  const data = buckets
    .map((b) => ({ name: b.label || 'unknown', value: Number(b.value ?? 0) }))
    .sort((a, b) => a.value - b.value);

  renderEcharts({
    series: [
      {
        animationDelay() {
          return Math.random() * 400;
        },
        animationEasing: 'exponentialInOut',
        animationType: 'scale',
        center: ['50%', '50%'],
        color: [
          '#5ab1ef',
          '#b6a2de',
          '#67e0e3',
          '#2ec7c9',
          '#fa8c16',
          '#13c2c2',
        ],
        data: data.length > 0 ? data : [{ name: '暂无数据', value: 1 }],
        name: props.data?.dimension ?? 'channel',
        radius: '80%',
        roseType: 'radius',
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
