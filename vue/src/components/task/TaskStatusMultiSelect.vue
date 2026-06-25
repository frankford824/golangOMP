<template>
  <div ref="rootEl" class="relative w-full">
    <div
      ref="triggerEl"
      class="flex h-11 w-full items-center gap-1 rounded-lg border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft))] px-2 text-sm text-[rgb(var(--yb-text-body))] shadow-sm transition focus-within:border-[rgb(var(--yb-brand-border-strong))] focus-within:ring-2 focus-within:ring-[rgb(var(--yb-brand)/0.12)] hover:border-[rgb(var(--yb-border-strong))] hover:bg-[rgb(var(--yb-surface-muted))]"
    >
      <button
        type="button"
        class="flex min-w-0 flex-1 items-center gap-2 rounded-md py-0 pl-1 pr-0 text-left outline-none"
        @click.stop="toggleOpen"
      >
        <span
          class="min-w-0 flex-1 truncate"
          :class="{ 'text-[rgb(var(--yb-text-muted))]': !internalValue.length }"
        >
          {{ displayLabel }}
        </span>
        <span
          class="inline-flex h-4 w-4 shrink-0 items-center justify-center text-[rgb(var(--yb-text-faint))] transition-transform duration-150"
          :class="{ 'rotate-180': open }"
        >
          ▾
        </span>
      </button>
      <button
        v-if="internalValue.length"
        type="button"
        class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-[rgb(var(--yb-text-faint))] transition hover:bg-[rgb(var(--yb-border-slate)/0.8)] hover:text-[rgb(var(--yb-text-secondary))]"
        aria-label="清除状态筛选"
        @click.stop="clearAll"
      >
        <span class="text-lg leading-none">×</span>
      </button>
    </div>
  </div>
  <!-- v4.2 修复：老板要求 + 状态筛选下拉被卡片遮挡，改为 Teleport 到 body -->
  <Teleport to="body">
    <div
      v-if="open"
      ref="panelEl"
      class="fixed rounded-2xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] py-1 shadow-lg ring-1 ring-[rgb(var(--yb-border-quiet)/0.6)] overflow-hidden"
      :style="panelStyle"
    >
      <div class="mb-1 flex items-center justify-between px-3">
        <span class="text-[11px] font-medium text-[rgb(var(--yb-text-muted))]">任务状态</span>
        <button
          type="button"
          class="text-[11px] text-[rgb(var(--yb-text-faint))] hover:text-[rgb(var(--yb-text-secondary))]"
          @click="clearAll"
        >
          清空
        </button>
      </div>
      <div class="max-h-56 space-y-1 overflow-auto py-1 pr-1 text-[11px] text-[rgb(var(--yb-text-body))]">
        <label
          v-for="opt in options"
          :key="opt.value"
          class="flex cursor-pointer items-center gap-1 px-3 py-1.5 rounded-md transition-colors hover:bg-[rgb(var(--yb-surface-muted))]"
        >
          <input
            type="checkbox"
            :value="opt.value"
            class="h-3 w-3 rounded border-[rgb(var(--yb-border-strong))] text-[rgb(var(--yb-text))] focus:ring-[rgb(var(--yb-brand)/0.2)]"
            :checked="internalValue.includes(opt.value)"
            @change="onToggle(opt.value)"
          />
          <span>{{ opt.label }}</span>
        </label>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import type { LegacyTaskStatus as TaskStatus } from '@/domain/types/task'

interface Option {
  value: TaskStatus
  label: string
}

const props = defineProps<{
  modelValue: TaskStatus[]
  options: Option[]
}>()

const emit = defineEmits<{
  'update:modelValue': [TaskStatus[]]
}>()

const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLElement | null>(null)
const panelEl = ref<HTMLElement | null>(null)
const open = ref(false)
const internalValue = ref<TaskStatus[]>([...props.modelValue])
const panelStyle = ref<Record<string, string>>({})

watch(
  () => props.modelValue,
  (v) => {
    internalValue.value = [...v]
  },
  { deep: true },
)

const displayLabel = computed(() => {
  if (!internalValue.value.length) return '全部状态'
  if (internalValue.value.length <= 3) {
    const labels = props.options
      .filter((o) => internalValue.value.includes(o.value))
      .map((o) => o.label)
    return labels.join('、')
  }
  return `已选 ${internalValue.value.length} 项`
})

function toggleOpen() {
  open.value = !open.value
}

function updatePanelPosition() {
  // v4.2 修复：老板要求 + 固定定位跟随触发器，避免 filter 容器和卡片层级冲突
  const trigger = triggerEl.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const panelHeight = panelEl.value?.offsetHeight ?? 0
  const viewportHeight = window.innerHeight
  const gap = 8
  const placeAbove = panelHeight > 0 && rect.bottom + gap + panelHeight > viewportHeight && rect.top - gap - panelHeight >= 8
  const top = placeAbove
    ? Math.max(8, rect.top - panelHeight - gap)
    : Math.min(viewportHeight - panelHeight - 8, rect.bottom + gap)

  panelStyle.value = {
    top: `${Math.max(8, top)}px`,
    left: `${Math.max(8, rect.left)}px`,
    width: `${rect.width}px`,
    zIndex: '4000',
  }
}

function handleDocumentClick(event: MouseEvent) {
  if (!open.value) return
  const el = rootEl.value
  if (!el) return
  const target = event.target as Node
  if (!el.contains(target) && !panelEl.value?.contains(target)) {
    open.value = false
  }
}

function handleViewportChange() {
  if (!open.value) return
  updatePanelPosition()
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  window.addEventListener('resize', handleViewportChange)
  window.addEventListener('scroll', handleViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
  window.removeEventListener('resize', handleViewportChange)
  window.removeEventListener('scroll', handleViewportChange, true)
})

watch(open, async (value) => {
  if (!value) return
  await nextTick()
  updatePanelPosition()
})

function onToggle(status: TaskStatus) {
  const set = new Set(internalValue.value)
  if (set.has(status)) {
    set.delete(status)
  } else {
    set.add(status)
  }
  const next = Array.from(set)
  internalValue.value = next
  emit('update:modelValue', next)
}

function clearAll() {
  internalValue.value = []
  emit('update:modelValue', [])
}
</script>

