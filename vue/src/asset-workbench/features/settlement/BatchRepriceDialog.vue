<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { assetWorkbenchApi, type BatchRepriceResult, type BatchRepriceScope, type PriceMatrixRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import PersonnelPicker from '@aw/shared/ui/PersonnelPicker.vue'

const props = defineProps<{ rule: PriceMatrixRow }>()
const emit = defineEmits<{ close: []; completed: [result: BatchRepriceResult] }>()

const month = ref(currentBusinessMonth())
const monthInput = ref<HTMLInputElement | null>(null)
onMounted(() => monthInput.value?.focus())
const payee = ref(0)
const from = ref('')
const to = ref('')
const includePriced = ref(false)
const saving = ref(false)
const saveError = ref('')
const scope = computed<BatchRepriceScope>(() => ({
  rule_id: props.rule.id,
  business_month: month.value,
  payee_user_id: payee.value || undefined,
  submitted_from: from.value || undefined,
  submitted_to: to.value || undefined,
  include_priced: includePriced.value,
}))
const validationError = computed(() => {
  if (!month.value) return '请选择结算月。'
  if (from.value && to.value && from.value > to.value) return '开始日期不能晚于结束日期。'
  return ''
})
const request = usePageRequest(
  signal => assetWorkbenchApi.previewBatchReprice(scope.value, signal),
  null,
  '重算预览加载失败，请重试。',
)
const { data: preview, loading, error } = request

watch(scope, () => {
  request.reset()
  saveError.value = ''
}, { flush: 'sync' })

const columns = [
  { key: 'order_no', label: '作品 / 订单', width: 160 },
  { key: 'payee_user_id', label: '账号编号', width: 90 },
  { key: 'submitted_on', label: '原提交日', width: 114 },
  { key: 'old_gross_label', label: '原毛额', width: 100, align: 'right' as const },
  { key: 'new_gross_label', label: '重算毛额', width: 104, align: 'right' as const },
]
const rows = computed(() => (preview.value?.items ?? []).map(item => ({
  ...item,
  order_no: item.order_no || '作品 #' + item.item_id,
  old_gross_label: formatMoney(item.old_gross),
  new_gross_label: formatMoney(item.new_gross),
})))

function close() {
  if (!saving.value) emit('close')
}

async function previewChanges() {
  if (validationError.value || loading.value || saving.value) return
  request.reset()
  saveError.value = ''
  await request.run()
}

async function confirmChanges() {
  if (saving.value || loading.value || !preview.value?.count) return
  saving.value = true
  saveError.value = ''
  try {
    const result = await assetWorkbenchApi.confirmBatchReprice(scope.value, preview.value.preview_token)
    emit('completed', result)
  } catch (err) {
    request.reset()
    saveError.value = err instanceof Error ? err.message : '重算失败，请刷新结果后重新预览。'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="aw-dialog-backdrop" role="presentation" @click.self="close">
    <section class="aw-confirm-dialog aw-batch-reprice-dialog" role="dialog" aria-modal="true" aria-labelledby="batch-reprice-title" @keydown.esc.prevent="close">
      <div>
        <h3 id="batch-reprice-title">批量重新计件</h3>
        <p class="aw-copy">{{ workerTypeMeta(rule.worker_type).label }} / {{ rule.job_grade }} / {{ rule.difficulty_class }} · {{ formatMoney(rule.unit_price) }}</p>
        <p class="aw-copy">单价适用日 {{ rule.effective_from.slice(0, 10) }}{{ rule.effective_to ? ' 至 ' + rule.effective_to.slice(0, 10) : ' 起' }}。按作品原提交日匹配，进入结算批次的作品会跳过。</p>
      </div>
      <div class="aw-form-grid">
        <label>
          结算月
          <input ref="monthInput" v-model="month" type="month" :disabled="saving" />
        </label>
        <label>
          原提交日起
          <input v-model="from" type="date" :disabled="saving" />
        </label>
        <label>
          原提交日至
          <input v-model="to" type="date" :disabled="saving" />
        </label>
        <div class="aw-form-grid__full" :inert="saving">
          <PersonnelPicker v-model="payee" label="限定人员（可选）" hint="留空时查询该规则的所有适用人员" clearable compact />
        </div>
        <p class="aw-copy aw-form-grid__full">默认只重算待补价作品。</p>
        <label class="aw-inline-check aw-form-grid__full">
          <input v-model="includePriced" type="checkbox" :disabled="saving" />
          包含尚未进入结算批次的已计价作品
        </label>
      </div>

      <p v-if="validationError || error || saveError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ validationError || error || saveError }}</p>
      <template v-if="preview">
        <div v-if="preview.count" class="aw-confirm-dialog__summary" aria-live="polite">
          <div><span>受影响作品</span><strong>{{ formatInt(preview.count) }} 件</strong></div>
          <div><span>原毛额</span><strong>{{ formatMoney(preview.old_gross) }}</strong></div>
          <div><span>重算毛额</span><strong>{{ formatMoney(preview.new_gross) }}</strong></div>
        </div>
        <WorkbenchDataGrid
          v-if="preview.count"
          :columns="columns"
          :rows="rows"
          row-key="item_id"
          storage-key="batch-reprice-preview"
          :height="170"
          :row-height="34"
        />
        <p v-else class="aw-copy" role="status">该范围内没有需要按这条规则重算的作品。</p>
      </template>
      <div class="aw-inline-actions">
        <button class="aw-secondary-button" type="button" :disabled="saving" @click="close">取消</button>
        <button class="aw-secondary-button" type="button" :disabled="!!validationError || loading || saving" @click="previewChanges">
          {{ loading ? '预览中…' : preview ? '重新预览' : '预览重算' }}
        </button>
        <button v-if="preview?.count" class="aw-primary-button" type="button" :disabled="saving || loading" @click="confirmChanges">
          {{ saving ? '正在重算…' : `确认重算 ${preview.count} 件` }}
        </button>
      </div>
    </section>
  </div>
</template>
