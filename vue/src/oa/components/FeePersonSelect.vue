<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { NSelect } from 'naive-ui'
import { errorText } from '../api'
import { searchFeePeople, approverLabel, type FeePerson } from '../feePeople'

const props = defineProps<{ label: string; disabled?: boolean }>()
const person = defineModel<FeePerson>()
const people = ref<FeePerson[]>([])
const options = computed(() => people.value.map(item => ({ value: item.id, label: approverLabel(item) })))
const selected = computed(() => person.value?.id ?? null)
const busy = ref(false)
const nextAfterId = ref(0)
const initialized = ref(false)
const pageCursors = ref<number[]>([0])
const pageIndex = ref(0)
let searchQuery = ''
const error = ref('')
let timer: number | undefined
let request = 0

function search(query: string, afterId = 0, requestedPage = 0) {
  window.clearTimeout(timer)
  const currentRequest = ++request
  busy.value = true
  if (!afterId && requestedPage === 0 && query !== searchQuery) {
    people.value = []
    nextAfterId.value = 0
    pageCursors.value = [0]
    pageIndex.value = 0
    searchQuery = query
  }
  error.value = ''
  timer = window.setTimeout(async () => {
    try {
      const { items, next_after_id } = await searchFeePeople(query, [], afterId)
      if (currentRequest === request) {
        people.value = items ?? []
        pageIndex.value = requestedPage
        pageCursors.value[requestedPage] = afterId
        nextAfterId.value = next_after_id || 0
        initialized.value = true
      }
    } catch (cause) {
      if (currentRequest === request) error.value = errorText(cause)
    } finally {
      if (currentRequest === request) busy.value = false
    }
  }, 200)
}

function choose(id: number | null) {
  person.value = people.value.find(item => item.id === id)
}

onUnmounted(() => {
  request++
  window.clearTimeout(timer)
})
</script>

<template>
  <div class="fee-person-select">
    <span>{{ props.label }}</span>
    <NSelect
      :value="selected"
      :options="options"
      :fallback-option="(id: string | number) => ({ value: id, label: person ? approverLabel(person) : '当前所选人员' })"
      :input-props="{ 'aria-label': props.label }"
      :disabled="disabled"
      :loading="busy"
      placeholder="姓名 / 拼音 / 部门 / 工号 / 完整手机号"
      filterable
      remote
      clearable
      @update:show="show => { if (show && !initialized && !busy) search('') }"
      @search="search"
      @update:value="choose"
    >
      <template #empty>{{ busy ? '正在搜索…' : '没有匹配的在职人员' }}</template>
      <template v-if="nextAfterId || pageIndex" #action>
        <div class="oa-actions">
          <button type="button" :disabled="disabled || busy || !pageIndex" @click="search(searchQuery, pageCursors[pageIndex - 1], pageIndex - 1)">上一页候选</button>
          <small role="status">第 {{ pageIndex + 1 }} 页</small>
          <button type="button" :disabled="disabled || busy || !nextAfterId" @click="search(searchQuery, nextAfterId, pageIndex + 1)">下一页候选</button>
        </div>
      </template>
    </NSelect>
    <small v-if="error" class="oa-error" role="alert">{{ error }}</small>
  </div>
</template>

<style scoped>
.fee-person-select :deep(.n-base-selection-input__content) { font-size: 14px; }
#app .fee-person-select :deep(input.n-base-selection-input-tag__input),
#app .fee-person-select :deep(input.n-base-selection-input__input) {
  min-height: 0; padding: 0; border: 0; border-radius: 0; background: transparent;
}
.fee-person-select { display: grid; gap: 4px; min-width: 0; color: var(--oa-secondary); }
</style>
