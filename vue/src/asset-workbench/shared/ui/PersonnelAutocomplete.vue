<script setup lang="ts">
import { computed, nextTick, ref, toRef, useId } from 'vue'

import type { PersonnelSuggestion, PersonnelSuggestionScope } from '@aw/shared/api/assetWorkbenchApi'
import { usePersonnelSuggestions } from '@aw/shared/composables/usePersonnelSuggestions'

const props = defineProps<{
  modelValue: string
  selected: boolean
  scope: PersonnelSuggestionScope
  label: string
  placeholder?: string
  status?: string
  hideLabel?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; selected: [person: PersonnelSuggestion]; submit: [] }>()
const open = ref(false)
const input = ref<HTMLInputElement | null>(null)
defineExpose({ focus: () => input.value?.focus() })
const listId = useId()
const menu = ref<HTMLElement | null>(null)
const expanded = computed(() => open.value && !props.selected && Boolean(props.modelValue.trim()))
const { items, total, error, loading, active, activeIndex, step } = usePersonnelSuggestions(props.scope, toRef(props, 'modelValue'), expanded, toRef(props, 'status'))
const value = computed({
  get: () => props.modelValue,
  set: (next: string) => {
    open.value = true
    emit('update:modelValue', next)
  },
})

function select(person: PersonnelSuggestion) {
  open.value = false
  emit('selected', person)
}

async function handleKey(event: KeyboardEvent) {
  if (event.isComposing) return
  if (event.key === 'Escape' && expanded.value) {
    event.preventDefault()
    open.value = false
  } else if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && !props.selected && value.value.trim()) {
    event.preventDefault()
    open.value = true
    step(event.key === 'ArrowDown' ? 1 : -1)
    await nextTick()
    menu.value?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
  } else if (event.key === 'Enter') {
    if (expanded.value && active.value) {
      event.preventDefault()
      select(active.value)
    } else {
      open.value = false
      emit('submit')
    }
  }
}
</script>

<template>
  <div class="aw-personnel-autocomplete">
    <label>
      <span v-if="!hideLabel">{{ label }}</span>
      <input
        ref="input"
        v-model="value"
        role="combobox"
        type="text"
        maxlength="80"
        :placeholder="placeholder || '姓名、拼音或编号'"
        :aria-label="label"
        autocomplete="off"
        aria-autocomplete="list"
        :aria-expanded="expanded"
        :aria-controls="expanded ? listId : undefined"
        :aria-activedescendant="expanded && active ? listId + '-' + active.user_id : undefined"
        @focus="open = true"
        @blur="open = false"
        @keydown="handleKey"
      />
    </label>
    <div v-if="expanded" class="aw-ledger-category__panel aw-personnel-autocomplete__menu">
      <p v-if="loading" class="aw-copy" role="status">正在查找人员…</p>
      <p v-else-if="error" class="aw-copy" role="status">{{ error }}</p>
      <p v-else-if="!items.length" class="aw-copy" role="status">没有匹配的人员</p>
      <div :id="listId" ref="menu" class="aw-personnel-picker__results" role="listbox" :aria-label="`${label}候选`">
        <button
          v-for="(person, index) in items"
          :id="listId + '-' + person.user_id"
          :key="person.user_id"
          class="aw-personnel-picker__result"
          type="button"
          role="option"
          tabindex="-1"
          :aria-selected="index === activeIndex"
          @mousedown.prevent
          @click="select(person)"
        >
          <span>
            <strong>{{ person.name }}</strong>
            <small>{{ person.username }} · 人员编号 {{ person.personnel_no || person.user_id }}</small>
          </span>
        </button>
      </div>
      <p v-if="total > items.length" class="aw-copy">共 {{ total }} 人，继续输入可缩小范围。</p>
    </div>
  </div>
</template>
