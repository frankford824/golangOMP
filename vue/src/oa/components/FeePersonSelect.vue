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
const error = ref('')
let timer: number | undefined
let request = 0

function search(query: string) {
  window.clearTimeout(timer)
  const currentRequest = ++request
  busy.value = true
  people.value = []
  error.value = ''
  timer = window.setTimeout(async () => {
    try {
      const { items } = await searchFeePeople(query)
      if (currentRequest === request) people.value = items ?? []
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
      @update:show="show => { if (show) search('') }"
      @search="search"
      @update:value="choose"
    >
      <template #empty>{{ busy ? '正在搜索…' : '没有匹配的在职人员' }}</template>
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
