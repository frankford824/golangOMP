<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { assetWorkbenchApi, type SettlementPreview, type SubmissionRow } from '@aw/shared/api/assetWorkbenchApi'

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

function submissionStatusLabel(status: string) {
  const labels: Record<string, string> = {
    submitted: '待质检',
    checked: '已通过',
    needs_fix: '待修正',
    voided: '已作废',
  }
  return labels[status] ?? status
}

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
  <section class="aw-page-stack">
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">本月工作台</p>
        <h2>今日先处理这些事</h2>
      </div>
      <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadDashboard">刷新</button>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>

    <div class="aw-metric-grid">
      <article class="aw-metric-card">
        <span>本月成品单数</span>
        <strong>{{ submittedCount }}</strong>
        <small>{{ pagesCount }} 页，按提交完成时间归入 {{ month }}</small>
      </article>
      <article class="aw-metric-card">
        <span>待质检/待修正</span>
        <strong>{{ pendingSubmissionCount }}</strong>
        <small>进入维护专区处理状态、预览和下载</small>
      </article>
      <article class="aw-metric-card">
        <span>本月预估净额</span>
        <strong class="aw-money">{{ netAmount.toFixed(2) }}</strong>
        <small>正常计件工资与补录计件工资分开展示</small>
      </article>
    </div>

    <div class="aw-two-column">
      <section class="aw-panel">
        <h3>常用入口</h3>
        <div class="aw-inline-actions">
          <RouterLink class="aw-secondary-button" to="/upload">上传成品</RouterLink>
          <RouterLink class="aw-secondary-button" to="/submissions">查看维护区</RouterLink>
          <RouterLink class="aw-secondary-button" to="/settlement">生成结算预览</RouterLink>
        </div>
      </section>
      <section class="aw-panel aw-panel--stage">
        <h3>最近提交</h3>
        <div v-if="recentSubmissions.length" class="aw-contact-sheet">
          <RouterLink
            v-for="submission in recentSubmissions"
            :key="submission.id"
            class="aw-contact-tile"
            to="/submissions"
          >
            <strong>{{ submission.submission_no }}</strong>
            <small>{{ submissionStatusLabel(submission.status) }} · {{ submission.item_count }} 单</small>
          </RouterLink>
        </div>
        <p v-else class="aw-copy">有新提交后，这里会显示最近需要检查的批次。</p>
      </section>
    </div>
  </section>
</template>
