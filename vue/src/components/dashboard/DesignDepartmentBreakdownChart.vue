<template>
  <div ref="chartRef" class="design-breakdown-chart" :aria-label="title" role="img" />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { DesignDepartmentDashboardBreakdown } from '@/types/dashboard'
import { resolveCssRgbToken } from '@/utils/color-tokens'

use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])
const props = defineProps<{ title: string; items: DesignDepartmentDashboardBreakdown[]; metric?: 'task_count' | 'design_file_count' }>()
const chartRef = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null
function render() {
  if (!chartRef.value) return
  chart ??= init(chartRef.value)
  const metric = props.metric ?? 'task_count'
  const items = props.items.slice(0, 10).reverse()
  chart.setOption({
    grid: { left: 12, right: 34, top: 10, bottom: 10, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, formatter: (params: unknown) => {
      const list = params as Array<{ name: string; value: number; dataIndex: number }>
      const point = list[0]; const source = items[point?.dataIndex ?? 0]
      return `${source?.label ?? ''}<br/>任务 ${source?.task_count ?? 0}<br/>设计文件 ${source?.design_file_count ?? 0}<br/>占比 ${source?.share ?? 0}%`
    } },
    xAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: resolveCssRgbToken('--yb-overlay-slate', '148 163 184', 0.18) } } },
    yAxis: { type: 'category', data: items.map((item) => item.label), axisLabel: { width: 92, overflow: 'truncate' } },
    series: [{ type: 'bar', data: items.map((item) => item[metric]), barMaxWidth: 18, itemStyle: { color: resolveCssRgbToken('--yb-brand-bright', '59 130 246'), borderRadius: [0, 5, 5, 0] }, label: { show: true, position: 'right' } }],
  }, { notMerge: true })
}
watch(() => [props.items, props.metric], render, { deep: true })
const resize = () => chart?.resize()
onMounted(() => { render(); window.addEventListener('resize', resize) })
onUnmounted(() => { window.removeEventListener('resize', resize); chart?.dispose(); chart = null })
</script>

<style scoped>.design-breakdown-chart { width: 100%; height: 270px; min-height: 220px; }</style>
