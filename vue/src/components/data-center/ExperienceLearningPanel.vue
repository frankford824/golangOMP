<template>
  <section class="experience-panel">
    <div class="panel-header">
      <div>
        <h3 class="panel-title">经验观测</h3>
        <p class="panel-subtitle">闭环健康、有效经验池、缺口队列与可复核样本</p>
      </div>
      <div class="panel-actions">
        <span v-if="stats?.generated_at" class="panel-time">统计 {{ shortDateTime(stats.generated_at) }}</span>
        <BaseButton variant="secondary" size="sm" :loading="loading" @click="load">
          <RefreshCw class="button-icon" aria-hidden="true" />
          刷新
        </BaseButton>
      </div>
    </div>

    <BaseErrorState v-if="error" :title="error" @retry="load" />

    <template v-else>
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
          <span :class="flagClass(runtimeFlags.ui_enabled)">页面 {{ flagLabel(runtimeFlags.ui_enabled) }}</span>
        </div>

        <section class="panel-block health-block" aria-labelledby="experience-health-title">
          <div class="block-header">
            <div>
              <h4 id="experience-health-title">闭环健康条</h4>
              <p>展示 -> 可定位 -> 有反馈 -> 有标签 -> 可复用</p>
            </div>
            <span>{{ integerLabel(sampleTotal) }} 条样本</span>
          </div>
          <div class="health-steps">
            <article v-for="step in healthSteps" :key="step.key" class="health-step">
              <span>{{ step.level }}</span>
              <strong>{{ step.label }}</strong>
              <b>{{ integerLabel(step.count) }} / {{ integerLabel(step.total) }}</b>
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
                <h4>有效经验池</h4>
                <p>默认只看 L2+，可复核后再进入自动化运营计算。</p>
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
            <BaseEmptyState v-else title="暂无有效经验" :description="effectiveEmptyDescription" />
          </section>

          <section class="panel-block">
            <div class="block-header">
              <div>
                <h4>缺口队列</h4>
                <p>优先补能把 displayed 变成 reusable 的监督信号。</p>
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

const runtimeFlags = computed(() => stats.value?.flags ?? configFlags.value)
const displayedCount = computed(() => count(stats.value?.displayed_events ?? stats.value?.ai_suggestion_events))
const locatableCount = computed(() => count(stats.value?.locatable_samples))
const feedbackCount = computed(() => count(stats.value?.feedback_samples ?? stats.value?.ai_feedback_events))
const reasonedCount = computed(() => count(stats.value?.reasoned_feedback_samples))
const reusableCount = computed(() => count(stats.value?.reusable_samples))
const reasonTagCount = computed(() => tags.value.length || count(stats.value?.tag_enabled))

const healthSteps = computed<HealthStep[]>(() => {
  const displayed = displayedCount.value
  const locatable = locatableCount.value
  const feedback = feedbackCount.value
  const reasoned = reasonedCount.value
  const reusable = reusableCount.value
  return [
    { key: 'displayed', level: 'L0', label: '展示', count: displayed, total: displayed, hint: displayed ? '建议已展示' : '未采集或未配置' },
    { key: 'locatable', level: 'L1', label: '可定位', count: locatable, total: displayed, hint: locatable ? '能回到任务或资产' : '无匹配数据' },
    { key: 'feedback', level: 'L2', label: '有反馈', count: feedback, total: displayed, hint: feedback ? '已有人工判断' : '监督信号为 0' },
    { key: 'tagged', level: 'L3', label: '有标签', count: reasoned, total: feedback, hint: reasoned ? '反馈已有原因' : '缺原因标签' },
    { key: 'reusable', level: 'L4', label: '可复用', count: reusable, total: displayed, hint: reusable ? '可进入经验池' : '暂不可复用' },
  ]
})

