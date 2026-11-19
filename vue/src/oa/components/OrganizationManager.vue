<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, errorText, send } from '../api'
import ManagerPicker from './ManagerPicker.vue'
import '../styles.css'

const emit = defineEmits<{ saved: [] }>()
const props = defineProps<{ kind: 'department' | 'team'; id: number }>()
type OrgManager = { kind: 'department' | 'team'; id: number; manager_id: number; manager_name?: string; department_id: number; version: number }
const record = ref<OrgManager>()
const manager = ref(0)
const busy = ref(false)
const error = ref('')
const notice = ref('')
let request = 0
async function load() {
  const sequence = ++request
  record.value = undefined
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    const response = await api<{ items: OrgManager[] }>('/oa-api/people/org-managers')
    if (sequence !== request) return
    record.value = response.items.find(item => item.kind === props.kind && item.id === props.id)
    manager.value = record.value?.manager_id || 0
  } catch (cause) { if (sequence === request) error.value = errorText(cause) }
  finally { if (sequence === request) busy.value = false }
}
async function save() {
  if (!record.value || busy.value) return
  const sequence = request
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await send<OrgManager>('/oa-api/people/org-managers/' + props.kind + '/' + props.id,
      { manager_id: manager.value, version: record.value.version }, 'PUT')
    if (sequence === request) {
      record.value = result
      notice.value = '默认负责人已保存'
      emit('saved')
    }
  } catch (cause) { if (sequence === request) error.value = errorText(cause) }
  finally { if (sequence === request) busy.value = false }
}
watch(() => [props.kind, props.id], load, { immediate: true })
</script>
<template>
  <section class="oa-form organization-manager">
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button class="oa-button" type="button" @click="load">重新读取</button></p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <p v-if="busy && !record" role="status">读取负责人配置…</p>
    <form v-if="record" class="oa-form" @submit.prevent="save">
      <ManagerPicker v-model="manager" :current-name="record.manager_name" :department="record.department_id"
        :team="record.kind === 'team' ? record.id : undefined" label="搜索组织默认负责人"
        :empty-label="record.kind === 'team' ? '跟随所属部门' : '无默认负责人'" :disabled="busy" />
      <button class="oa-button-primary" type="submit" :disabled="busy || manager === record.manager_id">保存默认负责人</button>
    </form>
  </section>
</template>
