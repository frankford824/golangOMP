<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { assetWorkbenchApi, type SettlementPreview, type SubmissionRow } from '@aw/shared/api/assetWorkbenchApi'
import LedgerReadout from '@aw/shared/console/LedgerReadout.vue'
import { formatInt, formatMoney } from '@aw/shared/format/number'
import { submissionStatusMeta } from '@aw/shared/format/status'

const month = ref(new Date().toISOString().slice(0, 7))
const submissions = ref<SubmissionRow[]>([])
const settlement = ref<SettlementPreview | null>(null)
const loading = ref(false)
const error = ref('')

const submittedCount = computed(() => settlement.value?.totals.item_count ?? 0)
const pendingSubmissionCount = computed(() => submissions.value.filter((item) => item.status !== 'checked').length)
const netAmount = computed(() => settlement.value?.totals.net_amount ?? 0)
const pagesCount = computed(() => settlement.value?.totals.page_count ?? 0)
const recentSubmissions = computed(() => submissions.value.slice(0, 8))
const ledgerSegments = computed(() => [
  { key: 'submitted', label: '成品单数', value: formatInt(submittedCount.value), hint: '本月已提交计件单' },
  { key: 'pending', label: '待处理', value: formatInt(pendingSubmissionCount.value), hint: '待质检 / 待修正' },
  { key: 'pages', label: '页数合计', value: formatInt(pagesCount.value), hint: '计价以页为单位' },
  { key: 'net', label: '本月预估净额', value: formatMoney(netAmount.value), hint: '正常计件与补录分开结算', money: true },
])

async function loadDashboard() {
  loading.value = true
  error.value = ''
  try {
    const [submissionResult, settlementResult] = await Promise.allSettled([
      assetWorkbenchApi.listSubmissions({ page: 1, page_size: 20 }),
      assetWorkbenchApi.previewSettlement(month.value),
    ])
    if (submissionResult.status === 'fulfilled') {
      submissions.value = submissionResult.value.items
    }
    if (settlementResult.status === 'fulfilled') {
      settlement.value = settlementResult.value
    }
    if (submissionResult.status === 'rejected' && settlementResult.status === 'rejected') {
      throw settlementResult.reason
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '总览数据加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadDashboard()
})
</script>

<template>
  <section class="aw-page-stack aw-dashboard-page">
    <p v-if="error" class="aw-inline-alert">{{ error }}</p>

    <LedgerReadout :eyebrow="`本月台账 · ${month}`" title="今日先处理这些事" :segments="ledgerSegments">
      <template #actions>
        <button class="aw-console-button" type="button" :disabled="loading" @click="loadDashboard">刷新</button>
      </template>
    </LedgerReadout>

    <div class="aw-two-column">
      <section class="aw-panel">
        <div class="aw-panel__head"><h3>常用入口</h3></div>
        <div class="aw-inline-actions">
          <RouterLink class="aw-primary-button" to="/upload">上传成品</RouterLink>
          <RouterLink class="aw-secondary-button" to="/submissions">查看维护区</RouterLink>
          <RouterLink class="aw-secondary-button" to="/settlement">生成结算预览</RouterLink>
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
  </section>
</template>
