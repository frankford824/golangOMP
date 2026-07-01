<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { useSetupChecklist } from '@aw/app/useSetupChecklist'
import { assetWorkbenchApi, type SettlementPreview, type SubmissionRow } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import LedgerReadout from '@aw/shared/console/LedgerReadout.vue'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { submissionStatusMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

interface DashboardData {
  submissions: SubmissionRow[]
  settlement: SettlementPreview | null
}

const month = ref(new Date().toISOString().slice(0, 7))
const { bootstrap } = useAssetWorkbenchBootstrap()
const {
  steps: setupSteps,
  completedCount: setupCompletedCount,
  collapsed: setupCollapsed,
  loading: setupLoading,
  refresh: refreshSetup,
  toggleCollapsed: toggleSetupCollapsed,
} = useSetupChecklist(() => bootstrap.value)
const dashboardRequest = usePageRequest<DashboardData>(
  async () => {
    const [submissionResult, settlementResult] = await Promise.allSettled([
      assetWorkbenchApi.listSubmissions({ page: 1, page_size: 20 }),
      assetWorkbenchApi.previewSettlement(month.value),
    ])
    if (submissionResult.status === 'rejected' && settlementResult.status === 'rejected') {
      throw settlementResult.reason
    }
    return {
      submissions: submissionResult.status === 'fulfilled' ? submissionResult.value.items : [],
      settlement: settlementResult.status === 'fulfilled' ? settlementResult.value : null,
    }
  },
  { submissions: [], settlement: null },
  '今日待办数据加载失败',
)
const dashboardData = computed(() => dashboardRequest.data.value ?? { submissions: [], settlement: null })
const submissions = computed(() => dashboardData.value.submissions)
const settlement = computed(() => dashboardData.value.settlement)
const loading = dashboardRequest.loading
const error = dashboardRequest.error

const submittedCount = computed(() => settlement.value?.totals?.item_count ?? 0)
const pendingSubmissionCount = computed(() => submissions.value.filter((item) => item.status !== 'checked').length)
const netAmount = computed(() => settlement.value?.totals?.net_amount ?? 0)
const pagesCount = computed(() => settlement.value?.totals?.page_count ?? 0)
const recentSubmissions = computed(() => submissions.value.slice(0, 8))
const ledgerSegments = computed(() => [
  { key: 'submitted', label: '成品单数', value: formatInt(submittedCount.value), hint: '本月已提交计件单', expandable: true },
  { key: 'pending', label: '待处理', value: formatInt(pendingSubmissionCount.value), hint: '待质检 / 待修正', expandable: true },
  { key: 'pages', label: '页数合计', value: formatInt(pagesCount.value), hint: '计价以页为单位', expandable: true },
  { key: 'net', label: '本月预估净额', value: formatMoney(netAmount.value), hint: '正常计件与补录分开结算', money: true, expandable: true },
])

const pendingSubmissions = computed(() => submissions.value.filter((item) => item.status !== 'checked'))
const payrollGridRows = computed(
  () =>
    (settlement.value?.payroll_rows ?? []).map((row) => ({
      ...row,
      grid_id: `${row.payee_user_id}-${row.row_type}`,
      row_label: row.row_type === 'supplement_piecework' ? '补录计件工资' : '正常计件工资',
    })) as unknown as Record<string, unknown>[],
)
const payrollColumns = [
  { key: 'payee_user_id', label: '人员', width: 96 },
  { key: 'row_label', label: '工资条', width: 150 },
  { key: 'item_count', label: '单数', width: 80, align: 'right' as const },
  { key: 'page_count', label: '页数', width: 80, align: 'right' as const },
  { key: 'gross_amount', label: '毛额', width: 104, align: 'right' as const },
  { key: 'deduction_amount', label: '质检扣减', width: 112, align: 'right' as const },
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

const loadDashboard = dashboardRequest.run

onMounted(() => {
  void loadDashboard()
  void refreshSetup()
})
</script>

<template>
  <section class="aw-page-stack aw-dashboard-page">
    <section v-if="setupSteps.length" class="aw-panel aw-setup-checklist">
      <div class="aw-panel__head">
        <div>
          <h3>开站清单</h3>
          <p class="aw-copy">先完成配置，再接收第一件作品。旧菜单名仍可在 ⌘K 中搜索。</p>
        </div>
        <div class="aw-inline-actions">
          <span class="aw-chip aw-chip--neutral">{{ setupCompletedCount }}/{{ setupSteps.length }} 已完成</span>
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

    <LedgerReadout :eyebrow="`本月台账 · ${month}`" title="今日先处理这些事" :segments="ledgerSegments">
      <template #actions>
        <button class="aw-console-button" type="button" :disabled="loading" @click="loadDashboard">刷新</button>
      </template>
      <template #detail="{ segment }">
        <div v-if="segment.key === 'pending'" class="aw-sheet-detail">
          <div v-if="pendingSubmissions.length" class="aw-contact-sheet">
            <RouterLink
              v-for="submission in pendingSubmissions"
              :key="submission.id"
              class="aw-contact-tile"
              to="/submissions"
            >
              <strong>{{ submission.submission_no }}</strong>
              <small>{{ submissionStatusMeta(submission.status).label }} · {{ formatInt(submission.item_count) }} 单</small>
            </RouterLink>
          </div>
          <div v-else class="aw-empty-state">
            <h3>没有待处理批次</h3>
            <p>当前提交都已质检通过，暂时不需要你介入。</p>
          </div>
        </div>
        <div v-else class="aw-sheet-detail">
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

    <AsyncBoundary
      :loading="loading"
      :error="error"
      loading-label="正在加载今日待办"
      @retry="loadDashboard"
    >
      <div class="aw-two-column">
        <section class="aw-panel">
          <div class="aw-panel__head"><h3>常用入口</h3></div>
          <div class="aw-inline-actions">
            <RouterLink class="aw-primary-button" to="/upload">上传成品</RouterLink>
            <RouterLink class="aw-secondary-button" to="/submissions">查改作品</RouterLink>
            <RouterLink class="aw-secondary-button" to="/settlement">本月结算</RouterLink>
          </div>
        </section>
        <section class="aw-panel">
          <div class="aw-panel__head">
            <h3>最近提交</h3>
            <RouterLink v-if="recentSubmissions.length" class="aw-link-button" to="/submissions">查看全部</RouterLink>
          </div>
          <div v-if="recentSubmissions.length" class="aw-contact-sheet">
            <RouterLink
              v-for="submission in recentSubmissions"
              :key="submission.id"
              class="aw-contact-tile"
              to="/submissions"
            >
              <strong>{{ submission.submission_no }}</strong>
              <small>{{ submissionStatusMeta(submission.status).label }} · {{ formatInt(submission.item_count) }} 单</small>
            </RouterLink>
          </div>
          <div v-else class="aw-empty-state">
            <h3>还没有提交</h3>
            <p>上传第一批成品后，这里会按提交时间显示需要质检的批次。</p>
            <RouterLink class="aw-primary-button" to="/upload">上传成品</RouterLink>
          </div>
        </section>
      </div>
    </AsyncBoundary>
  </section>
</template>
