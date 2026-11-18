<script setup lang="ts">
import { computed, ref } from 'vue'
import { api, errorText, money, send } from '../api'
import { can } from '../session'

type Cost = { sku: string; task_id: number; cost_price?: string | null; version: string; erp_status: string }
const props = defineProps<{ sku?: string; tasks?: { id: number; task_no: string }[] }>()
const allowed = computed(() => can('catalog.manage') && can('erp.manage'))
const selection = ref({ task: '', sku: props.sku || '' })
const data = ref<Cost>()
const price = ref('')
const reason = ref('')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const endpoint = computed(() => `/oa-api/orders/tasks/${encodeURIComponent(selection.value.task)}/skus/${encodeURIComponent(selection.value.sku.trim())}/cost`)

function invalidate() {
  data.value = undefined
  notice.value = ''
  error.value = ''
}
async function load() {
  busy.value = true
  invalidate()
  try {
    data.value = await api<Cost>(endpoint.value)
    price.value = data.value.cost_price ?? ''
    reason.value = ''
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
async function save() {
  if (!data.value || busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    data.value = await send<Cost>(endpoint.value, { version: data.value.version, cost_price: price.value.trim(), reason: reason.value.trim() }, 'PATCH')
    price.value = data.value.cost_price ?? ''
    reason.value = ''
    notice.value = 'OA 本地维护价已保存'
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
</script>

<template>
  <section v-if="allowed" class="oa-panel">
    <h2>SKU 本地维护价</h2>
    <p class="oa-subtitle">历史成交成本按订单记录展示。当前维护价保存在 OA 验证环境，ERP 未核验。</p>
    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <form class="oa-actions oa-filter oa-person-tabs" @submit.prevent="load">
      <label v-if="tasks?.length">关联任务<select v-model="selection.task" required @change="invalidate"><option value="">选择任务</option><option v-for="task in tasks" :key="task.id" :value="String(task.id)">{{ task.task_no }}</option></select></label>
      <label v-else>任务 ID<input v-model.trim="selection.task" inputmode="numeric" pattern="[1-9][0-9]*" required @input="invalidate" /></label>
      <label>SKU<input v-model="selection.sku" :readonly="!!sku" required @input="invalidate" /></label>
      <button :disabled="busy">读取本地维护价</button>
    </form>
    <form v-if="data" class="oa-form" @submit.prevent="save">
      <p class="oa-muted">当前版本 {{ data.version }} · 维护价 {{ money(data.cost_price) }}</p>
      <div class="oa-form-row"><label>新维护价（元）<input v-model.trim="price" inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,4})?" required /></label><label>修改依据<input v-model.trim="reason" required maxlength="1000" /></label></div>
      <button :disabled="busy" class="oa-button-primary">保存维护价</button>
    </form>
  </section>
</template>
