<script setup lang="ts">
import { ref, watch } from 'vue'
import ManagerPicker from './ManagerPicker.vue'
import { api, errorText, send } from '../api'

type Impact = {
  pending_approval_count: number
  future_approval_count: number
  bound_template_count: number
  pending_approvals: { id: string; template_name: string; current: boolean }[]
  pending_payments: number
  payment_covered: boolean
  bound_templates: { id: string; name: string }[]
  subordinates: { id: number; name: string; employee_no?: number | null }[]
  out_of_scope_subordinates: number
  can_transfer_approvals: boolean
  ready: boolean
}
const props = defineProps<{ userId: number | string }>()
const emit = defineEmits<{ ready: [ready: boolean] }>()
const summary = ref<Impact>()
const loading = ref(false)
const error = ref('')
const notice = ref('')
const target = ref(0)
const mode = ref<'approvals' | 'subordinates'>()
async function load() {
  emit('ready', false)
  loading.value = true
  error.value = ''
  try {
    summary.value = (await api<{ finance_handover: Impact }>('/oa-api/people/' + props.userId + '/handover')).finance_handover
    emit('ready', summary.value?.ready === true)
  } catch (cause) { summary.value = undefined; error.value = errorText(cause) }
  finally { loading.value = false }
}
async function apply() {
  if (!mode.value || !target.value || loading.value) return
  loading.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await send<{ count: number }>('/oa-api/people/' + props.userId + '/handover', { kind: mode.value, target_id: target.value })
    notice.value = `已交接 ${result.count} 项`
    mode.value = undefined
    target.value = 0
    await load()
  } catch (cause) { error.value = errorText(cause) }
  finally { loading.value = false }
}
function begin(kind: 'approvals' | 'subordinates') {
  mode.value = kind
  target.value = 0
  notice.value = ''
  error.value = ''
}
watch(() => props.userId, load, { immediate: true })
</script>

<template>
  <section class="finance-handover" aria-label="费用与直属下属交接">
    <h4>离职影响清单</h4>
    <p v-if="loading && !summary" class="oa-muted" role="status">正在核对交接影响…</p>
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button type="button" @click="load">重新核对</button></p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <template v-if="summary">
      <p v-if="!summary.ready" class="oa-error">完成以下交接后可停用账号或登记离职。</p>
      <dl class="oa-kv">
        <div><dt>待办与后续审批</dt><dd>{{ summary.pending_approval_count }} 笔当前待审 <span v-if="summary.future_approval_count">· {{ summary.future_approval_count }} 笔后续审批</span>
          <button v-if="summary.can_transfer_approvals && summary.pending_approvals.length" type="button" @click="begin('approvals')">批量转交审批</button>
        </dd></div>
        <div><dt>共享待付款池</dt><dd>{{ summary.pending_payments }} 笔<span v-if="!summary.payment_covered"> · 需先授权其他在职付款经办人</span></dd></div>
        <div><dt>固定审批或加签模板</dt><dd>{{ summary.bound_template_count }} 个<span v-if="summary.bound_template_count"> · 请费用管理员发布替换审批人的模板</span></dd></div>
        <div><dt>直属下属</dt><dd>{{ summary.subordinates.length + summary.out_of_scope_subordinates }} 人
          <button v-if="summary.subordinates.length && !summary.out_of_scope_subordinates" type="button" @click="begin('subordinates')">批量替换直属负责人</button>
          <span v-if="summary.out_of_scope_subordinates"> · {{ summary.out_of_scope_subordinates }} 人超出当前权限范围</span>
        </dd></div>
      </dl>
      <details v-if="summary.pending_approvals.some(item => item.id)">
        <summary>查看涉及的费用单据</summary>
        <ul><li v-for="item in summary.pending_approvals" :key="item.id">{{ item.template_name }} · {{ item.id }} · {{ item.current ? '当前审批' : '后续审批' }}</li></ul>
      </details>
      <details v-if="summary.bound_templates.some(item => item.id)">
        <summary>查看涉及的费用模板</summary>
        <ul><li v-for="item in summary.bound_templates" :key="item.id">{{ item.name }}</li></ul>
      </details>
      <details v-if="summary.subordinates.length">
        <summary>查看范围内的直属下属</summary>
        <ul><li v-for="item in summary.subordinates" :key="item.id">{{ item.name }}<span v-if="item.employee_no != null"> · {{ String(item.employee_no).padStart(4, '0') }}</span></li></ul>
      </details>
      <form v-if="mode" class="oa-form" @submit.prevent="apply">
        <ManagerPicker v-model="target" :exclude="Number(userId)" :disabled="loading" :label="mode === 'approvals' ? '审批接手人' : '新直属负责人'" />
        <div class="oa-actions"><button type="button" :disabled="loading" @click="mode = undefined">取消</button><button class="oa-button-primary" :disabled="loading || !target">确认批量交接</button></div>
      </form>
    </template>
  </section>
</template>

<style scoped>
.finance-handover { display: grid; gap: .5rem; }
.finance-handover .oa-kv dd { display: flex; align-items: center; flex-wrap: wrap; gap: .5rem; }
.finance-handover li { overflow-wrap: anywhere; }
</style>
