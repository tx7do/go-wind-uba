<script lang="ts" setup>
import { onMounted, ref, watch } from 'vue';

import { AnalysisChartCard } from '@vben/common-ui';
import {
  EchartsUI,
  type EchartsUIType,
  useEcharts,
} from '@vben/plugins/echarts';

interface Props {
  /** 图表配置；变化时自动重渲染 */
  height?: string;
  loading?: boolean;
  /** ECharts 配置项，使用宽松类型避免直接依赖 echarts 包 */
  option?: Record<string, any>;
  title?: string;
}

const props = withDefaults(defineProps<Props>(), {
  height: '320px',
  loading: false,
  option: undefined,
  title: '',
});

const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

onMounted(() => {
  if (props.option) {
    renderEcharts(props.option);
  }
});

watch(
  () => props.option,
  (opt) => {
    if (opt) {
      renderEcharts(opt);
    }
  },
  { deep: true },
);
</script>

<template>
  <AnalysisChartCard :title="title">
    <div v-if="loading" class="flex h-full items-center justify-center">
      <a-spin />
    </div>
    <EchartsUI v-show="!loading" ref="chartRef" :height="height" />
    <div
      v-if="!loading && !option"
      class="text-muted-foreground flex h-full items-center justify-center"
    >
      {{ '暂无数据' }}
    </div>
  </AnalysisChartCard>
</template>
