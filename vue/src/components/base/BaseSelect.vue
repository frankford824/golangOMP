<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" class="text-sm font-medium text-slate-600">
      {{ label }}
    </label>

    <div ref="rootEl" class="relative">
      <!-- 外层容器参与定位宽度；避免 button 嵌套 -->
      <div
        ref="triggerEl"
        class="flex h-11 w-full items-center gap-1 rounded-xl border border-stone-200 bg-stone-50/80 px-2 text-sm text-stone-800 transition focus-within:border-stone-400 focus-within:ring-1 focus-within:ring-stone-300 hover:bg-stone-50"
        :class="{ 'cursor-not-allowed bg-stone-100 text-stone-500': disabled }"
      >
        <button
          type="button"
          class="flex min-w-0 flex-1 items-center gap-2 rounded-lg py-0 pl-1 pr-0 text-left outline-none disabled:cursor-not-allowed disabled:text-stone-500"
          :disabled="disabled"
          @click="toggleOpen"
        >
          <span
            class="min-w-0 flex-1 truncate"
            :class="{ 'text-slate-500': !selectedLabel && placeholder }"
          >
            {{ selectedLabel || placeholder || '请选择' }}
          </span>
          <span
            class="inline-flex h-4 w-4 shrink-0 items-center justify-center text-slate-400 transition-transform duration-150"
            :class="{ 'rotate-180': open }"
          >
            ▾
          </span>
        </button>
        <button
          v-if="showClear"
          type="button"
          class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-stone-400 transition hover:bg-stone-200/80 hover:text-stone-600"
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
          class="fixed overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-md"
          :class="filterable ? 'flex max-h-72 flex-col' : 'py-1'"
          :style="panelStyle"
        >
          <template v-if="filterable">
            <div class="shrink-0 border-b border-stone-200 px-2 py-1.5">
              <input
                ref="filterInputRef"
                v-model="filterQuery"
                type="search"
                autocomplete="off"
                :placeholder="filterPlaceholder"
                class="w-full rounded-lg border border-stone-200 bg-stone-50/80 px-2 py-1.5 text-sm text-stone-800 outline-none placeholder:text-stone-400 focus:border-stone-400 focus:ring-1 focus:ring-stone-300"
                @click.stop
                @keydown.stop.escape="close"
              />
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto py-0.5">
              <button
                v-for="opt in displayedOptions"
                :key="String(opt.value)"
                type="button"
                class="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm text-stone-700 transition-colors"
                :class="{
                  'bg-stone-100 text-stone-900 font-medium': isSelected(opt.value),
                  'hover:bg-stone-50': !isSelected(opt.value),
                }"
                @click="selectOption(opt.value)"
              >
                <span class="truncate">
                  {{ opt.label }}
                </span>
              </button>
              <p
                v-if="!displayedOptions.length"
                class="px-3 py-2 text-center text-xs text-slate-500"
              >
                无匹配结果
              </p>
            </div>
          </template>
          <template v-else>
            <div class="py-1">
              <button
                v-for="opt in options"
                :key="String(opt.value)"
                type="button"
                class="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm text-stone-700 transition-colors"
                :class="{
                  'bg-stone-100 text-stone-900 font-medium': isSelected(opt.value),
                  'hover:bg-stone-50': !isSelected(opt.value),
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

    <p v-if="hint" class="text-xs text-slate-400">
      {{ hint }}
    </p>
    <p v-if="error" class="text-xs text-red-600">
      {{ error }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

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
  }>(),
  {
    disabled: false,
    clearable: false,
    clearValue: '',
    filterable: false,
    filterPlaceholder: '输入关键字筛选',
  },
)

const emit = defineEmits<{
  'update:modelValue': [string | number]
  'filter-change': [string]
}>()

const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLElement | null>(null)
const panelEl = ref<HTMLElement | null>(null)
const filterInputRef = ref<HTMLInputElement | null>(null)
const filterQuery = ref('')
const panelStyle = ref<Record<string, string>>({})

const displayedOptions = computed(() => {
  if (!props.filterable) return props.options
  const q = filterQuery.value.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter((opt) => {
    const label = String(opt.label).toLowerCase()
    const val = String(opt.value).toLowerCase()
    return label.includes(q) || val.includes(q)
  })
})

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
  updatePanelPosition()
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
})

watch(open, async (value) => {
  if (!value) {
    filterQuery.value = ''
    return
  }
  filterQuery.value = ''
  await nextTick()
  updatePanelPosition()
  if (props.filterable) {
    filterInputRef.value?.focus()
  }
})

watch(filterQuery, () => {
  if (!open.value || !props.filterable) return
  emit('filter-change', filterQuery.value)
  void nextTick(() => {
    updatePanelPosition()
  })
})
</script>

<style scoped>
/* Light admin select dropdown skin. Style-only. */
.fixed {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: 0 10px 24px -8px rgba(15, 23, 42, 0.12) !important;
}

.fixed :deep(input) {
  border-color: #e5e7eb !important;
  background: #f9fafb !important;
  color: #111827 !important;
}

.fixed :deep(input::placeholder) {
  color: #9ca3af !important;
}

.fixed button {
  color: #374151 !important;
}

.fixed button:hover,
.fixed .bg-stone-100 {
  background: #f3f4f6 !important;
  color: #111827 !important;
}

.fixed .bg-stone-100 {
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.fixed .border-b {
  border-color: #e5e7eb !important;
}

.cursor-not-allowed {
  opacity: 1 !important;
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
