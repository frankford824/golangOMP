<script setup lang="ts">
import type { IWorkbookData, Plugin, PluginCtor, Univer } from '@univerjs/core'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CheckCircle2, LoaderCircle, RotateCcw, Table2, X } from 'lucide-vue-next'

import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import type { WorkbenchSpreadsheetAction, WorkbenchSpreadsheetActionPayload, WorkbenchSpreadsheetSource } from './types'
import { buildWorkbookSnapshot, extractRowsFromSnapshot, readAwSpreadsheetPalette } from './workbook'

const props = withDefaults(
  defineProps<{
    source: WorkbenchSpreadsheetSource
    height?: number
    closable?: boolean
  }>(),
  {
    height: 520,
    closable: true,
  },
)

const emit = defineEmits<{
  close: []
  action: [payload: WorkbenchSpreadsheetActionPayload]
}>()

const rootRef = ref<HTMLElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)
const activeSheetIndex = ref(0)
const loading = ref(false)
const ready = ref(false)
const loadError = ref('')

let univerInstance: Univer | null = null
let workbookInstance: { save?: () => IWorkbookData; getSnapshot?: () => IWorkbookData; dispose?: () => void } | null = null
let bootSeq = 0
let univerStylesInjected = false

type UniverPresetPlugin = PluginCtor<Plugin> | [PluginCtor<Plugin>, ConstructorParameters<PluginCtor<Plugin>>[0]]

const activeSheet = computed(() => props.source.sheets[Math.min(activeSheetIndex.value, Math.max(0, props.source.sheets.length - 1))])
const activeSource = computed<WorkbenchSpreadsheetSource>(() => ({
  ...props.source,
  sheets: activeSheet.value ? [activeSheet.value] : [],
}))
const fallbackColumns = computed(() =>
  (activeSheet.value?.columns ?? []).map((column) => ({
    key: column.key,
    label: column.label,
    width: column.width,
    align: column.align,
  })),
)
const fallbackRows = computed(() => (activeSheet.value?.rows ?? []) as Record<string, unknown>[])
const validationCount = computed(() => props.source.sheets.reduce((sum, sheet) => sum + (sheet.validations?.length ?? 0), 0))
const actionClass = (action: WorkbenchSpreadsheetAction) => ({
  'aw-primary-button': action.tone === 'success' || action.tone === 'money',
  'aw-secondary-button': action.tone !== 'success' && action.tone !== 'money',
  'aw-secondary-button--danger': action.tone === 'danger',
})

onMounted(() => {
  void bootUniver()
})

onBeforeUnmount(() => {
  disposeUniver()
})

watch(
  () => [props.source.id, props.source.revision, activeSheetIndex.value],
  () => {
    void bootUniver()
  },
)

watch(
  () => props.source.sheets.length,
  () => {
    if (activeSheetIndex.value >= props.source.sheets.length) activeSheetIndex.value = 0
  },
)

async function bootUniver() {
  const seq = ++bootSeq
  disposeUniver()
  await nextTick()
  if (!containerRef.value) return

  loading.value = true
  ready.value = false
  loadError.value = ''
  containerRef.value.replaceChildren()

  try {
    await ensureSegmenter()
    await ensureUniverStyles()
    const [{ LogLevel, LocaleType, Univer, mergeLocales }, { FUniver }, { UniverSheetsCorePreset }, locale] = await Promise.all([
      import('@univerjs/core'),
      import('@univerjs/core/facade'),
      import('@univerjs/preset-sheets-core'),
      import('@univerjs/preset-sheets-core/locales/zh-CN'),
    ])
    if (seq !== bootSeq || !containerRef.value) return

    const palette = readAwSpreadsheetPalette(rootRef.value)
    const snapshot = buildWorkbookSnapshot(activeSource.value, palette)
    const univer = new Univer({
      logLevel: LogLevel.WARN,
      locale: LocaleType.ZH_CN,
      locales: {
        [LocaleType.ZH_CN]: mergeLocales(locale.default),
      },
    })
    const preset = UniverSheetsCorePreset({
      container: containerRef.value,
      header: false,
      toolbar: false,
      ribbonType: 'collapsed',
      formulaBar: false,
      footer: false,
      contextMenu: false,
      disableAutoFocus: true,
      sheets: {
        maxAutoHeightCount: 1000,
        protectedRangeShadow: 'non-editable',
        disableForceStringAlert: true,
        disableForceStringMark: true,
      },
    })
    for (const pluginEntry of preset.plugins as UniverPresetPlugin[]) {
      const [plugin, options] = Array.isArray(pluginEntry) ? pluginEntry : [pluginEntry, undefined]
      univer.registerPlugin(plugin, options)
    }
    const univerAPI = FUniver.newAPI(univer)
    univerInstance = univer
    workbookInstance = univerAPI.createWorkbook(snapshot)
    ready.value = true
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '表格工作台加载失败'
  } finally {
    if (seq === bootSeq) loading.value = false
  }
}

async function ensureSegmenter() {
  if ('Segmenter' in Intl) return
  await import('@formatjs/intl-segmenter/polyfill.js')
}

