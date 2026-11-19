<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import { api, errorText } from '../api'

const model = defineModel<number>({ required: true })
const props = defineProps<{ label?: string; currentName?: string; emptyLabel?: string; exclude?: number; department?: number; team?: number; disabled?: boolean }>()
const query = ref('')
const items = ref<{ id: number; name: string; employee_no?: number | null }[]>([])
const failed = ref('')
const busy = ref(false)
let request = 0

const options = computed(() => {
  const available = items.value.filter(item => item.id !== props.exclude).map(item => ({
    value: String(item.id),
    label: item.name + (item.employee_no != null ? ' · ' + String(item.employee_no).padStart(4, '0') : ''),
  }))
  if (model.value && !available.some(item => item.value === String(model.value))) {
    available.unshift({ value: String(model.value), label: props.currentName || '目前所选负责人' })
  }
  return [{ value: '0', label: props.emptyLabel || '跟随组织默认负责人' }, ...available]
})
const selected = computed({
  get: () => String(model.value),
  set: (value: unknown) => { model.value = Number(value || 0) },
})
async function search() {
  const sequence = ++request
  busy.value = true
  failed.value = ''
  const params = new URLSearchParams({ q: query.value.trim(), status: 'active', page_size: '25' })
  if (props.department) params.set('department_id', String(props.department))
  if (props.team) params.set('team_id', String(props.team))
  try {
    const result = await api<{ items: typeof items.value }>('/oa-api/people/roster?' + params)
    if (sequence === request) items.value = result.items
  } catch (cause) {
    if (sequence === request) failed.value = errorText(cause)
  } finally {
    if (sequence === request) busy.value = false
  }
}
onMounted(search)
watch(() => [props.department, props.team], () => { items.value = []; void search() })
</script>

<template>
  <div class="oa-form manager-picker">
    <label><span>{{ label || '搜索直属负责人' }}</span><input v-model="query" type="search" placeholder="输入姓名或工号" :disabled="disabled" @keydown.enter.prevent="search" /></label>
    <div class="oa-actions">
      <button class="oa-button" type="button" :disabled="disabled || busy" @click="search">{{ busy ? '搜索中…' : '搜索' }}</button>
      <div class="manager-options"><BaseSelect v-model="selected" filterable filter-placeholder="筛选当前结果" :options="options" :disabled="disabled || busy" label="选择负责人" /></div>
    </div>
    <p v-if="failed" class="oa-error" role="alert">{{ failed }}</p>
  </div>
</template>
<style scoped>
.manager-options { flex: 1; min-inline-size: 9rem; }
</style>
