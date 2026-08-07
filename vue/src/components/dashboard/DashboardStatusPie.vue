<template>
  <div class="pie w-full min-w-0">
    <div
      v-if="loading"
      class="pie__skeleton h-[min(16rem,38vh)] w-full min-h-[200px] rounded-lg bg-[rgb(var(--yb-surface-muted)/0.8)] sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
      role="img"
      aria-label="分布图加载中"
    />
    <div
      v-show="!loading"
      ref="chartRef"
      class="pie__chart h-[min(16rem,38vh)] w-full min-h-[200px] min-w-0 rounded-lg bg-[rgb(var(--yb-surface-subtle))] p-0.5 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { PieChart } from 'echarts/charts'
import { LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { resolveCssRgbToken } from '@/utils/color-tokens'

use([PieChart, LegendComponent, TitleComponent, TooltipComponent, CanvasRenderer])

const props = withDefaults(
  defineProps<{
    series: { name: string; value: number }[]
    loading?: boolean
  }>(),
  { loading: false }
)

const chartRef = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null
const chartFontFamily = 'Segoe UI, PingFang SC, Microsoft YaHei, sans-serif'

const option = computed<EChartsCoreOption>(() => {
  const data = props.series.filter((d) => d.value > 0)
  const textPlaceholder = resolveCssRgbToken('--yb-text-placeholder', '148 163 184')
  const textSoft = resolveCssRgbToken('--yb-text-soft', '71 85 105')
  const panel = resolveCssRgbToken('--yb-surface-subtle', '248 250 252')
  if (!data.length) {
    return {
      textStyle: { fontFamily: chartFontFamily },
      title: {
        text: '暂无数据',
        left: 'center',
        top: 'middle',
        textStyle: { color: textPlaceholder, fontFamily: chartFontFamily, fontSize: 13, fontWeight: 400 },
      },
    }
  }
  return {
    color: [
      resolveCssRgbToken('--yb-chart-pie-blue', '84 112 198'),
      resolveCssRgbToken('--yb-chart-pie-green', '145 204 117'),
      resolveCssRgbToken('--yb-chart-pie-yellow', '250 200 88'),
      resolveCssRgbToken('--yb-chart-pie-red', '238 102 102'),
      resolveCssRgbToken('--yb-chart-pie-cyan', '115 192 222'),
    ],
    textStyle: { fontFamily: chartFontFamily },
    tooltip: {
      trigger: 'item' as const,
      textStyle: { fontFamily: chartFontFamily },
    },
    legend: {
      type: 'scroll' as const,
      orient: 'horizontal' as const,
      left: 'center',
      bottom: 0,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: textSoft, fontFamily: chartFontFamily, fontSize: 10 },
    },
    series: [
      {
        name: '状态',
        type: 'pie' as const,
        radius: ['38%', '62%'],
        center: ['50%', '46%'],
        avoidLabelOverlap: true,
        itemStyle: { borderRadius: 2, borderColor: panel, borderWidth: 2 },
        label: { show: false },
        data,
      },
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
