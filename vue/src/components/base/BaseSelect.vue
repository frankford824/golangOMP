<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :id="labelId" class="text-sm font-medium text-[rgb(var(--yb-text-muted-strong))]">
      {{ label }}
    </label>

    <div ref="rootEl" class="relative">
      <!-- 外层容器参与定位宽度；避免 button 嵌套 -->
      <div
        ref="triggerEl"
        class="flex h-11 w-full items-center gap-1 rounded-xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft)_/_0.8)] px-2 text-sm text-[rgb(var(--yb-text))] transition focus-within:border-[rgb(var(--yb-brand-border-strong))] focus-within:ring-1 focus-within:ring-[rgb(var(--yb-brand-accent)_/_0.35)] hover:bg-[rgb(var(--yb-surface))]"
        :class="{ 'cursor-not-allowed bg-[rgb(var(--yb-surface-muted))] text-[rgb(var(--yb-text-faint))]': disabled }"
      >
        <button
          type="button"
          class="base-select-trigger-button flex min-w-0 flex-1 items-center gap-2 rounded-lg py-0 pl-1 pr-0 text-left outline-none disabled:cursor-not-allowed disabled:text-[rgb(var(--yb-text-faint))]"
          :class="{ 'base-select-trigger-button-open': open }"
          :disabled="disabled"
          aria-haspopup="listbox"
          :aria-expanded="open"
          :aria-labelledby="triggerLabelledBy"
          :aria-invalid="error ? 'true' : undefined"
          :aria-describedby="describedBy"
          @click="toggleOpen"
        >
          <span
            :id="valueId"
            class="min-w-0 flex-1 truncate"
            :class="{ 'text-[rgb(var(--yb-text-placeholder))]': !selectedLabel && placeholder }"
          >
            {{ selectedLabel || placeholder || '请选择' }}
          </span>
          <span
            class="inline-flex h-4 w-4 shrink-0 items-center justify-center text-[rgb(var(--yb-text-faint))] transition-transform duration-150"
            :class="{ 'rotate-180': open }"
          >
            ▾
          </span>
        </button>
        <button
          v-if="showClear"
          type="button"
          class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-[rgb(var(--yb-text-faint))] transition hover:bg-[rgb(var(--yb-surface-muted))] hover:text-[rgb(var(--yb-text-muted-strong))]"
          aria-label="清除选择"
          @click.stop="clearSelection"
        >
          <span class="text-lg leading-none">×</span>
        </button>
      </div>
    </div>
    <!-- v4.2 修复：老板要求 + 下拉层被任务卡片 stacking context 压住，改为 Teleport 到 body -->
    <Teleport to="body">
      <transition name="fade-scale">
        <div
          v-if="open"
          ref="panelEl"
          class="base-select-panel fixed overflow-hidden rounded-2xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] text-[rgb(var(--yb-text-body))] shadow-md"
          :class="filterable ? 'flex max-h-72 flex-col' : 'py-1'"
          :style="panelStyle"
          role="listbox"
          aria-label="选择选项"
        >
          <template v-if="filterable">
            <div class="shrink-0 border-b border-[rgb(var(--yb-border))] px-2 py-1.5">
              <input
                ref="filterInputRef"
                v-model="filterQuery"
                type="search"
                autocomplete="off"
                :placeholder="filterPlaceholder"
                class="w-full rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft)_/_0.8)] px-2 py-1.5 text-sm text-[rgb(var(--yb-text))] outline-none placeholder:text-[rgb(var(--yb-text-placeholder))] focus:border-[rgb(var(--yb-brand-border-strong))] focus:ring-1 focus:ring-[rgb(var(--yb-brand-accent)_/_0.35)]"
                @click.stop
                @keydown.stop.escape="close"
              />
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto py-0.5">
              <button
                v-for="opt in displayedOptions"
                :key="String(opt.value)"
                type="button"
                class="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm text-[rgb(var(--yb-text-body))] transition-colors"
                :class="{
                  'bg-[rgb(var(--yb-brand-soft))] text-[rgb(var(--yb-brand-strong))] font-medium': isSelected(opt.value),
                  'hover:bg-[rgb(var(--yb-surface-muted))]': !isSelected(opt.value),
                }"
                @click="selectOption(opt.value)"
              >
                <span class="truncate">
                  {{ opt.label }}
                </span>
              </button>
              <p
                v-if="!displayedOptions.length"
                class="px-3 py-2 text-center text-xs text-[rgb(var(--yb-text-muted))]"
              >
                无匹配结果
              </p>
              <p
                v-else-if="hiddenOptionCount > 0"
                class="px-3 py-2 text-center text-xs text-[rgb(var(--yb-text-muted))]"
              >
                还有 {{ hiddenOptionCount }} 项，请输入关键字缩小范围
              </p>
            </div>
          </template>
          <template v-else>
            <div class="py-1">
              <button
                v-for="opt in options"
                :key="String(opt.value)"
                type="button"
                class="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm text-[rgb(var(--yb-text-body))] transition-colors"
                :class="{
                  'bg-[rgb(var(--yb-brand-soft))] text-[rgb(var(--yb-brand-strong))] font-medium': isSelected(opt.value),
                  'hover:bg-[rgb(var(--yb-surface-muted))]': !isSelected(opt.value),
                }"
                @click="selectOption(opt.value)"
              >
                <span class="truncate">
                  {{ opt.label }}
                </span>
              </button>
            </div>
          </template>
        </div>
      </transition>
    </Teleport>

    <p v-if="hint" :id="hintId" class="text-xs text-[rgb(var(--yb-text-faint))]">
      {{ hint }}
    </p>
    <p v-if="error" :id="errorId" class="text-xs text-[rgb(var(--yb-danger))]">
      {{ error }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'

export interface BaseSelectOption {
  value: string | number
  label: string
}

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    label?: string
    placeholder?: string
    disabled?: boolean
    clearable?: boolean
    /** Emitted when clearing; default '' matches task filters */
    clearValue?: string | number
    options: BaseSelectOption[]
    hint?: string
    error?: string
    /**
     * 在已有 options 上本地按文案 / value 过滤（适合已全量加载的人员下拉）。
     * 面板内顶部出现搜索框，下方为可滚动结果列表。
     */
    filterable?: boolean
    filterPlaceholder?: string
    maxDisplayedOptions?: number
  }>(),
  {
    disabled: false,
    clearable: false,
    clearValue: '',
    filterable: false,
    filterPlaceholder: '输入关键字筛选',
    maxDisplayedOptions: 160,
  },
)

