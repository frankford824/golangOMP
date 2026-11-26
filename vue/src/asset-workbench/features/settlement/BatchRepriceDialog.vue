<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { assetWorkbenchApi, type BatchRepriceResult, type BatchRepriceScope, type PersonnelSuggestion, type PriceMatrixRow } from '@aw/shared/api/assetWorkbenchApi'
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
const personnelMode = ref<'all' | 'selected'>('all')
const people = ref<PersonnelSuggestion[]>([])
const receipt = ref<BatchRepriceResult | null>(null)
const ruleStart = computed(() => props.rule.effective_from.slice(0, 10))
function suggestedStart(value: string) {
  const firstDay = value + '-01'
  return ruleStart.value > firstDay ? ruleStart.value : firstDay
}
const from = ref(suggestedStart(month.value))
watch(month, (next, previous) => {
  if (from.value === suggestedStart(previous) && next) from.value = suggestedStart(next)
})
function addPerson(person: PersonnelSuggestion) {
  if (!people.value.some(row => row.user_id === person.user_id)) people.value.push(person)
}
const personnelLabel = computed(() => personnelMode.value === 'all' ? '全部适用人员' :
  people.value.map(person => person.name + '（' + (person.personnel_no || person.user_id) + '）').join('、'))

const to = ref('')
const includePriced = ref(true)
const saving = ref(false)
const saveError = ref('')
const scope = computed<BatchRepriceScope>(() => ({
  rule_id: props.rule.id,
  business_month: month.value,
  payee_user_ids: personnelMode.value === 'selected' ? people.value.map(p => p.user_id) : undefined,
  submitted_from: from.value || undefined,
  submitted_to: to.value || undefined,
  include_priced: includePriced.value,
}))
const validationError = computed(() => {
  if (personnelMode.value === 'selected' && !people.value.length) return '请添加至少一位人员。'
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
  if (receipt.value || saving.value || loading.value || !preview.value?.count) return
  saving.value = true
  saveError.value = ''
  try {
    const result = await assetWorkbenchApi.confirmBatchReprice(scope.value, preview.value.preview_token)
    receipt.value = result
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
        <p class="aw-copy">单价适用日 {{ rule.effective_from.slice(0, 10) }}{{ rule.effective_to ? ' 至 ' + rule.effective_to.slice(0, 10) : ' 起' }}。在所选结算月内按日常作品的原提交日匹配，进入结算批次的作品会跳过。</p>
      </div>
      <div v-if="!receipt" class="aw-form-grid">
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
        <label>人员范围
          <select v-model="personnelMode" :disabled="saving">
            <option value="all">全部适用人员</option>
            <option value="selected">指定人员（可多选）</option>
          </select>
        </label>
        <div v-if="personnelMode === 'selected'" class="aw-form-grid__full" :inert="saving">
          <PersonnelPicker :model-value="0" add-to-list label="选择要加入的人员" hint="可继续添加其他人员" compact @selected="addPerson" />
          <ul v-if="people.length" aria-label="已选人员">
            <li v-for="person in people" :key="person.user_id">
              {{ person.name }} · 人员编号 {{ person.personnel_no || person.user_id }}
              <button class="aw-grid-button" type="button" :aria-label="'移除' + person.name" @click="people = people.filter(p => p.user_id !== person.user_id)">移除</button>
            </li>
          </ul>
        </div>
        <label class="aw-inline-check aw-form-grid__full">
          <input v-model="includePriced" type="checkbox" :disabled="saving" />
          包含尚未进入结算批次的已计价作品
        </label>
      </div>

      <p v-if="validationError || error || saveError" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ validationError || error || saveError }}</p>
      <div v-if="receipt" role="status" class="aw-inline-alert">
        <strong>已重算 {{ formatInt(receipt.updated) }} 件作品</strong>
        <p>{{ month }} · {{ personnelLabel }} · 原提交日 {{ from || ruleStart }} 起{{ to ? '，截至 ' + to : '' }}</p>
        <p>原毛额 {{ formatMoney(receipt.old_gross) }} → 重算毛额 {{ formatMoney(receipt.new_gross) }}</p>
      </div>
      <template v-else-if="preview">
        <p v-if="preview.count" role="status">预览完成，尚未写入。确认后按下列金额重新计件。</p>
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
        <p v-else class="aw-copy" role="status">预览完成，0 件需要重算。请核对所选人员、原提交日及 {{ rule.difficulty_class }} 类规则；已进批次或计价结果相同的作品会跳过。</p>
      </template>
      <div class="aw-inline-actions">
        <button class="aw-secondary-button" type="button" :disabled="saving" @click="close">{{ receipt ? '完成' : '取消' }}</button>
        <button v-if="!receipt" class="aw-secondary-button" type="button" :disabled="!!validationError || loading || saving" @click="previewChanges">
          {{ loading ? '预览中…' : preview ? '重新预览' : '预览重算' }}
        </button>
        <button v-if="!receipt && preview?.count" class="aw-primary-button" type="button" :disabled="saving || loading" @click="confirmChanges">
          {{ saving ? '正在重算…' : `确认重算 ${preview.count} 件` }}
        </button>
      </div>
    </section>
  </div>
</template>
