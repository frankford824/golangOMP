<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight, Minus, Plus } from 'lucide-vue-next'

interface WorkbenchDataGridColumn {
  key: string
  label: string
  width?: number
  align?: 'left' | 'right' | 'center'
}

interface GridColumnState {
  order: string[]
  widths: Record<string, number>
}

type GridRow = Record<string, unknown>
type FlatItem = { type: 'group'; key: string; label: string } | { type: 'row'; key: string; row: GridRow }

const props = withDefaults(
  defineProps<{
    columns: WorkbenchDataGridColumn[]
    rows: GridRow[]
    rowKey: string
    storageKey: string
    groupBy?: string
    groupLabels?: Record<string, string>
    height?: number
    rowHeight?: number
  }>(),
  {
    groupBy: '',
    groupLabels: () => ({}),
    height: 420,
    rowHeight: 36,
  },
)

const scrollTop = ref(0)
const columnState = ref<GridColumnState>({ order: [], widths: {} })

const storageName = computed(() => `aw:grid:${props.storageKey}`)
const orderedColumns = computed(() => {
  const byKey = new Map(props.columns.map((column) => [column.key, column]))
  const ordered = columnState.value.order
    .map((key) => byKey.get(key))
    .filter((column): column is WorkbenchDataGridColumn => Boolean(column))
  const missing = props.columns.filter((column) => !columnState.value.order.includes(column.key))
  return [...ordered, ...missing]
})
const gridTemplateColumns = computed(() =>
  orderedColumns.value
    .map((column) => `${Math.max(72, columnState.value.widths[column.key] ?? column.width ?? 120)}px`)
    .join(' '),
)
const flatItems = computed<FlatItem[]>(() => {
  if (!props.groupBy) {
    return props.rows.map((row) => ({ type: 'row', key: String(row[props.rowKey]), row }))
  }
  const groups = new Map<string, GridRow[]>()
  for (const row of props.rows) {
    const value = String(row[props.groupBy] ?? '未分组')
    const rows = groups.get(value) ?? []
    rows.push(row)
    groups.set(value, rows)
  }
  const items: FlatItem[] = []
  for (const [group, rows] of groups) {
    const label = props.groupLabels[group] ?? group
    items.push({ type: 'group', key: `group:${group}`, label: `${label} · ${rows.length}` })
    for (const row of rows) {
      items.push({ type: 'row', key: String(row[props.rowKey]), row })
    }
  }
  return items
})
const totalHeight = computed(() => flatItems.value.length * props.rowHeight)
const visibleRange = computed(() => {
  const overscan = 6
  const start = Math.max(0, Math.floor(scrollTop.value / props.rowHeight) - overscan)
  const count = Math.ceil(props.height / props.rowHeight) + overscan * 2
  const end = Math.min(flatItems.value.length, start + count)
  return { start, end }
})
const visibleItems = computed(() => flatItems.value.slice(visibleRange.value.start, visibleRange.value.end))
const topSpacerHeight = computed(() => visibleRange.value.start * props.rowHeight)
const bottomSpacerHeight = computed(() => Math.max(0, totalHeight.value - topSpacerHeight.value - visibleItems.value.length * props.rowHeight))

function loadColumnState() {
  try {
    const raw = localStorage.getItem(storageName.value)
    if (!raw) return
    const parsed = JSON.parse(raw) as Partial<GridColumnState>
    columnState.value = {
      order: Array.isArray(parsed.order) ? parsed.order.filter((key) => typeof key === 'string') : [],
      widths: parsed.widths && typeof parsed.widths === 'object' ? parsed.widths : {},
    }
  } catch {
    columnState.value = { order: [], widths: {} }
  }
}

function persistColumnState() {
  localStorage.setItem(storageName.value, JSON.stringify(columnState.value))
}

function updateScroll(event: Event) {
  scrollTop.value = (event.target as HTMLElement).scrollTop
}

function moveColumn(key: string, direction: -1 | 1) {
  const order = orderedColumns.value.map((column) => column.key)
  const index = order.indexOf(key)
  const target = index + direction
  if (index < 0 || target < 0 || target >= order.length) return
  const next = [...order]
  const [item] = next.splice(index, 1)
  next.splice(target, 0, item)
  columnState.value = { ...columnState.value, order: next }
  persistColumnState()
}

function resizeColumn(key: string, delta: number) {
  const column = props.columns.find((item) => item.key === key)
  const current = columnState.value.widths[key] ?? column?.width ?? 120
  columnState.value = {
    ...columnState.value,
    widths: {
      ...columnState.value.widths,
      [key]: Math.max(72, current + delta),
    },
  }
  persistColumnState()
}

function cellValue(row: GridRow, column: WorkbenchDataGridColumn) {
  return row[column.key]
}

onMounted(loadColumnState)

watch(
  () => props.storageKey,
  () => {
    columnState.value = { order: [], widths: {} }
    loadColumnState()
  },
)
</script>

<template>
  <div class="aw-data-grid" :style="{ '--aw-grid-columns': gridTemplateColumns }">
    <div class="aw-data-grid__head">
      <div
        v-for="column in orderedColumns"
        :key="column.key"
        class="aw-data-grid__th"
        :data-align="column.align || 'left'"
      >
        <span>{{ column.label }}</span>
        <span class="aw-data-grid__tools">
          <button type="button" title="左移列" @click="moveColumn(column.key, -1)">
            <ChevronLeft :size="12" aria-hidden="true" />
          </button>
          <button type="button" title="右移列" @click="moveColumn(column.key, 1)">
            <ChevronRight :size="12" aria-hidden="true" />
          </button>
          <button type="button" title="缩窄列" @click="resizeColumn(column.key, -16)">
            <Minus :size="12" aria-hidden="true" />
          </button>
          <button type="button" title="加宽列" @click="resizeColumn(column.key, 16)">
            <Plus :size="12" aria-hidden="true" />
          </button>
        </span>
      </div>
    </div>
    <div class="aw-data-grid__body" :style="{ maxHeight: `${height}px` }" @scroll="updateScroll">
      <div :style="{ height: `${topSpacerHeight}px` }" />
      <template v-for="item in visibleItems" :key="item.key">
        <div v-if="item.type === 'group'" class="aw-data-grid__group">
          {{ item.label }}
        </div>
        <div v-else class="aw-data-grid__row">
          <div
            v-for="column in orderedColumns"
            :key="column.key"
            class="aw-data-grid__td"
            :data-align="column.align || 'left'"
          >
            <slot name="cell" :row="item.row" :column="column" :value="cellValue(item.row, column)">
              {{ cellValue(item.row, column) }}
            </slot>
          </div>
        </div>
      </template>
      <div :style="{ height: `${bottomSpacerHeight}px` }" />
    </div>
  </div>
</template>