const emit = defineEmits<{
  'update:modelValue': [string | number]
  'filter-change': [string]
}>()

const uid = useId()
const labelId = `${uid}-label`
const valueId = `${uid}-value`
const hintId = `${uid}-hint`
const errorId = `${uid}-error`
const triggerLabelledBy = computed(() =>
  props.label ? `${labelId} ${valueId}` : valueId,
)
const describedBy = computed(() => {
  const ids: string[] = []
  if (props.error) ids.push(errorId)
  if (props.hint) ids.push(hintId)
  return ids.length ? ids.join(' ') : undefined
})

const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLElement | null>(null)
const panelEl = ref<HTMLElement | null>(null)
const filterInputRef = ref<HTMLInputElement | null>(null)
const filterQuery = ref('')
const panelStyle = ref<Record<string, string>>({})
let positionFrame: number | null = null

const filteredOptions = computed(() => {
  if (!props.filterable) return props.options
  const q = filterQuery.value.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter((opt) => {
    const label = String(opt.label).toLowerCase()
    const val = String(opt.value).toLowerCase()
    return label.includes(q) || val.includes(q)
  })
})

const displayedOptions = computed(() => {
  if (!props.filterable) return props.options
  const max = Math.max(20, props.maxDisplayedOptions)
  return filteredOptions.value.slice(0, max)
})

const hiddenOptionCount = computed(() =>
  props.filterable ? Math.max(0, filteredOptions.value.length - displayedOptions.value.length) : 0,
)

const selectedLabel = computed(() => {
  const match = props.options.find((opt) => opt.value === props.modelValue)
  return match?.label ?? ''
})

