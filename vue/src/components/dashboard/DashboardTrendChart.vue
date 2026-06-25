<template>
  <div class="trend w-full min-w-0">
    <div
      v-if="loading"
      class="trend__skeleton h-[min(16rem,38vh)] w-full min-h-[200px] rounded-lg bg-slate-100/80 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
      role="img"
      aria-label="趋势图加载中"
    />
    <div
      v-show="!loading"
      ref="chartRef"
      class="trend__chart h-[min(16rem,38vh)] w-full min-h-[200px] min-w-0 rounded-lg bg-[#fafbfc] p-0.5 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { BarChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

use([BarChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const props = withDefaults(
  defineProps<{
    labels: string[]
    created: number[]
    completed: number[]
    dueOnDay: number[]
    loading?: boolean
  }>(),
  { loading: false }
)

const chartRef = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null
const chartFontFamily = 'YB Source Han Sans'

const option = computed<EChartsCoreOption>(() => ({
  color: ['#6f8cff', '#8ee27f', '#ff6b6b'],
  textStyle: { fontFamily: chartFontFamily },
  grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
  tooltip: {
    trigger: 'axis' as const,
    className: 'echarts-tooltip',
    backgroundColor: 'rgba(18, 20, 28, 0.94)',
    borderColor: 'rgba(255, 255, 255, 0.16)',
    borderWidth: 1,
    padding: [10, 12],
    textStyle: {
      color: '#dce6ff',
      fontFamily: chartFontFamily,
      fontSize: 12,
      fontWeight: 600,
    },
    extraCssText: 'border-radius:12px;box-shadow:0 18px 50px -24px rgba(0,0,0,.95);backdrop-filter:blur(18px);',
  },
  legend: {
    data: ['新建', '完成', '当日截止'],
    type: 'scroll' as const,
    top: 0,
    left: 'center',
    textStyle: { color: '#aab5cc', fontFamily: chartFontFamily, fontSize: 10 },
  },
  xAxis: {
    type: 'category' as const,
    data: props.labels,
    axisLine: { lineStyle: { color: 'rgba(220, 230, 255, 0.42)' } },
    axisLabel: { color: 'rgba(220, 230, 255, 0.56)', fontFamily: chartFontFamily, fontSize: 10, interval: 0 },
  },
  yAxis: {
    type: 'value' as const,
    minInterval: 1,
    splitLine: { lineStyle: { color: 'rgba(220, 230, 255, 0.18)' } },
    axisLabel: { color: 'rgba(220, 230, 255, 0.62)', fontFamily: chartFontFamily, fontSize: 10 },
  },
  series: [
    { name: '新建', type: 'bar' as const, data: props.created, barMaxWidth: 16 },
    { name: '完成', type: 'bar' as const, data: props.completed, barMaxWidth: 16 },
    { name: '当日截止', type: 'bar' as const, data: props.dueOnDay, barMaxWidth: 16 },
  ],
}))

function resize() {
  chart?.resize()
}

function render() {
  if (!chartRef.value) return
  if (!chart) {
    chart = init(chartRef.value, undefined, { renderer: 'canvas' })
  }
  chart.setOption(option.value, { notMerge: true })
}

watch(
  option,
  () => {
    if (props.loading) return
    void nextTick(() => {
      render()
    })
  },
  { deep: true }
)

watch(
  () => props.loading,
  (v) => {
    if (!v) {
      void nextTick(() => {
        render()
        resize()
      })
    }
  }
)

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  void nextTick(() => {
    if (!props.loading) {
      render()
    }
    window.addEventListener('resize', resize)
    if (typeof ResizeObserver !== 'undefined' && chartRef.value) {
      resizeObserver = new ResizeObserver(() => {
        resize()
      })
      resizeObserver.observe(chartRef.value)
    }
  })
})

onUnmounted(() => {
  window.removeEventListener('resize', resize)
  resizeObserver?.disconnect()
  resizeObserver = null
  chart?.dispose()
  chart = null
})
</script>
