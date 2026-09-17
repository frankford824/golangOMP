<template>
  <div ref="chartRef" class="design-trend-chart" role="img" aria-label="任务、完成和设计文件趋势" />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { DesignDepartmentTrendPoint } from '@/types/dashboard'
import { resolveCssRgbToken } from '@/utils/color-tokens'

use([BarChart, LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])
const props = defineProps<{ points: DesignDepartmentTrendPoint[] }>()
const chartRef = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null

function render() {
  if (!chartRef.value) return
  chart ??= init(chartRef.value)
  chart.setOption({
    color: [
      resolveCssRgbToken('--yb-brand', '37 99 235'),
      resolveCssRgbToken('--yb-success', '22 163 74'),
      resolveCssRgbToken('--yb-purple', '124 58 237'),
      resolveCssRgbToken('--yb-warning-accent', '245 158 11'),
    ],
    grid: { left: 16, right: 18, top: 42, bottom: 18, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
    tooltip: { trigger: 'axis' },
    legend: { data: ['任务量', '完成量', '设计文件量', 'SKU量'], top: 4 },
    xAxis: { type: 'category', data: props.points.map((item) => item.period), axisLabel: { hideOverlap: true } },
    yAxis: [{ type: 'value', minInterval: 1 }, { type: 'value', minInterval: 1, splitLine: { show: false } }],
    series: [
      { name: '任务量', type: 'bar', data: props.points.map((item) => item.task_count), barMaxWidth: 18 },
      { name: '完成量', type: 'bar', data: props.points.map((item) => item.completed_count), barMaxWidth: 18 },
      { name: '设计文件量', type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 6, data: props.points.map((item) => item.design_file_count) },
      { name: 'SKU量', type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: props.points.map((item) => item.sku_count) },
    ],
  }, { notMerge: true })
}

watch(() => props.points, render, { deep: true })
const resize = () => chart?.resize()
onMounted(() => { render(); window.addEventListener('resize', resize) })
onUnmounted(() => { window.removeEventListener('resize', resize); chart?.dispose(); chart = null })
</script>

<style scoped>
.design-trend-chart { width: 100%; height: 320px; min-height: 260px; }
@media (max-width: 720px) { .design-trend-chart { height: 280px; } }
</style>
