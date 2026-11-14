<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { assetWorkbenchApi, type ErrorImportBatchRow, type QualityImportReviewRow, type QualityImportRowOverride } from '@aw/shared/api/assetWorkbenchApi'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const props = defineProps<{ file: File }>()
const emit = defineEmits<{ close: []; imported: [batch: ErrorImportBatchRow] }>()
const rows = ref<QualityImportReviewRow[]>([])
const selectedMonth = ref('')
const month = ref('')
const latestMonth = currentBusinessMonth()
const monthReady = computed(() => /^\d{4}-(0[1-9]|1[0-2])$/.test(selectedMonth.value) && selectedMonth.value <= latestMonth)
const choices = ref<Record<string, QualityImportRowOverride['payroll_row_type'] | ''>>({})
const reviewing = ref(false)
const submitting = ref(false)
const error = ref('')
const showAll = ref(false)
const page = ref(1)
const pageSize = 25
let request: AbortController | undefined

const rowKey = (row: QualityImportReviewRow) => `${row.sheet}:${row.row}`
const payrollType = (row: QualityImportReviewRow) => choices.value[rowKey(row)] || row.payroll_row_type
const needsAttention = (row: QualityImportReviewRow) => row.blocking || row.match_status !== 'matched'
const attentionCount = computed(() => rows.value.filter(needsAttention).length)
const pendingCount = computed(() => rows.value.filter(row => row.blocking && (!row.needs_payroll_type || !choices.value[rowKey(row)])).length)
const excludedCount = computed(() => rows.value.filter(row => !row.blocking && row.match_status !== 'matched').length)
const countedRows = computed(() => rows.value.filter(row => row.match_status === 'matched'))
const normalCount = computed(() => countedRows.value.filter(row => payrollType(row) === 'normal_piecework').length)
const supplementCount = computed(() => countedRows.value.filter(row => payrollType(row) === 'supplement_piecework').length)
const filtered = computed(() => showAll.value ? rows.value : rows.value.filter(needsAttention))
const lastPage = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const visibleRows = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))

watch(() => [props.file, month.value] as const, async ([file, targetMonth]) => {
  request?.abort()
  const current = new AbortController()
  request = current
  rows.value = []
  choices.value = {}
  error.value = ''
  page.value = 1
  reviewing.value = !!targetMonth
  if (!targetMonth) return
  try {
    const result = await assetWorkbenchApi.previewQualityImportExcel(targetMonth, file, current.signal)
    if (!current.signal.aborted) {
      rows.value = result
      showAll.value = !result.some(needsAttention)
    }
  } catch (err) {
    if (!current.signal.aborted) error.value = resolveApiUserMessage(err, { fallback: '校对失败，请检查 Excel 后重新选择。' })
  } finally {
    if (!current.signal.aborted) reviewing.value = false
  }
}, { immediate: true })

watch(showAll, () => { page.value = 1 })
onBeforeUnmount(() => request?.abort())

function payrollLabel(row: QualityImportReviewRow) {
  const type = payrollType(row)
  if (row.match_status !== 'matched') return '—'
  return type === 'supplement_piecework' ? '补录计件' : type === 'normal_piecework' ? '日常计件' : '待选择'
}

