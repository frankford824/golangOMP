<template>
  <section class="experience-panel">
    <div class="panel-header">
      <div>
        <h3 class="panel-title">经验观测</h3>
        <p class="panel-subtitle">闭环健康、监督样本池、缺口队列与可复核样本</p>
      </div>
      <div class="panel-actions">
        <span v-if="stats?.generated_at" class="panel-time">统计 {{ shortDateTime(stats.generated_at) }}</span>
        <BaseButton variant="secondary" size="sm" :loading="loading" @click="load">
          <RefreshCw class="button-icon" aria-hidden="true" />
          刷新
        </BaseButton>
      </div>
    </div>

    <BaseErrorState v-if="error && !stats" :title="error" @retry="load" />

    <template v-else>
      <p v-if="error" class="block-note review-error" role="alert">{{ error }}</p>

      <div v-if="loading" class="metric-grid">
        <BaseSkeleton v-for="i in 8" :key="i" width="100%" height="4.75rem" />
      </div>

      <BaseEmptyState
        v-else-if="!runtimeFlags.ui_enabled"
        title="经验观测未启用"
        description="当前环境已关闭经验观测页面，属于未配置状态。"
      />

      <template v-else>
        <div class="flag-row" aria-label="经验开关">
          <span :class="flagClass(runtimeFlags.capture_enabled)">采集 {{ flagLabel(runtimeFlags.capture_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.worker_enabled)">Worker {{ flagLabel(runtimeFlags.worker_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.ai_feedback_enabled)">AI 反馈 {{ flagLabel(runtimeFlags.ai_feedback_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.behavior_capture_enabled)">行为 {{ flagLabel(runtimeFlags.behavior_capture_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.micro_question_enabled)">微追问 {{ flagLabel(runtimeFlags.micro_question_enabled) }}</span>
          <span>采样 {{ percentLabel(runtimeFlags.behavior_sample_rate) }}</span>
          <span v-if="runtimeFlags.runtime_config_loaded" class="status-pill status-pill--on">运行配置 已加载</span>
          <span v-else-if="runtimeFlags.runtime_config_error" class="status-pill">运行配置 异常</span>
          <span :class="flagClass(runtimeFlags.ui_enabled)">页面 {{ flagLabel(runtimeFlags.ui_enabled) }}</span>
        </div>
        <p v-if="runtimeFlags.runtime_config_error" class="block-note review-error" role="alert">
          运行配置未生效：{{ runtimeFlags.runtime_config_error }}
        </p>

        <section class="panel-block worker-block" aria-labelledby="experience-worker-title">
          <div class="block-header">
            <div>
              <h4 id="experience-worker-title">Worker 运行</h4>
              <p>用于区分未采集、无数据和后台处理失败。</p>
            </div>
            <span>{{ workerHealthLabel }}</span>
          </div>
          <div v-if="workerRuns.length" class="worker-run-list">
            <article v-for="run in workerRuns.slice(0, 6)" :key="`${run.worker_name}-${run.source_name || 'all'}-${run.started_at}`" class="worker-run-item">
              <div class="worker-run-main">
                <strong>{{ workerNameLabel(run.worker_name) }}</strong>
                <small>{{ run.source_name || 'all' }} · {{ shortDateTime(run.started_at) }}</small>
              </div>
              <div class="worker-run-meta">
                <span :class="workerStatusClass(run.status)">{{ workerStatusLabel(run.status) }}</span>
                <small>{{ integerLabel(run.scanned_count) }} 扫描 / {{ integerLabel(run.enqueued_count) }} 写入 / {{ integerLabel(run.failed_count) }} 失败</small>
              </div>
            </article>
          </div>
          <BaseEmptyState v-else title="暂无 Worker 记录" :description="workerEmptyDescription" />
        </section>

        <section class="panel-block health-block" aria-labelledby="experience-health-title">
          <div class="block-header">
            <div>
              <h4 id="experience-health-title">闭环健康条</h4>
              <p>展示建议 -> 展示可定位 -> 正式反馈 -> 反馈原因 -> 侧路候选</p>
            </div>
            <span>{{ integerLabel(sampleTotal) }} 条样本</span>
          </div>
          <div class="health-steps">
            <article v-for="step in healthSteps" :key="step.key" class="health-step">
              <span>{{ step.level }}</span>
              <strong>{{ step.label }}</strong>
              <b>{{ step.value }}</b>
              <small>{{ step.hint }}</small>
            </article>
          </div>
        </section>

        <div class="metric-grid">
          <article v-for="metric in metrics" :key="metric.key" class="metric-card">
            <span>{{ metric.label }}</span>
            <strong>{{ metric.value }}</strong>
            <small>{{ metric.hint }}</small>
          </article>
        </div>

        <div class="content-grid">
          <section class="panel-block">
            <div class="block-header">
              <div>
                <h4>监督样本池（L2+）</h4>
                <p>默认只看 L2+；复核通过后只沉淀侧路经验，不直接驱动任务、资产、ERP、审单或成本。</p>
              </div>
              <span>{{ integerLabel(effectiveSampleTotal) }} 条</span>
            </div>
            <div v-if="effectiveSamples.length" class="experience-list">
              <article v-for="item in effectiveSamples" :key="`effective-${item.id}`" class="experience-item">
                <div>
                  <span :class="evidenceClass(item.evidence_level)">{{ evidenceLabel(item.evidence_level) }}</span>
                  <strong>{{ item.action || item.source_type }}</strong>
                  <small>{{ item.source_type }} · {{ item.source_id || '未记录来源 ID' }}</small>
                </div>
                <p>{{ feedbackLabel(item.feedback_value) }}{{ item.feedback_reason_code ? ` · ${reasonLabel(item.feedback_reason_code)}` : '' }}</p>
              </article>
            </div>
            <BaseEmptyState v-else title="暂无 L2+ 候选" :description="effectiveEmptyDescription" />
          </section>

          <section class="panel-block">
            <div class="block-header">
              <div>
                <h4>缺口队列</h4>
                <p>按监督信号缺口排查；缺口可重复计数，不等于线性转化率。</p>
              </div>
              <span>{{ integerLabel(totalGapCount) }} 个缺口</span>
            </div>
            <div class="gap-list">
              <article v-for="gap in gapQueue" :key="gap.key" class="gap-item">
                <div>
                  <strong>{{ gap.label }}</strong>
                  <small>{{ gap.hint }}</small>
                </div>
                <b>{{ integerLabel(gap.count) }} / {{ integerLabel(gap.total) }}</b>
              </article>
            </div>
            <p class="block-note">
              原因标签 {{ integerLabel(reasonTagCount) }} / {{ integerLabel(stats?.tag_total) }}；资产质量标签
              {{ integerLabel(stats?.asset_quality_labels) }}。
            </p>
          </section>
        </div>

        <section class="panel-block review-block" aria-labelledby="experience-review-title">
          <div class="block-header">
            <div>
              <h4 id="experience-review-title">候选归因复核</h4>
              <p>Attribution 只生成候选，SuperAdmin 复核后才进入侧路经验候选治理。</p>
            </div>
            <span>{{ integerLabel(reviewItemTotal) }} 个候选</span>
          </div>
          <div v-if="reviewItems.length" class="review-list">
            <article v-for="item in reviewItems" :key="item.item_key" class="review-item">
              <div class="review-main">
                <div class="review-title-row">
                  <span :class="reviewPriorityClass(item.priority)">{{ reviewPriorityLabel(item.priority) }}</span>
                  <strong>{{ reviewCandidateTitle(item) }}</strong>
                </div>
                <p>{{ reviewCandidateMeta(item) }}</p>
                <small>{{ reviewCandidateEvidence(item) }}</small>
              </div>
              <div class="review-actions">
                <BaseButton
                  variant="secondary"
                  size="sm"
                  :loading="reviewBusyKey === `${item.item_key}:approve`"
                  :disabled="!reviewMaterializationEnabled"
                  @click="submitReview(item, 'approve')"
                >
                  {{ reviewMaterializationEnabled ? '确认归因（侧路）' : 'Shadow 观察中' }}
                </BaseButton>
                <BaseButton variant="secondary" size="sm" :loading="reviewBusyKey === `${item.item_key}:needs_more_data`" @click="submitReview(item, 'needs_more_data')">
                  需更多数据
                </BaseButton>
                <BaseButton variant="secondary" size="sm" :loading="reviewBusyKey === `${item.item_key}:reject`" @click="submitReview(item, 'reject')">
                  误报
                </BaseButton>
              </div>
            </article>
          </div>
          <BaseEmptyState v-else :title="reviewEmptyTitle" :description="reviewEmptyDescription" />
          <p v-if="reviewActionError" class="block-note review-error" role="alert">{{ reviewActionError }}</p>
        </section>

        <section class="panel-block">
          <div class="block-header">
            <div>
              <h4>样本表</h4>
              <p>按证据等级分组，避免把展示流水误读成可用经验。</p>
            </div>
            <span>{{ integerLabel(sampleTotal) }} 条</span>
          </div>
          <div v-if="groupedSamples.length" class="sample-groups">
            <section v-for="group in groupedSamples" :key="group.level" class="sample-group">
              <h5>
                <span :class="evidenceClass(group.level)">{{ evidenceLabel(group.level) }}</span>
                <b>{{ integerLabel(group.items.length) }} 条</b>
              </h5>
              <div class="table-scroll">
                <table class="experience-table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>来源</th>
                      <th>动作</th>
                      <th>反馈</th>
                      <th>缺口</th>
                      <th>任务</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="item in group.items" :key="item.id">
                      <td>{{ shortDateTime(item.event_time) }}</td>
                      <td>
                        <strong>{{ item.source_type }}</strong>
                        <small>{{ item.source_id || '-' }}</small>
                      </td>
                      <td>{{ item.action || '-' }}</td>
                      <td>
                        <strong>{{ feedbackLabel(item.feedback_value) }}</strong>
                        <small>{{ item.feedback_reason_code ? reasonLabel(item.feedback_reason_code) : '-' }}</small>
                      </td>
                      <td>{{ missingSignalsLabel(item.missing_signals) }}</td>
                      <td>{{ item.task_id ?? '-' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>
          <BaseEmptyState v-else title="暂无样本" :description="samplesEmptyDescription" />
        </section>
      </template>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import {
  experienceApi,
  type AISuggestionFeedbackValue,
  type ExperienceEvent,
  type ExperienceEvidenceLevel,
  type ExperienceReasonTag,
  type ExperienceReviewDecisionValue,
  type ExperienceReviewItem,
  type ExperienceRuntimeFlags,
  type ExperienceStats,
  type PaginatedEnvelope,
} from '@/services/api/experienceApi'

interface MetricItem {
  key: string
  label: string
  value: string
  hint: string
}

interface HealthStep {
  key: string
  level: ExperienceEvidenceLevel
  label: string
  count: number
  total: number
  value: string
  hint: string
}

interface GapItem {
  key: string
  label: string
  count: number
  total: number
  hint: string
}

const emptyFlags: ExperienceRuntimeFlags = {
  ui_enabled: false,
  capture_enabled: false,
  ai_feedback_enabled: false,
  worker_enabled: false,
  behavior_capture_enabled: false,
  micro_question_enabled: false,
  review_materialization_enabled: false,
  behavior_sample_rate: 0,
}

const evidenceLevels: ExperienceEvidenceLevel[] = ['L4', 'L3', 'L2', 'L1', 'L0']
const evidenceRank: Record<ExperienceEvidenceLevel, number> = { L0: 0, L1: 1, L2: 2, L3: 3, L4: 4 }

const reasonLabels: Record<string, string> = {
  spec_mismatch: '规格不符',
  asset_mismatch: '资产不符',
  stage_not_applicable: '阶段不适用',
  missing_context: '缺上下文',
  outdated: '已过时',
  customer_special_case: '客户特例',
  already_handled: '已处理',
  not_relevant: '不相关',
}

const loading = ref(false)
const error = ref('')
const configFlags = ref<ExperienceRuntimeFlags>(emptyFlags)
const stats = ref<ExperienceStats | null>(null)
const samples = ref<ExperienceEvent[]>([])
const effectiveSamples = ref<ExperienceEvent[]>([])
const sampleTotal = ref(0)
const effectiveSampleTotal = ref(0)
const tags = ref<ExperienceReasonTag[]>([])
const reviewItems = ref<ExperienceReviewItem[]>([])
const reviewItemTotal = ref(0)
const reviewBusyKey = ref('')
const reviewQueueError = ref(false)
const reviewActionError = ref('')

const runtimeFlags = computed(() => stats.value?.flags ?? configFlags.value)
const reviewMaterializationEnabled = computed(() => Boolean(runtimeFlags.value.review_materialization_enabled))
const displayedCount = computed(() => count(stats.value?.displayed_events ?? stats.value?.ai_suggestion_events))
const locatableCount = computed(() => count(stats.value?.locatable_samples))
const locatableDisplayedCount = computed(() => count(stats.value?.locatable_displayed_events ?? Math.min(locatableCount.value, displayedCount.value)))
const feedbackCount = computed(() => count(stats.value?.feedback_samples ?? stats.value?.ai_feedback_events))
const reasonRequiredFeedbackCount = computed(() => count(stats.value?.feedback_partially_accepted) + count(stats.value?.feedback_rejected))
const reasonedCount = computed(() => count(stats.value?.reasoned_feedback_samples))
const reusableCount = computed(() => count(stats.value?.reusable_samples))
const reasonTagCount = computed(() => tags.value.length || count(stats.value?.tag_enabled))

const healthSteps = computed<HealthStep[]>(() => {
  const displayed = displayedCount.value
  const locatable = locatableDisplayedCount.value
  const feedback = feedbackCount.value
  const reasonRequired = reasonRequiredFeedbackCount.value
  const reasoned = Math.min(reasonedCount.value, reasonRequired)
  const reusable = reusableCount.value
  return [
    { key: 'displayed', level: 'L0', label: '建议展示', count: displayed, total: displayed, value: fractionLabel(displayed, displayed), hint: displayed ? '建议已展示' : '未采集或未配置' },
    { key: 'locatable', level: 'L1', label: '展示可定位', count: locatable, total: displayed, value: fractionLabel(locatable, displayed), hint: locatable ? '展示建议能回到任务或资产' : '无匹配数据' },
    { key: 'feedback', level: 'L2', label: '正式反馈', count: feedback, total: displayed, value: fractionLabel(feedback, displayed), hint: feedback ? '已有有用/部分/无用判断' : '监督信号为 0' },
    { key: 'tagged', level: 'L3', label: '反馈原因', count: reasoned, total: reasonRequired, value: fractionLabel(reasoned, reasonRequired), hint: reasoned ? '部分/无用反馈已有原因' : '缺部分/无用反馈原因' },
    { key: 'reusable', level: 'L4', label: '侧路候选', count: reusable, total: displayed, value: `${integerLabel(reusable)} 条`, hint: reusable ? '侧路候选数；不是展示到成交或自动化的转化率' : '暂无候选沉淀' },
  ]
})

const metrics = computed<MetricItem[]>(() => {
  const data = stats.value
  const displayed = displayedCount.value
  const feedback = feedbackCount.value
  const reasonRequired = reasonRequiredFeedbackCount.value
  const reasoned = Math.min(reasonedCount.value, reasonRequired)
  const reusable = reusableCount.value
  return [
    {
      key: 'feedback_rate',
      label: '反馈率',
      value: fractionLabel(feedback, displayed),
      hint: metricRatioHint(feedback, displayed, '不是事件总数'),
    },
    {
      key: 'reason_coverage',
      label: '原因覆盖',
      value: fractionLabel(reasoned, reasonRequired),
      hint: metricRatioHint(reasoned, reasonRequired, '分母仅含部分有用/无用'),
    },
    {
      key: 'reusable_rate',
      label: 'L4 侧路候选数',
      value: integerLabel(reusable),
      hint: '侧路候选沉淀数，不等于转化率或自动化结论',
    },
    {
      key: 'feedback_distribution',
      label: '反馈分布',
      value: `${integerLabel(data?.feedback_accepted)} / ${integerLabel(data?.feedback_partially_accepted)} / ${integerLabel(data?.feedback_rejected)}`,
      hint: feedback < 10 ? '有用 / 部分 / 无用；低样本，仅观察' : '有用 / 部分 / 无用',
    },
    {
      key: 'attribution_shadow',
      label: '归因候选',
      value: `${integerLabel(data?.attribution_positive)} / ${integerLabel(data?.attribution_weak)} / ${integerLabel(data?.attribution_rejected)}`,
      hint:
        count(data?.attribution_total) > 0
          ? `强 / 弱 / 低可信；待复核 ${integerLabel(data?.review_items_open)}`
          : '试算归因尚无候选',
    },
    {
      key: 'micro_question',
      label: '微追问',
      value: `${integerLabel(data?.micro_question_answered)} / ${integerLabel(data?.micro_question_dismissed)} / ${integerLabel(data?.micro_question_rate_limited)}`,
      hint:
        count(data?.micro_question_answers) > 0 || count(data?.micro_question_rate_limited) > 0
          ? `已答 / 跳过 / 今日限流用户；总 ${integerLabel(data?.micro_question_answers)}`
          : '未灰度或暂无回答',
    },
    {
      key: 'outbox',
      label: '后台入账',
      value: integerLabel((data?.outbox_queued ?? 0) + (data?.outbox_processing ?? 0)),
      hint: `${integerLabel(data?.outbox_dead_letter)} 失败队列，24h 失败 ${integerLabel(data?.outbox_failed_24h)}`,
    },
    {
      key: 'capture',
      label: '采集成功',
      value: outbox24hTotal.value > 0 ? percentLabel(data?.capture_success_rate_24h) : '无样本',
      hint:
        outbox24hTotal.value > 0
          ? `24h 处理 ${integerLabel(data?.outbox_processed_24h)} 条，失败 ${integerLabel(data?.outbox_failed_24h)} 条`
          : '无 24h 处理样本，不能解读为 0%',
    },
    {
      key: 'profiles',
      label: '任务画像',
      value: integerLabel(data?.task_profiles),
      hint: data?.latest_profile_rebuilt_at ? shortDateTime(data.latest_profile_rebuilt_at) : '未生成或未运行',
    },
    {
      key: 'samples',
      label: '样本口径',
      value: integerLabel(data?.sample_total ?? sampleTotal.value),
      hint: '包含 L0-L4，不等于可用经验',
    },
  ]
})

const gapQueue = computed<GapItem[]>(() => {
  const displayed = displayedCount.value
  const locatable = locatableCount.value
  const feedback = feedbackCount.value
  const reasonRequired = reasonRequiredFeedbackCount.value
  const reasoned = Math.min(reasonedCount.value, reasonRequired)
  const profiles = count(stats.value?.task_profiles)
  const assetLabels = count(stats.value?.asset_quality_labels)
  return [
    {
      key: 'missing_feedback',
      label: '缺反馈',
      count: Math.max(0, displayed - feedback),
      total: displayed,
      hint: displayed ? '建议已展示但没有人工判断' : '未采集、未配置或真实为 0',
    },
    {
      key: 'missing_reason',
      label: '缺标签',
      count: Math.max(0, reasonRequired - reasoned),
      total: reasonRequired,
      hint: reasonRequired ? '负向/部分反馈缺原因 chip' : '没有部分/无用反馈需要打标',
    },
    {
      key: 'missing_profile',
      label: '缺画像',
      count: Math.max(0, locatable - profiles),
      total: locatable,
      hint: locatable ? '可定位样本缺任务画像' : '还没有可定位样本',
    },
    {
      key: 'missing_asset_quality',
      label: '缺资产质量',
      count: assetLabels > 0 ? 0 : displayed,
      total: displayed,
      hint: assetLabels > 0 ? '已有资产质量标签' : '第一阶段未采到质量标签',
    },
  ]
})

const totalGapCount = computed(() => gapQueue.value.reduce((sum, item) => sum + item.count, 0))
const workerRuns = computed(() => stats.value?.worker_last_runs ?? [])
const outbox24hTotal = computed(() => count(stats.value?.outbox_processed_24h) + count(stats.value?.outbox_failed_24h))
const failedWorkerCount = computed(() => workerRuns.value.filter((run) => run.status === 'failed' || run.status === 'partial' || count(run.failed_count) > 0).length)

const workerHealthLabel = computed(() => {
  if (!runtimeFlags.value.worker_enabled) return 'Worker 关闭'
  if (!workerRuns.value.length) return '无运行记录'
  if (failedWorkerCount.value > 0) return `${integerLabel(failedWorkerCount.value)} 个失败`
  return '最近正常'
})

const workerEmptyDescription = computed(() => {
  if (!runtimeFlags.value.worker_enabled) return 'Worker 开关未开启，因此不会产生后台运行记录。'
  if (displayedCount.value === 0 && sampleTotal.value === 0) return '可能是刚启用、未采集，或当前环境尚未产生经验样本。'
  return '没有查到最近 worker_runs，需检查后台 worker 是否部署或已启动。'
})

const groupedSamples = computed(() => {
  return evidenceLevels
    .map((level) => ({
      level,
      items: samples.value.filter((item) => normalizeEvidenceLevel(item.evidence_level) === level),
    }))
    .filter((group) => group.items.length > 0)
})

const effectiveEmptyDescription = computed(() => {
  if (!runtimeFlags.value.ai_feedback_enabled) return 'AI 反馈开关未开启，当前不会采集监督信号。'
  if (displayedCount.value === 0) return '未采集、未配置，或当前筛选下真实为 0。'
  if (feedbackCount.value === 0) return '当前只有展示流水，还没有人工反馈，因此不能作为有效经验。'
  return '当前无匹配 L2+ 样本，可能是缺少原因标签、画像或资产质量信号。'
})

const samplesEmptyDescription = computed(() => {
  if (!runtimeFlags.value.capture_enabled) return '采集开关未开启，样本表不会新增记录。'
  return '未采集、未配置、无匹配数据，或当前真实为 0。'
})

const reviewEmptyTitle = computed(() => (reviewQueueError.value ? '复核队列暂不可用' : '暂无待复核候选'))
const reviewEmptyDescription = computed(() =>
  reviewQueueError.value
    ? '主指标已加载；复核队列接口暂时失败，不能解读为真实无候选。'
    : '归因 worker 尚未生成 open 候选，或当前证据不足以进入人工复核。',
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const configRes = await experienceApi.config()
    configFlags.value = configRes.data?.data ?? emptyFlags
    reviewItems.value = []
    reviewItemTotal.value = 0
    reviewQueueError.value = false
    reviewActionError.value = ''
    if (!configFlags.value.ui_enabled) {
      stats.value = null
      samples.value = []
      effectiveSamples.value = []
      sampleTotal.value = 0
      effectiveSampleTotal.value = 0
      tags.value = []
      return
    }

    const [statsRes, samplesRes, effectiveSamplesRes] = await Promise.all([
      experienceApi.stats(),
      experienceApi.samples({ page: 1, page_size: 20 }),
      experienceApi.samples({ page: 1, page_size: 20, min_evidence_level: 'L2' }),
    ])
    stats.value = statsRes.data?.data ?? null
    const parsedSamples = samplesRes.data as PaginatedEnvelope<ExperienceEvent>
    const parsedEffectiveSamples = effectiveSamplesRes.data as PaginatedEnvelope<ExperienceEvent>
    samples.value = parsedSamples.data ?? []
    effectiveSamples.value = parsedEffectiveSamples.data ?? []
    sampleTotal.value = Number(parsedSamples.pagination?.total ?? samples.value.length)
    effectiveSampleTotal.value = Number(parsedEffectiveSamples.pagination?.total ?? effectiveSamples.value.length)
    try {
      const tagsRes = await experienceApi.reasonTags({ scene: 'ai_suggestion_feedback' })
      tags.value = tagsRes.data?.data ?? []
    } catch {
      tags.value = tags.value.length ? tags.value : []
    }
    try {
      const reviewRes = await experienceApi.reviewItems({
        status: 'open',
        item_type: 'attribution_candidate',
        page: 1,
        page_size: 8,
      })
      const parsedReviewItems = reviewRes.data as PaginatedEnvelope<ExperienceReviewItem>
      reviewItems.value = parsedReviewItems.data ?? []
      reviewItemTotal.value = Number(parsedReviewItems.pagination?.total ?? reviewItems.value.length)
      reviewQueueError.value = false
    } catch {
      reviewItems.value = []
      reviewItemTotal.value = 0
      reviewQueueError.value = true
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载经验观测失败'
  } finally {
    loading.value = false
  }
}

async function submitReview(item: ExperienceReviewItem, decision: ExperienceReviewDecisionValue) {
  if (!item.item_key || reviewBusyKey.value) return
  const confirmed =
    decision !== 'approve' ||
    (typeof window !== 'undefined' &&
      window.confirm('确认后会写入侧路经验候选，不会修改任务、资产、ERP 或审核状态。是否继续？'))
  if (!confirmed) return
  const busyKey = `${item.item_key}:${decision}`
  reviewBusyKey.value = busyKey
  reviewActionError.value = ''
  try {
    await experienceApi.reviewDecision(item.item_key, {
      decision,
      reason_code: reviewDecisionReasonCode(decision),
      payload: {
        surface: 'data_center_experience',
        item_type: item.item_type,
        review_confirmation: decision === 'approve',
      },
    })
    await load()
  } catch {
    reviewActionError.value = '复核结果未保存，请稍后重试。'
  } finally {
    if (reviewBusyKey.value === busyKey) {
      reviewBusyKey.value = ''
    }
  }
}

function reviewDecisionReasonCode(decision: ExperienceReviewDecisionValue): string {
  if (decision === 'approve') return 'verified'
  if (decision === 'reject') return 'misattributed'
  return 'insufficient_evidence'
}

function count(value: unknown): number {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n < 0) return 0
  return Math.round(n)
}

function integerLabel(value: unknown): string {
  return count(value).toLocaleString('zh-CN')
}

function fractionLabel(numerator: unknown, denominator: unknown): string {
  return `${integerLabel(numerator)} / ${integerLabel(denominator)}`
}

function percentFromCounts(numerator: unknown, denominator: unknown): string {
  const total = count(denominator)
  if (total <= 0) return '无分母'
  return `${Math.round((count(numerator) / total) * 1000) / 10}%`
}

function percentLabel(value: unknown): string {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n < 0) return '无样本'
  return `${Math.round(n * 1000) / 10}%`
}

function metricRatioHint(numerator: unknown, denominator: unknown, base: string): string {
  const total = count(denominator)
  if (total <= 0) return `无分母：未开始采集、筛选无样本或真实为 0；${base}`
  if (total < 30) return `${percentFromCounts(numerator, denominator)}，低样本，仅观察；${base}`
  return `${percentFromCounts(numerator, denominator)}，${base}`
}

function flagLabel(enabled: boolean): string {
  return enabled ? '开' : '关'
}

function flagClass(enabled: boolean): string {
  return enabled ? 'status-pill status-pill--on' : 'status-pill'
}

function normalizeEvidenceLevel(level?: string): ExperienceEvidenceLevel {
  if (level && level in evidenceRank) return level as ExperienceEvidenceLevel
  return 'L0'
}

function evidenceLabel(level?: string): string {
  const normalized = normalizeEvidenceLevel(level)
  const labels: Record<ExperienceEvidenceLevel, string> = {
    L0: 'L0 displayed',
    L1: 'L1 locatable',
    L2: 'L2 feedback',
    L3: 'L3 tagged',
    L4: 'L4 侧路候选',
  }
  return labels[normalized]
}

function evidenceClass(level?: string): string {
  return `evidence-badge evidence-badge--${normalizeEvidenceLevel(level).toLowerCase()}`
}

function feedbackLabel(value?: AISuggestionFeedbackValue): string {
  if (value === 'accepted') return '有用'
  if (value === 'partially_accepted') return '部分有用'
  if (value === 'rejected') return '无用'
  return '无反馈'
}

function reasonLabel(code?: string): string {
  if (!code) return '-'
  return reasonLabels[code] ?? code
}

function missingSignalsLabel(signals?: string[]): string {
  if (!signals?.length) return '-'
  return signals.map((item) => signalLabel(item)).join('、')
}

function signalLabel(signal: string): string {
  const labels: Record<string, string> = {
    feedback: '缺反馈',
    reason: '缺标签',
    target: '缺定位',
    locatable_target: '缺定位',
    profile: '缺画像',
    asset_quality: '缺资产质量',
  }
  return labels[signal] ?? signal
}

function workerNameLabel(name?: string): string {
  const labels: Record<string, string> = {
    outcome_observer: '结果观察',
    outbox: '后台入账',
    attribution: '归因计算',
    retention: '保留清理',
  }
  return labels[name || ''] ?? (name || 'Worker')
}

function workerStatusLabel(status?: string): string {
  if (status === 'success') return '正常'
  if (status === 'partial') return '部分失败'
  if (status === 'failed') return '失败'
  return status || '未知'
}

function workerStatusClass(status?: string): string {
  if (status === 'success') return 'worker-status worker-status--success'
  if (status === 'partial') return 'worker-status worker-status--partial'
  if (status === 'failed') return 'worker-status worker-status--failed'
  return 'worker-status'
}

function reviewPriorityLabel(priority?: string): string {
  if (priority === 'high') return '高优先'
  if (priority === 'low') return '低优先'
  return '中优先'
}

function reviewPriorityClass(priority?: string): string {
  if (priority === 'high') return 'review-priority review-priority--high'
  if (priority === 'low') return 'review-priority review-priority--low'
  return 'review-priority'
}

function reviewCandidateTitle(item: ExperienceReviewItem): string {
  const summary = asRecord(item.evidence_summary)
  const outcome = asRecord(summary.outcome)
  const suggestion = asRecord(summary.suggestion)
  const action = stringValue(outcome.action || outcome.outcome || item.item_type)
  const target = [stringValue(suggestion.target_type), stringValue(suggestion.target_id)].filter(Boolean).join(' ')
  return target ? `${action} · ${target}` : action || '候选归因'
}

function reviewCandidateMeta(item: ExperienceReviewItem): string {
  const summary = asRecord(item.evidence_summary)
  const status = stringValue(summary.status) || item.status
  const confidence = stringValue(summary.confidence) || '-'
  const score = Number(summary.score ?? 0)
  const gap = Number(summary.time_gap_hours ?? 0)
  const scoreLabel = Number.isFinite(score) && score > 0 ? `评分 ${Math.round(score * 100) / 100}` : '评分 -'
  const gapLabel = Number.isFinite(gap) && gap > 0 ? `${Math.round(gap * 10) / 10}h` : '-'
  return `${reviewCandidateStatusLabel(status)} · ${reviewConfidenceLabel(confidence)} · ${scoreLabel} · 间隔 ${gapLabel}`
}

function reviewCandidateStatusLabel(status?: string): string {
  if (status === 'positive_candidate') return '强候选'
  if (status === 'weak_candidate') return '弱候选'
  if (status === 'rejected_candidate') return '低可信候选'
  if (status === 'open') return '待复核'
  if (status === 'approved') return '已确认'
  if (status === 'rejected') return '已驳回'
  if (status === 'needs_more_data') return '需补信号'
  return status || '待复核'
}

function reviewConfidenceLabel(confidence?: string): string {
  if (confidence === 'high') return '高置信'
  if (confidence === 'medium') return '中置信'
  if (confidence === 'low') return '低置信'
  return '置信未知'
}

function reviewCandidateEvidence(item: ExperienceReviewItem): string {
  const summary = asRecord(item.evidence_summary)
  const behavior = asRecord(summary.behavior)
  const feedback = asRecord(summary.feedback)
  const outcome = asRecord(summary.outcome)
  const changed = Array.isArray(outcome.changed_fields) ? outcome.changed_fields.length : 0
  return `行为次数 ${integerLabel(behavior.count)} / 行为分 ${integerLabel(behavior.score)}；反馈 ${feedbackLabel(stringValue(feedback.value) as AISuggestionFeedbackValue)}；字段变化 ${integerLabel(changed)}`
}

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return {}
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function shortDateTime(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return `${d.toLocaleDateString('zh-CN')} ${d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
}

onMounted(load)
</script>

<style scoped>
.experience-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  color: rgb(var(--yb-text-navy));
  font-family: var(--yb-font-text);
  letter-spacing: 0;
}

.panel-header,
.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.panel-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 750;
  line-height: 1.3;
  letter-spacing: 0;
}

.panel-subtitle,
.block-header p,
.block-note {
  margin: 0.2rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  letter-spacing: 0;
}

.panel-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
}

.panel-time {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.72rem;
  white-space: nowrap;
}

.button-icon {
  margin-right: 0.3rem;
  width: 0.9rem;
  height: 0.9rem;
}

.flag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.8rem;
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 999px;
  background: rgb(var(--yb-surface));
  padding: 0.25rem 0.65rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: rgb(var(--yb-text-muted-strong));
  white-space: nowrap;
}

.status-pill--on {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-text));
}

.panel-block,
.metric-card {
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 0.35rem 1.1rem rgb(var(--yb-shadow-blue) / 0.06);
}

.panel-block {
  min-width: 0;
  padding: 0.75rem;
}

.block-header {
  align-items: flex-start;
  margin-bottom: 0.65rem;
}

.block-header h4 {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 750;
  letter-spacing: 0;
}

.block-header > span {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-blue-gray));
  font-size: 0.72rem;
}

.health-steps {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.5rem;
}

.health-step {
  display: grid;
  gap: 0.16rem;
  min-height: 5.4rem;
  border: 1px solid rgb(var(--yb-border-blue) / 0.75);
  border-radius: 0.45rem;
  background: rgb(var(--yb-surface-soft));
  padding: 0.55rem;
}

.health-step span,
.metric-card span,
.metric-card small,
.experience-item small,
.gap-item small,
.experience-table td small {
  color: rgb(var(--yb-text-blue-gray));
  font-size: 0.72rem;
  letter-spacing: 0;
}

.health-step strong,
.gap-item strong,
.experience-item strong,
.experience-table td strong {
  color: rgb(var(--yb-text-deep));
  font-weight: 750;
  line-height: 1.3;
}

.health-step b {
  color: rgb(var(--yb-brand-strong));
  font-size: 0.95rem;
}

.health-step small {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.68rem;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
  gap: 0.55rem;
}

.metric-card {
  display: flex;
  min-height: 4.75rem;
  flex-direction: column;
  justify-content: center;
  gap: 0.15rem;
  padding: 0.65rem 0.75rem;
}

.metric-card strong {
  color: rgb(var(--yb-text-deep));
  font-size: 1.18rem;
  line-height: 1.2;
  letter-spacing: 0;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(18rem, 0.85fr);
  gap: 0.75rem;
}

.experience-list,
.gap-list,
.review-list,
.sample-groups,
.worker-run-list {
  display: grid;
  gap: 0.45rem;
}

.experience-item,
.gap-item,
.review-item,
.worker-run-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgb(var(--yb-border-blue) / 0.72);
  border-radius: 0.45rem;
  padding: 0.55rem 0.6rem;
}

.worker-run-item {
  min-height: 3.2rem;
}

.worker-run-main {
  display: grid;
  gap: 0.16rem;
  min-width: 0;
}

.worker-run-main strong,
.worker-run-main small {
  display: block;
  min-width: 0;
}

.worker-run-main strong {
  overflow: hidden;
  color: rgb(var(--yb-text-deep));
  font-size: 0.82rem;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.worker-run-main small {
  overflow: hidden;
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.72rem;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.review-item {
  align-items: flex-start;
}

.review-main {
  display: grid;
  gap: 0.22rem;
  min-width: 0;
}

.review-title-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.review-title-row strong {
  overflow: hidden;
  color: rgb(var(--yb-text-deep));
  font-size: 0.82rem;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.review-main p,
.review-main small {
  margin: 0;
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.72rem;
}

.review-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.35rem;
}

.review-priority {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 999px;
  background: rgb(var(--yb-brand-soft));
  padding: 0.08rem 0.42rem;
  color: rgb(var(--yb-brand-strong));
  font-size: 0.66rem;
  font-weight: 800;
  white-space: nowrap;
}

.review-priority--high {
  border-color: rgb(var(--yb-warning-border));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
}

.review-priority--low {
  border-color: rgb(var(--yb-border-blue));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text-muted));
}

.experience-item p {
  margin: 0;
  color: rgb(var(--yb-text-muted));
  font-size: 0.72rem;
  font-weight: 700;
  text-align: right;
}

.gap-item b {
  flex: 0 0 auto;
  color: rgb(var(--yb-brand-strong));
  font-size: 0.8rem;
}

.worker-run-meta {
  display: grid;
  justify-items: end;
  gap: 0.2rem;
  min-width: 10rem;
  text-align: right;
}

.worker-status {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 999px;
  background: rgb(var(--yb-surface-soft));
  padding: 0.08rem 0.45rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.66rem;
  font-weight: 800;
}

.worker-status--success {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-text));
}

.worker-status--partial {
  border-color: rgb(var(--yb-warning-border));
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning-text));
}

.worker-status--failed {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}

.block-note {
  margin-top: 0.65rem;
}

.review-error {
  color: rgb(var(--yb-danger-text));
  font-weight: 700;
}

.evidence-badge {
  display: inline-flex;
  align-items: center;
  width: max-content;
  min-height: 1.35rem;
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 999px;
  background: rgb(var(--yb-surface-soft));
  padding: 0.08rem 0.42rem;
  color: rgb(var(--yb-text-muted));
  font-size: 0.66rem;
  font-weight: 800;
  white-space: nowrap;
}

.evidence-badge--l2,
.evidence-badge--l3 {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.evidence-badge--l4 {
  border-color: rgb(var(--yb-success-border));
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-text));
}

.sample-group {
  display: grid;
  gap: 0.45rem;
}

.sample-group h5 {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin: 0;
}

.sample-group h5 b {
  color: rgb(var(--yb-text-blue-gray));
  font-size: 0.72rem;
}

.table-scroll {
  overflow-x: auto;
}

.experience-table {
  width: 100%;
  min-width: 54rem;
  border-collapse: collapse;
  font-size: 0.75rem;
}

.experience-table th,
.experience-table td {
  border-bottom: 1px solid rgb(var(--yb-border-blue) / 0.72);
  padding: 0.55rem 0.45rem;
  text-align: left;
  vertical-align: top;
}

.experience-table th {
  color: rgb(var(--yb-text-blue-gray));
  font-weight: 700;
}

@media (max-width: 1100px) {
  .health-steps {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 920px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .panel-header,
  .panel-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .metric-grid,
  .health-steps {
    grid-template-columns: 1fr;
  }

  .experience-item,
  .gap-item,
  .review-item,
  .worker-run-item {
    align-items: flex-start;
    flex-direction: column;
  }

  .experience-item p {
    text-align: left;
  }

  .worker-run-meta {
    justify-items: start;
    min-width: 0;
    text-align: left;
  }

  .review-actions {
    justify-content: flex-start;
  }
}
</style>
