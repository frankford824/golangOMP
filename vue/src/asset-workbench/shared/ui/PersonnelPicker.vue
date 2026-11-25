<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'

import type { PersonnelSuggestion } from '@aw/shared/api/assetWorkbenchApi'
import { usePersonnelSuggestions } from '@aw/shared/composables/usePersonnelSuggestions'
import { chipClass, profileStatusMeta, workerTypeMeta } from '@aw/shared/format/status'

const props = withDefaults(defineProps<{
  modelValue: number
  label: string
  hint?: string
  clearable?: boolean
  compact?: boolean
}>(), {
  hint: '按姓名、拼音或人员编号查找',
  clearable: false,
  compact: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: number]
  selected: [person: PersonnelSuggestion]
  cleared: []
}>()

const dialogOpen = ref(false)
const query = ref('')
const { items, total, error, loading, active, activeIndex, search, step } = usePersonnelSuggestions('payee', query, dialogOpen)
const selectedPerson = ref<PersonnelSuggestion | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const results = ref<HTMLElement | null>(null)
const listId = useId()

const selectedLabel = computed(() => {
  if (selectedPerson.value?.user_id === props.modelValue) {
    return `${selectedPerson.value.name} · 编号 ${selectedPerson.value.personnel_no || selectedPerson.value.user_id}`
  }
  return props.modelValue > 0 ? `人员编号 ${props.modelValue}` : '尚未选择人员'
})

watch(() => props.modelValue, (value) => {
  if (selectedPerson.value && selectedPerson.value.user_id !== value) selectedPerson.value = null
})

async function openPicker() {
  query.value = ''
  dialogOpen.value = true
  await nextTick()
  searchInput.value?.focus()
}

function closePicker() {
  dialogOpen.value = false
}

async function handleSearchKey(event: KeyboardEvent) {
  if (event.isComposing) return
  if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && items.value.length) {
    event.preventDefault()
    step(event.key === 'ArrowDown' ? 1 : -1)
    await nextTick()
    results.value?.querySelector('[data-highlighted="true"]')?.scrollIntoView({ block: 'nearest' })
  } else if (event.key === 'Enter') {
    event.preventDefault()
    if (active.value) selectPerson(active.value)
    else void search()
  }
}

function selectPerson(person: PersonnelSuggestion) {
  selectedPerson.value = person
  emit('update:modelValue', person.user_id)
  emit('selected', person)
  closePicker()
}

function clearSelection() {
  selectedPerson.value = null
  emit('update:modelValue', 0)
  emit('cleared')
}
</script>

<template>
  <div class="aw-personnel-picker" :class="{ 'aw-personnel-picker--compact': compact }">
    <span class="aw-personnel-picker__label">{{ label }}</span>
    <div class="aw-personnel-picker__field" :class="{ 'aw-personnel-picker__field--selected': modelValue > 0 }">
      <span>
        <strong>{{ selectedLabel }}</strong>
        <small>{{ hint }}</small>
      </span>
      <span class="aw-inline-actions aw-inline-actions--compact">
        <button v-if="clearable && modelValue > 0" class="aw-secondary-button" type="button" @click="clearSelection">清除</button>
        <button class="aw-secondary-button" type="button" @click="openPicker">{{ modelValue > 0 ? '重新选择' : '选择人员' }}</button>
      </span>
    </div>

    <div v-if="dialogOpen" class="aw-dialog-backdrop" role="presentation" @click.self="closePicker">
      <section class="aw-confirm-dialog aw-personnel-picker__dialog" role="dialog" aria-modal="true" :aria-labelledby="`personnel-picker-${label}`" @keydown.esc.stop="!$event.isComposing && closePicker()">
        <div>
          <p class="aw-eyebrow">人员查找</p>
          <h3 :id="`personnel-picker-${label}`">{{ label }}</h3>
          <p class="aw-copy">输入姓名片段、拼音或编号查找。请核对姓名和人员编号后选择。</p>
        </div>

        <div class="aw-personnel-picker__search">
          <input ref="searchInput" v-model="query" type="search" maxlength="80" placeholder="输入姓名、拼音或人员编号"
            aria-label="搜索人员" role="combobox" aria-autocomplete="list" :aria-expanded="items.length > 0"
            :aria-controls="items.length ? listId : undefined"
            :aria-activedescendant="active ? listId + '-' + active.user_id : undefined" @keydown="handleSearchKey" />
          <button class="aw-secondary-button" type="button" @click="search">查找</button>
        </div>

        <p v-if="loading" class="aw-copy" role="status">正在查找人员…</p>
        <p v-else-if="error" class="aw-inline-alert aw-inline-alert--warning" role="status">{{ error }}</p>
        <div v-if="items.length" :id="listId" ref="results" class="aw-personnel-picker__results" role="listbox" aria-label="人员搜索结果">
          <button
            v-for="(person, index) in items"
            :id="listId + '-' + person.user_id"
            :key="person.user_id"
            class="aw-personnel-picker__result"
            type="button"
            role="option"
            :aria-selected="person.user_id === modelValue"
            :data-highlighted="index === activeIndex || undefined"
            @click="selectPerson(person)"
          >
            <span>
              <strong>{{ person.name }}</strong>
              <small>人员编号 {{ person.personnel_no || person.user_id }} · {{ workerTypeMeta(person.worker_type).label }} · {{ person.job_grade || '未定级' }}</small>
              <small v-if="person.username && person.username !== person.name">{{ person.username }}</small>
            </span>
            <span :class="chipClass(profileStatusMeta(person.status).tone)">{{ profileStatusMeta(person.status).label }}</span>
          </button>
        </div>
        <p v-if="!loading && !error && !items.length" class="aw-copy" role="status">没有找到匹配人员，请检查姓名或编号。</p>
        <p v-if="total > items.length" class="aw-copy">共 {{ total }} 人，继续输入可缩小范围。</p>

        <div class="aw-inline-actions">
          <button class="aw-secondary-button" type="button" @click="closePicker">取消</button>
        </div>
      </section>
    </div>
  </div>
</template>