const metrics = computed<MetricItem[]>(() => {
  const data = stats.value
  const displayed = displayedCount.value
  const feedback = feedbackCount.value
  const reasoned = reasonedCount.value
  const reusable = reusableCount.value
  return [
    {
      key: 'feedback_rate',
      label: '反馈率',
      value: fractionLabel(feedback, displayed),
      hint: `${percentFromCounts(feedback, displayed)}，不是事件总数`,
    },
    {
      key: 'reason_coverage',
      label: '原因覆盖',
      value: fractionLabel(reasoned, feedback),
      hint: `${percentFromCounts(reasoned, feedback)}，部分/无用优先补齐`,
    },
    {
      key: 'reusable_rate',
      label: '可复用率',
      value: fractionLabel(reusable, displayed),
      hint: `${percentLabel(data?.reusable_rate)}，L4 才能沉淀经验`,
    },
    {
      key: 'feedback_distribution',
      label: '反馈分布',
      value: `${integerLabel(data?.feedback_accepted)} / ${integerLabel(data?.feedback_partially_accepted)} / ${integerLabel(data?.feedback_rejected)}`,
      hint: '有用 / 部分 / 无用',
    },
    {
      key: 'outbox',
      label: 'Outbox',
      value: integerLabel((data?.outbox_queued ?? 0) + (data?.outbox_processing ?? 0)),
      hint: `${integerLabel(data?.outbox_dead_letter)} dead-letter，24h 失败 ${integerLabel(data?.outbox_failed_24h)}`,
    },
    {
      key: 'capture',
      label: '采集成功',
      value: percentLabel(data?.capture_success_rate_24h),
      hint: `24h 处理 ${integerLabel(data?.outbox_processed_24h)} 条`,
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
  const reasoned = reasonedCount.value
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
      count: Math.max(0, feedback - reasoned),
      total: feedback,
      hint: feedback ? '负向/部分反馈缺原因 chip' : '没有反馈可打标',
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const configRes = await experienceApi.config()
    configFlags.value = configRes.data?.data ?? emptyFlags
    stats.value = null
    samples.value = []
    effectiveSamples.value = []
    sampleTotal.value = 0
    effectiveSampleTotal.value = 0
    tags.value = []
    if (!configFlags.value.ui_enabled) return

    const [statsRes, samplesRes, effectiveSamplesRes, tagsRes] = await Promise.all([
      experienceApi.stats(),
      experienceApi.samples({ page: 1, page_size: 20 }),
      experienceApi.samples({ page: 1, page_size: 20, min_evidence_level: 'L2' }),
      experienceApi.reasonTags({ scene: 'ai_suggestion_feedback' }),
    ])
    stats.value = statsRes.data?.data ?? null
    const parsedSamples = samplesRes.data as PaginatedEnvelope<ExperienceEvent>
    const parsedEffectiveSamples = effectiveSamplesRes.data as PaginatedEnvelope<ExperienceEvent>
    samples.value = parsedSamples.data ?? []
    effectiveSamples.value = parsedEffectiveSamples.data ?? []
    sampleTotal.value = Number(parsedSamples.pagination?.total ?? samples.value.length)
    effectiveSampleTotal.value = Number(parsedEffectiveSamples.pagination?.total ?? effectiveSamples.value.length)
    tags.value = tagsRes.data?.data ?? []
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载经验观测失败'
  } finally {
    loading.value = false
  }
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
  if (total <= 0) return '0%'
  return `${Math.round((count(numerator) / total) * 1000) / 10}%`
}

function percentLabel(value: unknown): string {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n <= 0) return '0%'
  return `${Math.round(n * 1000) / 10}%`
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
    L4: 'L4 reusable',
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
.sample-groups {
  display: grid;
  gap: 0.45rem;
}

.experience-item,
.gap-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgb(var(--yb-border-blue) / 0.72);
  border-radius: 0.45rem;
  padding: 0.55rem 0.6rem;
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

.block-note {
  margin-top: 0.65rem;
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
  .gap-item {
    align-items: flex-start;
    flex-direction: column;
  }

  .experience-item p {
    text-align: left;
  }
}
</style>
