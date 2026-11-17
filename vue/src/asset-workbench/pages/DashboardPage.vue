<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { canAccessPath } from '@aw/app/access'
import UploadPendingList from '@aw/features/upload/UploadPendingList.vue'
import { useSetupChecklist } from '@aw/app/useSetupChecklist'
import { assetWorkbenchApi, type SubmissionRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import LedgerReadout from '@aw/shared/console/LedgerReadout.vue'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import { currentBusinessMonth } from '@aw/shared/format/businessMonth'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { submissionStatusMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const month = currentBusinessMonth()
const { bootstrap } = useAssetWorkbenchBootstrap()
const {
  steps: setupSteps,
  completedCount: setupCompletedCount,
  collapsed: setupCollapsed,
  loading: setupLoading,
  refresh: refreshSetup,
  toggleCollapsed: toggleSetupCollapsed,
} = useSetupChecklist(() => bootstrap.value)
const capabilities = computed(() => new Set(bootstrap.value?.capabilities ?? []))
const canReview = computed(() => capabilities.value.has('asset.workbench.manage') || capabilities.value.has('asset.workbench.settlement'))
const canViewSettlement = computed(() => capabilities.value.has('asset.workbench.settlement'))
const canOpenLedger = computed(() => canAccessPath(bootstrap.value, '/upload-overview'))
const canUpload = computed(() => canAccessPath(bootstrap.value, '/upload'))
const recentRequest = usePageRequest(signal => assetWorkbenchApi.listSubmissions({ page: 1, page_size: 6 }, signal))
const settlementRequest = usePageRequest(signal => assetWorkbenchApi.previewSettlement(month, signal))
const recentSubmissions = computed(() => recentRequest.data.value?.items ?? [])
const settlement = settlementRequest.data
const ledgerSegments = computed(() => [
  { key: 'submitted', label: '成品单数', value: formatInt(settlement.value?.totals.item_count), hint: '本月已提交计件单', expandable: true },
  { key: 'pages', label: '页数合计', value: formatInt(settlement.value?.totals.page_count), hint: '计价以页为单位', expandable: true },
  { key: 'net', label: '本月预估净额', value: formatMoney(settlement.value?.totals.net_amount), hint: '日常计件与补录分开结算', money: true, expandable: true },
])

const payrollGridRows = computed(
  () =>
    (settlement.value?.payroll_rows ?? []).map((row) => ({
      ...row,
      grid_id: `${row.payee_user_id}-${row.row_type}`,
      row_label: row.row_type === 'supplement_piecework' ? '补录计件工资' : '日常计件工资',
    })) as unknown as Record<string, unknown>[],
)
const payrollColumns = [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'row_label', label: '工资条', width: 150 },
  { key: 'item_count', label: '单数', width: 80, align: 'right' as const },
  { key: 'page_count', label: '页数', width: 80, align: 'right' as const },
  { key: 'gross_amount', label: '毛额', width: 104, align: 'right' as const },
  { key: 'deduction_amount', label: '质检扣款', width: 112, align: 'right' as const },
  { key: 'net_amount', label: '净额', width: 116, align: 'right' as const },
]
const moneyColumns = new Set(['gross_amount', 'deduction_amount', 'net_amount'])
const intColumns = new Set(['item_count', 'page_count'])

function isMoneyColumn(key: string) {
  return moneyColumns.has(key)
}

function gridValue(key: string, value: unknown) {
  if (moneyColumns.has(key)) return formatMoney(value)
  if (intColumns.has(key)) return formatInt(value)
  return value || '—'
}

function submissionDriveTarget(submission: SubmissionRow) {
  return {
    path: canOpenLedger.value ? '/upload-overview' : '/drive',
    query: {
      ...(canOpenLedger.value ? {} : { scope: 'orders' }),
      q: submission.submission_no,
    },
  }
}

onMounted(() => {
  void recentRequest.run()
  if (canViewSettlement.value) void settlementRequest.run()
  void refreshSetup()
})
</script>

<template>
  <section class="aw-page-stack aw-dashboard-page">
    <UploadPendingList :can-review="canReview" :can-open-ledger="canOpenLedger" :can-upload="canUpload" />
    <AsyncBoundary
      v-if="canViewSettlement"
      :loading="settlementRequest.loading.value"
      :error="settlementRequest.error.value"
      loading-label="正在读取月度指标"
      @retry="settlementRequest.run"
    >
      <LedgerReadout v-if="settlement" :eyebrow="month" title="月度指标" :segments="ledgerSegments">
        <template #detail>
          <div class="aw-sheet-detail">
            <WorkbenchDataGrid
              v-if="payrollGridRows.length"
              :columns="payrollColumns"
              :rows="payrollGridRows"
              row-key="grid_id"
              storage-key="dashboard-ledger-payroll"
              group-by="payee_user_id"
              :height="440"
              :row-height="34"
            >
              <template #cell="{ column, value }">
                <span v-if="isMoneyColumn(column.key)" class="aw-cell-money">{{ gridValue(column.key, value) }}</span>
                <span v-else-if="column.align === 'right'" class="aw-cell-num">{{ gridValue(column.key, value) }}</span>
                <span v-else>{{ gridValue(column.key, value) }}</span>
              </template>
            </WorkbenchDataGrid>
            <div v-else class="aw-empty-state">
              <h3>还没有可结算明细</h3>
              <p>本月暂无计件工资行，去“结算”生成预览后，这里会按人员分组显示工资条。</p>
            </div>
          </div>
        </template>
      </LedgerReadout>
    </AsyncBoundary>
    <section class="aw-panel">
      <div class="aw-panel__head">
        <h3>最近提交</h3>
        <RouterLink v-if="canOpenLedger" class="aw-link-button" to="/upload-overview">查看上传台账</RouterLink>
      </div>
      <AsyncBoundary :loading="recentRequest.loading.value" :error="recentRequest.error.value" loading-label="正在读取最近提交" @retry="recentRequest.run">
        <ul v-if="recentSubmissions.length" class="aw-submission-tasks__list">
          <li v-for="submission in recentSubmissions" :key="submission.id">
            <RouterLink class="aw-submission-tasks__row" :to="submissionDriveTarget(submission)">
              <strong>{{ submission.submission_no }}</strong>
              <span class="aw-copy">{{ formatShanghaiDateTime(submission.submitted_at) }} · {{ formatInt(submission.item_count) }} 件</span>
              <span class="aw-submission-tasks__action">{{ submissionStatusMeta(submission.status).label }}</span>
            </RouterLink>
          </li>
        </ul>
        <p v-else class="aw-copy">还没有提交记录。</p>
      </AsyncBoundary>
    </section>
    <section v-if="setupSteps.length" class="aw-panel aw-setup-checklist">
      <div class="aw-panel__head">
        <div>
          <h3>开站清单</h3>
          <p class="aw-copy">检查工作台配置和启用进度。</p>
        </div>
        <div class="aw-inline-actions">
          <span class="aw-chip aw-chip--neutral">{{ setupSteps.length - setupCompletedCount }} 项待完成</span>
          <button class="aw-secondary-button" type="button" :disabled="setupLoading" @click="refreshSetup()">刷新进度</button>
          <button class="aw-secondary-button" type="button" @click="toggleSetupCollapsed()">
            {{ setupCollapsed ? '展开清单' : '折叠清单' }}
          </button>
        </div>
      </div>
      <ol v-if="!setupCollapsed" class="aw-setup-checklist__list">
        <li v-for="step in setupSteps" :key="step.id" class="aw-setup-checklist__item" :data-done="step.done">
          <div>
            <strong>{{ step.label }}</strong>
            <span>{{ step.hint }}</span>
          </div>
          <RouterLink class="aw-secondary-button" :to="step.to">{{ step.done ? '查看' : '去完成' }}</RouterLink>
        </li>
      </ol>
    </section>

  </section>
</template>