const showClear = computed(
  () =>
    props.clearable &&
    !props.disabled &&
    props.modelValue !== undefined &&
    props.modelValue !== null &&
    props.modelValue !== props.clearValue,
)

function clearSelection() {
  emit('update:modelValue', props.clearValue)
  close()
}

function isSelected(value: string | number) {
  return value === props.modelValue
}

function toggleOpen() {
  if (props.disabled) return
  open.value = !open.value
}

function close() {
  open.value = false
  filterQuery.value = ''
}

function updatePanelPosition() {
  // v4.2 修复：老板要求 + Teleport 后用 fixed 跟随触发器，规避父容器 overflow / transform 影响
  const trigger = triggerEl.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const panelHeight = panelEl.value?.offsetHeight ?? 0
  const viewportHeight = window.innerHeight
  const viewportWidth = window.innerWidth
  const gap = 8
  const placeAbove = panelHeight > 0 && rect.bottom + gap + panelHeight > viewportHeight && rect.top - gap - panelHeight >= 8
  const top = placeAbove
    ? Math.max(8, rect.top - panelHeight - gap)
    : Math.min(viewportHeight - panelHeight - 8, rect.bottom + gap)
  const width = Math.min(Math.max(rect.width, 180), Math.max(180, viewportWidth - 16))
  const left = Math.min(Math.max(8, rect.left), Math.max(8, viewportWidth - width - 8))

  panelStyle.value = {
    top: `${Math.max(8, top)}px`,
    left: `${left}px`,
    width: `${width}px`,
    maxWidth: 'calc(100vw - 16px)',
    // Must stay above BaseModal overlay (7100) after visual upgrade.
    zIndex: '7200',
  }
}

function schedulePanelPositionUpdate() {
  if (positionFrame != null) return
  positionFrame = window.requestAnimationFrame(() => {
    positionFrame = null
    updatePanelPosition()
  })
}

function selectOption(value: string | number) {
  emit('update:modelValue', value)
  close()
}

function handleClickOutside(e: MouseEvent) {
  if (!rootEl.value) return
  if (rootEl.value.contains(e.target as Node)) return
  if (panelEl.value?.contains(e.target as Node)) return
  if (open.value) {
    close()
  }
}

function handleViewportChange() {
  if (!open.value) return
  schedulePanelPositionUpdate()
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('resize', handleViewportChange)
  window.addEventListener('scroll', handleViewportChange, true)
  window.addEventListener('layout-change', handleViewportChange)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', handleViewportChange)
  window.removeEventListener('scroll', handleViewportChange, true)
  window.removeEventListener('layout-change', handleViewportChange)
  if (positionFrame != null) {
    window.cancelAnimationFrame(positionFrame)
    positionFrame = null
  }
})

watch(open, async (value) => {
  if (!value) {
    filterQuery.value = ''
    return
  }
  filterQuery.value = ''
  await nextTick()
  schedulePanelPositionUpdate()
  if (props.filterable) {
    filterInputRef.value?.focus()
  }
})

watch(filterQuery, () => {
  if (!open.value || !props.filterable) return
  emit('filter-change', filterQuery.value)
  void nextTick(() => {
    schedulePanelPositionUpdate()
  })
})
</script>

<style scoped>
/* Light admin select dropdown skin. Style-only. */
.base-select-panel.fixed {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  box-shadow: 0 10px 24px -8px rgb(var(--yb-shadow) / 0.12);
}

.base-select-panel :deep(input) {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.base-select-panel :deep(input::placeholder) {
  color: rgb(var(--yb-text-faint));
}

.base-select-panel button {
  color: rgb(var(--yb-text-body));
}

.base-select-panel button:hover {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text));
}

.base-select-panel .border-b {
  border-color: rgb(var(--yb-border));
}

.base-select-trigger-button:hover,
.base-select-trigger-button-open {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text));
}

.relative .cursor-not-allowed {
  opacity: 1;
}
</style>

<style scoped>
.fade-scale-enter-active,
.fade-scale-leave-active {
  transition: opacity 0.12s ease-out, transform 0.12s ease-out;
}
.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: translateY(-2px) scale(0.98);
}
</style>
