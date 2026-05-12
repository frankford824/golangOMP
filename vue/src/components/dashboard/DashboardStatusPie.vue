<template>
  <div class="pie w-full min-w-0">
    <div
      v-if="loading"
      class="pie__skeleton h-[min(16rem,38vh)] w-full min-h-[200px] rounded-lg bg-slate-100/80 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
      role="img"
      aria-label="分布图加载中"
    />
    <div
      v-show="!loading"
      ref="chartRef"
      class="pie__chart h-[min(16rem,38vh)] w-full min-h-[200px] min-w-0 rounded-lg bg-[#f8fafc] p-0.5 sm:h-[260px] sm:min-h-[240px] lg:h-[300px] lg:min-h-[280px]"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { PieChart } from 'echarts/charts'
import { LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

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

const option = computed<EChartsCoreOption>(() => {
  const data = props.series.filter((d) => d.value > 0)
  if (!data.length) {
    return {
      title: {
        text: '暂无数据',
        left: 'center',
        top: 'middle',
        textStyle: { color: '#94a3b8', fontSize: 13, fontWeight: 400 },
      },
    }
  }
  return {
    color: ['#5470C6', '#91CC75', '#FAC858', '#EE6666', '#73C0DE'],
    tooltip: { trigger: 'item' as const },
    legend: {
      type: 'scroll' as const,
      orient: 'horizontal' as const,
      left: 'center',
      bottom: 0,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: '#475569', fontSize: 10 },
    },
    series: [
      {
        name: '状态',
        type: 'pie' as const,
        radius: ['38%', '62%'],
        center: ['50%', '46%'],
        avoidLabelOverlap: true,
        itemStyle: { borderRadius: 2, borderColor: '#f8fafc', borderWidth: 2 },
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
