<template>
  <div class="trend w-full min-w-0">
    <div
      v-if="loading"
      class="trend__skeleton h-[min(16rem,38vh)] w-full min-h-[200px] rounded-lg bg-[rgb(var(--yb-surface-muted)/0.8)] sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
      role="img"
      aria-label="趋势图加载中"
    />
    <div
      v-show="!loading"
      ref="chartRef"
      class="trend__chart h-[min(16rem,38vh)] w-full min-h-[200px] min-w-0 rounded-lg bg-[rgb(var(--yb-chart-panel))] p-0.5 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { BarChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { resolveCssRgbToken } from '@/utils/color-tokens'

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
const chartFontFamily = 'Segoe UI, PingFang SC, Microsoft YaHei, sans-serif'

const option = computed<EChartsCoreOption>(() => {
  const axisColor = '--yb-chart-axis'
  return {
    color: [
      resolveCssRgbToken('--yb-chart-trend-created', '111 140 255'),
      resolveCssRgbToken('--yb-chart-trend-completed', '142 226 127'),
      resolveCssRgbToken('--yb-chart-trend-due', '255 107 107'),
    ],
    textStyle: { fontFamily: chartFontFamily },
    grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis' as const,
      className: 'echarts-tooltip',
      backgroundColor: resolveCssRgbToken('--yb-chart-tooltip-bg', '18 20 28', 0.94),
      borderColor: resolveCssRgbToken('--yb-chart-tooltip-border', '255 255 255', 0.16),
      borderWidth: 1,
      padding: [10, 12],
      textStyle: {
        color: resolveCssRgbToken('--yb-chart-tooltip-text', '220 230 255'),
        fontFamily: chartFontFamily,
        fontSize: 12,
        fontWeight: 600,
      },
      extraCssText: `border-radius:12px;box-shadow:0 18px 50px -24px ${resolveCssRgbToken('--yb-black', '0 0 0', 0.95)};backdrop-filter:blur(18px);`,
    },
    legend: {
      data: ['新建', '完成', '当日截止'],
      type: 'scroll' as const,
      top: 0,
      left: 'center',
      textStyle: { color: resolveCssRgbToken('--yb-chart-legend-text', '170 181 204'), fontFamily: chartFontFamily, fontSize: 10 },
    },
    xAxis: {
      type: 'category' as const,
      data: props.labels,
      axisLine: { lineStyle: { color: resolveCssRgbToken(axisColor, '220 230 255', 0.42) } },
      axisLabel: { color: resolveCssRgbToken(axisColor, '220 230 255', 0.56), fontFamily: chartFontFamily, fontSize: 10, interval: 0 },
    },
    yAxis: {
      type: 'value' as const,
      minInterval: 1,
      splitLine: { lineStyle: { color: resolveCssRgbToken(axisColor, '220 230 255', 0.18) } },
      axisLabel: { color: resolveCssRgbToken(axisColor, '220 230 255', 0.62), fontFamily: chartFontFamily, fontSize: 10 },
    },
    series: [
      { name: '新建', type: 'bar' as const, data: props.created, barMaxWidth: 16 },
      { name: '完成', type: 'bar' as const, data: props.completed, barMaxWidth: 16 },
      { name: '当日截止', type: 'bar' as const, data: props.dueOnDay, barMaxWidth: 16 },
    ],
  }
})

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