async function ensureUniverStyles() {
  if (univerStylesInjected || document.querySelector('style[data-aw-univer-preset]')) {
    univerStylesInjected = true
    return
  }
  const css = await import('@univerjs/preset-sheets-core/lib/index.css?inline')
  const style = document.createElement('style')
  style.dataset.awUniverPreset = 'true'
  style.textContent = css.default
  document.head.appendChild(style)
  univerStylesInjected = true
}

function disposeUniver() {
  try {
    workbookInstance?.dispose?.()
  } catch {
    // Univer disposal should never block page navigation.
  }
  try {
    univerInstance?.dispose()
  } catch {
    // Univer disposal should never block page navigation.
  }
  workbookInstance = null
  univerInstance = null
  ready.value = false
}

function currentSnapshot() {
  return workbookInstance?.save?.() ?? workbookInstance?.getSnapshot?.() ?? null
}

function runAction(action: WorkbenchSpreadsheetAction) {
  emit('action', {
    action,
    sheets: extractRowsFromSnapshot(activeSource.value, currentSnapshot()),
  })
}
</script>

<template>
  <section ref="rootRef" class="aw-spreadsheet-workbench aw-token-scope" :data-mode="source.mode">
    <header class="aw-spreadsheet-workbench__head">
      <div>
        <p class="aw-eyebrow">表格工作台</p>
        <h3>{{ source.title }}</h3>
        <p v-if="source.description" class="aw-copy">{{ source.description }}</p>
      </div>
      <div class="aw-spreadsheet-workbench__status">
        <span class="aw-chip" :class="source.readonly ? 'aw-chip--neutral' : 'aw-chip--success'">
          {{ source.readonly ? '只读预览' : '可校对' }}
        </span>
        <span v-if="validationCount" class="aw-chip aw-chip--warn">{{ validationCount }} 个提示</span>
        <span v-else class="aw-chip aw-chip--success">
          <CheckCircle2 :size="13" aria-hidden="true" />
          预校验通过
        </span>
      </div>
      <button v-if="closable" class="aw-grid-button" type="button" aria-label="关闭表格工作台" @click="emit('close')">
        <X :size="15" aria-hidden="true" />
      </button>
    </header>

    <div class="aw-spreadsheet-workbench__toolbar">
      <div v-if="source.sheets.length > 1" class="aw-spreadsheet-tabs" aria-label="表格页">
        <button
          v-for="(sheet, index) in source.sheets"
          :key="sheet.id"
          class="aw-spreadsheet-tabs__tab"
          :class="{ 'is-active': index === activeSheetIndex }"
          type="button"
          @click="activeSheetIndex = index"
        >
          {{ sheet.name }}
        </button>
      </div>
      <div class="aw-spreadsheet-workbench__actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="bootUniver">
          <RotateCcw :size="15" aria-hidden="true" />
          重新载入
        </button>
        <button
          v-for="action in source.actions"
          :key="action.key"
          :class="actionClass(action)"
          type="button"
          :disabled="action.disabled"
          @click="runAction(action)"
        >
          {{ action.label }}
        </button>
      </div>
    </div>

    <div class="aw-spreadsheet-workbench__canvas-wrap" :style="{ '--aw-spreadsheet-height': `${height}px` }">
      <div
        ref="containerRef"
        class="aw-univer-theme"
        :class="{ 'is-ready': ready }"
        :style="{ minHeight: `${height}px` }"
        aria-label="Univer 表格画布"
      />
      <div v-if="loading" class="aw-spreadsheet-workbench__loading">
        <LoaderCircle :size="20" aria-hidden="true" />
        <span>正在载入表格引擎</span>
      </div>
      <div v-if="loadError" class="aw-spreadsheet-workbench__fallback">
        <div class="aw-inline-alert aw-inline-alert--error">Univer 加载失败：{{ loadError }}。下方保留同一份数据的工作台网格。</div>
        <WorkbenchDataGrid
          v-if="activeSheet"
          :columns="fallbackColumns"
          :rows="fallbackRows"
          :row-key="activeSheet.rowKey"
          :storage-key="`spreadsheet-fallback-${source.id}-${activeSheet.id}`"
          :height="Math.max(260, height - 120)"
          :row-height="34"
        />
      </div>
      <div v-else-if="!ready && !loading" class="aw-spreadsheet-workbench__fallback">
        <div class="aw-empty-state">
          <Table2 :size="24" aria-hidden="true" />
          <h3>表格画布待载入</h3>
          <p>如果浏览器阻止了表格引擎，仍可通过原页面表格继续操作。</p>
        </div>
      </div>
    </div>

    <footer v-if="activeSheet?.validations?.length" class="aw-spreadsheet-workbench__issues">
      <strong>当前表提示</strong>
      <ul>
        <li v-for="(issue, index) in activeSheet.validations.slice(0, 6)" :key="`${issue.rowKey}-${issue.columnKey}-${index}`">
          <span class="aw-chip" :class="`aw-chip--${issue.tone}`">{{ issue.tone }}</span>
          <span>{{ issue.message }}</span>
        </li>
      </ul>
    </footer>
  </section>
</template>