async function submit() {
  if (reviewing.value || submitting.value || pendingCount.value || !rows.value.length) return
  submitting.value = true
  error.value = ''
  const overrides: QualityImportRowOverride[] = []
  for (const row of rows.value) {
    const type = choices.value[rowKey(row)]
    if (type) overrides.push({ sheet: row.sheet, row: row.row, payroll_row_type: type })
  }
  try {
    emit('imported', await assetWorkbenchApi.importErrorExcel(month.value, props.file, undefined, overrides))
  } catch (err) {
    error.value = resolveApiUserMessage(err, { fallback: '质检导入失败，请核对后重试。' })
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="aw-data-surface aw-quality-import" aria-label="质检导入校对">
    <div class="aw-settlement-section__head">
      <div>
        <h3>质检导入校对</h3>
        <p class="aw-copy">{{ file.name }}</p>
      </div>
      <button class="aw-secondary-button" type="button" :disabled="submitting" @click="emit('close')">取消</button>
    </div>
    <form v-if="!month" class="aw-quality-import__summary" @submit.prevent="monthReady && (month = selectedMonth)">
      <label class="aw-month-control">
        <span>导入结算年月</span>
        <input v-model="selectedMonth" type="month" lang="zh-CN" :max="latestMonth" required />
      </label>
      <button class="aw-primary-button" type="submit" :disabled="!monthReady">按所选年月校对</button>
    </form>
    <div v-else class="aw-quality-import__summary">
      <span>将导入到 <strong>{{ month }}</strong></span>
      <button class="aw-secondary-button" type="button" :disabled="submitting" @click="month = ''; selectedMonth = ''">重新选择年月</button>
    </div>
    <p v-if="reviewing" class="aw-copy" role="status">正在核对人员和作品归属…</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>
    <template v-if="rows.length">
      <div class="aw-quality-import__summary" aria-live="polite">
        <span>日常计件 <strong>{{ normalCount }}</strong> 行</span>
        <span>补录计件 <strong>{{ supplementCount }}</strong> 行</span>
        <span v-if="pendingCount" class="aw-chip aw-chip--warn">待处理 {{ pendingCount }} 行</span>
        <span v-if="excludedCount" class="aw-chip aw-chip--warn">{{ excludedCount }} 行不计扣款</span>
        <button
          v-if="attentionCount"
          class="aw-secondary-button"
          type="button"
          :aria-pressed="!showAll"
          @click="showAll = !showAll"
        >{{ showAll ? '只看需核对' : '查看全部' }}</button>
      </div>
      <div class="aw-quality-import__table">
        <table>
          <thead>
            <tr><th scope="col">表 / 行</th><th scope="col">人员 / 作品</th><th scope="col">日期 / 出错</th><th scope="col">工资条</th><th scope="col">校对结果</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in visibleRows" :key="rowKey(row)">
              <td>{{ row.sheet }} / {{ row.row }}</td>
              <td>
                <strong>{{ row.payee_name || '—' }}</strong>
                <div v-if="row.order_no || row.file_name" class="aw-copy">{{ [row.order_no, row.file_name].filter(Boolean).join(' · ') }}</div>
              </td>
              <td>
                <span>{{ row.occurred_date || '—' }} · {{ row.difficulty_class || '—' }} · {{ row.error_count }} 张</span>
                <div v-if="row.issue_description" class="aw-copy">{{ row.issue_description }}</div>
              </td>
              <td>
                <select
                  v-if="row.needs_payroll_type"
                  v-model="choices[rowKey(row)]"
                  class="aw-input"
                  :aria-label="`工作表 ${row.sheet} 第 ${row.row} 行工资条类型`"
                  :disabled="submitting"
                >
                  <option value="">请选择</option>
                  <option value="normal_piecework">日常计件</option>
                  <option value="supplement_piecework">补录计件</option>
                </select>
                <span v-else>{{ payrollLabel(row) }}</span>
              </td>
              <td>{{ choices[rowKey(row)] ? '已手动选择' : row.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="aw-quality-import__footer">
        <div v-if="lastPage > 1" class="aw-quality-import__summary">
          <button class="aw-secondary-button" type="button" :disabled="page <= 1" @click="page--">上一页</button>
          <span>{{ page }} / {{ lastPage }}</span>
          <button class="aw-secondary-button" type="button" :disabled="page >= lastPage" @click="page++">下一页</button>
        </div>
        <button class="aw-primary-button" type="button" :disabled="submitting || pendingCount > 0" @click="submit">
          {{ submitting ? '正在导入…' : `确认导入 ${rows.length} 行` }}
        </button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.aw-quality-import {
  display: grid;
  gap: var(--aw-space-3);
  padding: var(--aw-space-4);
}

.aw-quality-import__summary,
.aw-quality-import__footer {
  display: flex;
  align-items: center;
  gap: var(--aw-space-3);
  flex-wrap: wrap;
}

.aw-quality-import__footer > .aw-primary-button {
  margin-inline-start: auto;
}

.aw-quality-import__table {
  overflow-x: auto;
}

.aw-quality-import__table table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.aw-quality-import__table :is(th, td) {
  border-bottom: thin solid var(--aw-hairline);
  padding: var(--aw-space-2);
  vertical-align: top;
  overflow-wrap: anywhere;
}

.aw-quality-import__table th {
  background: var(--aw-surface-muted);
  white-space: nowrap;
}
</style>
