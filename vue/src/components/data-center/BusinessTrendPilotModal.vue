<template>
  <BaseModal
    v-model="modalOpen"
    title="业务热点 AI 试验"
    :show-confirm="false"
    panel-class="max-w-6xl"
  >
    <section class="trend-pilot-modal">
      <header class="trend-hero">
        <div>
          <span>零现金验证版</span>
          <h3>{{ result?.headline || '近期业务热点判断' }}</h3>
          <p>{{ result?.overview || rangeLabel }}</p>
        </div>
        <div class="trend-confidence" :class="`trend-confidence--${confidenceLevel}`">
          {{ confidenceLabel }}
        </div>
      </header>

      <div class="trend-controls">
        <div class="control-group">
          <span class="control-label">时间</span>
          <div class="segmented">
            <button
              v-for="item in rangeOptions"
              :key="item.value"
              type="button"
              :class="{ active: rangeDays === item.value }"
              :disabled="loading || deepBusy"
              @click="rangeDays = item.value"
            >
              {{ item.label }}
            </button>
          </div>
        </div>
        <div class="control-group">
          <span class="control-label">范围</span>
          <div class="segmented segmented--wide">
            <button type="button" :class="{ active: mode === 'internal' }" :disabled="loading || deepBusy" @click="mode = 'internal'">
              仅内部任务
            </button>
            <button type="button" :class="{ active: mode === 'external' }" :disabled="loading || deepBusy" @click="mode = 'external'">
              内部 + 免费热点
            </button>
          </div>
        </div>
        <BaseButton variant="primary" size="sm" :loading="loading" @click="generate">
          <Sparkles class="button-icon" aria-hidden="true" />
          生成分析
        </BaseButton>
        <BaseButton
          variant="secondary"
          size="sm"
          :loading="deepStarting || deepBusy"
          :disabled="loading || !result"
          @click="startDeepAnalysis"
        >
          <Sparkles class="button-icon" aria-hidden="true" />
          深度分析
        </BaseButton>
      </div>

      <div v-if="sourceStatuses.length" class="source-strip" aria-label="来源状态">
        <span
          v-for="source in sourceStatuses"
          :key="`${source.source}-${source.status}`"
          class="source-pill"
          :class="`source-pill--${sourceStatusClass(source.status)}`"
        >
          <strong>{{ source.source }}</strong>
          <small>{{ sourceStatusText(source) }}</small>
        </span>
      </div>

      <div v-if="deepJob || deepError" class="deep-status" :class="`deep-status--${deepStatusClass}`">
        <div>
          <strong>{{ deepStatusTitle }}</strong>
          <p>{{ deepStatusMessage }}</p>
        </div>
        <BaseButton v-if="deepJob?.status === 'failed'" variant="secondary" size="sm" @click="startDeepAnalysis">
          重试
        </BaseButton>
      </div>

      <div v-if="loading" class="trend-loading" role="status">
        <div class="trend-loading-dot" aria-hidden="true" />
        <div>
          <strong>正在生成分析</strong>
          <p>正在汇总近期任务和可用热点样本。</p>
        </div>
      </div>

      <div v-else-if="error" class="trend-error">
        <p>{{ error }}</p>
        <BaseButton size="sm" variant="primary" @click="generate">重新生成</BaseButton>
      </div>

      <div v-else-if="result" class="trend-content">
        <div class="trend-tabs" role="tablist" aria-label="分析内容">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            :class="{ active: activeTab === tab.key }"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>

        <section v-if="activeTab === 'hotspots'" class="trend-grid">
          <article v-for="item in hotspots" :key="item.topic" class="trend-panel">
            <div class="panel-head">
              <h4>{{ item.topic }}</h4>
              <span>{{ item.count || 0 }} 条</span>
            </div>
            <p>{{ item.signal || '近期任务有集中需求' }}</p>
            <div v-if="item.keywords?.length" class="keyword-row">
              <small v-for="keyword in item.keywords" :key="keyword">{{ keyword }}</small>
            </div>
            <ul v-if="item.task_samples?.length" class="compact-list">
              <li v-for="sample in item.task_samples" :key="sample">{{ sample }}</li>
            </ul>
          </article>
          <BaseEmptyState
            v-if="!hotspots.length"
            title="暂无明显热点"
            description="当前范围内任务样本较少。"
          />
        </section>

        <section v-else-if="activeTab === 'matches'" class="trend-grid">
          <article v-for="item in matches" :key="`${item.source}-${item.topic}`" class="trend-panel">
            <div class="panel-head">
              <h4>{{ item.topic }}</h4>
              <span>{{ item.source }}</span>
            </div>
            <p>{{ item.business_meaning || item.signal || '可作为业务判断参考' }}</p>
            <ul v-if="item.evidence?.length" class="compact-list">
              <li v-for="line in item.evidence" :key="line">{{ line }}</li>
            </ul>
          </article>
          <BaseEmptyState
            v-if="!matches.length"
            title="暂无外部匹配"
            description="本次主要基于内部任务判断。"
          />
        </section>

        <section v-else-if="activeTab === 'directions'" class="trend-grid">
          <article v-for="item in directions" :key="item.title" class="trend-panel trend-panel--direction">
            <div class="panel-head">
              <h4>{{ item.title }}</h4>
              <span>{{ priorityText(item.priority) }}</span>
            </div>
            <p>{{ item.reason || '近期任务有相关信号' }}</p>
            <strong>{{ item.suggested_action || '整理样本并继续观察转化情况' }}</strong>
          </article>
          <article v-for="risk in risks" :key="risk.title" class="trend-panel trend-panel--risk">
            <div class="panel-head">
              <h4>{{ risk.title }}</h4>
              <span>{{ riskLevelText(risk.level) }}</span>
            </div>
            <p>{{ risk.reason || '需要持续观察' }}</p>
          </article>
        </section>

        <section v-else class="evidence-list">
          <article v-for="item in evidenceSamples" :key="`${item.source || ''}-${item.task_no || item.note}`">
            <div>
              <strong>{{ item.task_no || item.source || '样本' }}</strong>
              <span>{{ [item.task_name, item.created_at].filter(Boolean).join(' · ') }}</span>
            </div>
            <p>{{ item.note }}</p>
          </article>
          <BaseEmptyState
            v-if="!evidenceSamples.length"
            title="暂无样本"
            description="当前结果没有可展示的样本。"
          />
        </section>
      </div>

      <div v-else class="trend-idle">
        <Sparkles class="idle-icon" aria-hidden="true" />
        <strong>{{ rangeLabel }}</strong>
        <p>内部任务可直接分析，外部热点按配置启用。</p>
      </div>
    </section>

    <template #footer>
      <footer class="trend-footer">
        <span>{{ result?.generated_at ? `生成于 ${formatTime(result.generated_at)}` : rangeLabel }}</span>
        <div>
          <BaseButton variant="secondary" size="sm" @click="modalOpen = false">关闭</BaseButton>
          <BaseButton variant="primary" size="sm" :loading="loading" @click="generate">
            {{ result ? '重新生成' : '生成分析' }}
          </BaseButton>
        </div>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import {
  reportsApi,
  type BusinessTrendDeepAnalysisJob,
  type BusinessTrendEvidenceSample,
  type BusinessTrendHotspot,
  type BusinessTrendMatch,
  type BusinessTrendPilotParams,
  type BusinessTrendPilotResponse,
} from '@/services/api/reportsApi'

type TrendTab = 'hotspots' | 'matches' | 'directions' | 'evidence'
type RangeDays = 7 | 14 | 30
type TrendMode = 'internal' | 'external'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

const modalOpen = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const rangeOptions: Array<{ label: string; value: RangeDays }> = [
  { label: '近 7 天', value: 7 },
  { label: '近 14 天', value: 14 },
  { label: '近 30 天', value: 30 },
]
const tabs: Array<{ key: TrendTab; label: string }> = [
  { key: 'hotspots', label: '热点主题' },
  { key: 'matches', label: '外部匹配' },
  { key: 'directions', label: '业务建议' },
  { key: 'evidence', label: '证据来源' },
]

const rangeDays = ref<RangeDays>(7)
const mode = ref<TrendMode>('internal')
const activeTab = ref<TrendTab>('hotspots')
const loading = ref(false)
const error = ref('')
const result = ref<BusinessTrendPilotResponse | null>(null)
const deepStarting = ref(false)
const deepError = ref('')
const deepJob = ref<BusinessTrendDeepAnalysisJob | null>(null)
let currentController: AbortController | null = null
let deepController: AbortController | null = null
let deepPollTimer: number | null = null

const rangeStart = computed(() => {
  const d = new Date()
  d.setDate(d.getDate() - rangeDays.value + 1)
  d.setHours(0, 0, 0, 0)
  return d
})
const rangeEnd = computed(() => {
  const d = new Date()
  d.setHours(23, 59, 59, 999)
  return d
})
const rangeLabel = computed(() => `${dateOnly(rangeStart.value)} 至 ${dateOnly(rangeEnd.value)}`)
const hotspots = computed<BusinessTrendHotspot[]>(() => result.value?.internal_hotspots ?? [])
const matches = computed<BusinessTrendMatch[]>(() => result.value?.external_matches ?? [])
const directions = computed(() => result.value?.business_directions ?? [])
const risks = computed(() => result.value?.risks ?? [])
const sourceStatuses = computed(() => result.value?.source_statuses ?? [])
const evidenceSamples = computed<BusinessTrendEvidenceSample[]>(() => result.value?.evidence_samples ?? [])
const confidenceLevel = computed(() => (result.value?.confidence === 'high' || result.value?.confidence === 'low' ? result.value.confidence : 'medium'))
const confidenceLabel = computed(() => {
  if (!result.value) return '待生成'
  if (confidenceLevel.value === 'high') return '依据充分'
  if (confidenceLevel.value === 'low') return '继续观察'
  return '可参考'
})
const deepBusy = computed(() => deepJob.value?.status === 'queued' || deepJob.value?.status === 'running')
const deepStatusClass = computed(() => {
  if (deepJob.value?.status === 'succeeded') return 'done'
  if (deepJob.value?.status === 'failed' || deepError.value) return 'failed'
  return 'running'
})
const deepStatusTitle = computed(() => {
  if (deepJob.value?.status === 'succeeded') return '深度分析已完成'
  if (deepJob.value?.status === 'failed' || deepError.value) return '深度分析暂时不可用'
  if (deepJob.value?.status === 'queued') return '深度分析已开始'
  return '正在深度分析'
})
const deepStatusMessage = computed(() => {
  if (deepError.value) return deepError.value
  if (deepJob.value?.analysis && deepJob.value.status === 'succeeded') return '已更新为深度业务判断。'
  return deepJob.value?.message || '正在结合近期任务与可用热点做判断。'
})

async function generate() {
  currentController?.abort()
  stopDeepPolling()
  const controller = new AbortController()
  currentController = controller
  loading.value = true
  error.value = ''
  deepError.value = ''
  deepJob.value = null
  try {
    const res = await reportsApi.businessTrendPilotAnalysis(
      currentTrendParams(),
      controller.signal,
    )
    const parsed = parseBusinessTrendResponse(res.data as { data?: BusinessTrendPilotResponse } | BusinessTrendPilotResponse)
    if (!parsed) throw new Error('业务热点分析暂未返回内容')
    result.value = parsed
    activeTab.value = 'hotspots'
  } catch (e) {
    if (controller.signal.aborted) return
    error.value = e instanceof Error ? e.message : '业务热点分析生成失败，请稍后重试'
  } finally {
    if (currentController === controller) {
      loading.value = false
      currentController = null
    }
  }
}

async function startDeepAnalysis() {
  if (loading.value || deepBusy.value || !result.value) return
  stopDeepPolling()
  const controller = new AbortController()
  deepController = controller
  deepStarting.value = true
  deepError.value = ''
  try {
    const res = await reportsApi.startBusinessTrendDeepAnalysis(currentTrendParams(), controller.signal)
    const job = parseBusinessTrendJobResponse(res.data as { data?: BusinessTrendDeepAnalysisJob } | BusinessTrendDeepAnalysisJob)
    if (!job?.job_id) throw new Error('深度分析暂未开始，请稍后重试')
    deepJob.value = job
    if (job.status === 'succeeded') {
      applyDeepAnalysis(job)
      return
    }
    if (job.status === 'failed') {
      markDeepFailed(job.error_message || job.message || '深度分析暂时不可用，基础分析仍可使用')
      return
    }
    scheduleDeepPoll(job.job_id)
  } catch (e) {
    if (controller.signal.aborted) return
    markDeepFailed(e instanceof Error ? e.message : '深度分析暂时不可用，基础分析仍可使用')
  } finally {
    if (deepController === controller) {
      deepController = null
    }
    deepStarting.value = false
  }
}

function scheduleDeepPoll(jobId: string, delay = 1500) {
  if (deepPollTimer) window.clearTimeout(deepPollTimer)
  deepPollTimer = window.setTimeout(() => {
    void pollDeepJob(jobId)
  }, delay)
}

async function pollDeepJob(jobId: string) {
  const controller = new AbortController()
  deepController = controller
  try {
    const res = await reportsApi.getBusinessTrendDeepAnalysisJob(jobId, controller.signal)
    const job = parseBusinessTrendJobResponse(res.data as { data?: BusinessTrendDeepAnalysisJob } | BusinessTrendDeepAnalysisJob)
    if (!job) throw new Error('深度分析状态暂时不可读')
    deepJob.value = job
    if (job.status === 'succeeded') {
      applyDeepAnalysis(job)
      return
    }
    if (job.status === 'failed') {
      markDeepFailed(job.error_message || job.message || '深度分析暂时不可用，基础分析仍可使用')
      return
    }
    scheduleDeepPoll(job.job_id)
  } catch (e) {
    if (controller.signal.aborted) return
    markDeepFailed(e instanceof Error ? e.message : '深度分析状态暂时不可读，基础分析仍可使用')
  } finally {
    if (deepController === controller) {
      deepController = null
    }
  }
}

function applyDeepAnalysis(job: BusinessTrendDeepAnalysisJob) {
  if (job.analysis) {
    result.value = job.analysis
    activeTab.value = 'directions'
  }
}

function markDeepFailed(message: string) {
  deepError.value = message
  if (deepJob.value) {
    deepJob.value = {
      ...deepJob.value,
      status: 'failed',
      message,
      error_message: message,
      updated_at: new Date().toISOString(),
    }
  }
}

function stopDeepPolling() {
  if (deepPollTimer) {
    window.clearTimeout(deepPollTimer)
    deepPollTimer = null
  }
  deepController?.abort()
  deepController = null
}

function currentTrendParams(): BusinessTrendPilotParams {
  return {
    from: dateOnly(rangeStart.value),
    to: dateOnly(rangeEnd.value),
    mode: mode.value,
  }
}

function parseBusinessTrendResponse(payload: { data?: BusinessTrendPilotResponse } | BusinessTrendPilotResponse | undefined): BusinessTrendPilotResponse | null {
  if (!payload) return null
  if ('headline' in payload || 'overview' in payload) return payload as BusinessTrendPilotResponse
  return payload.data ?? null
}

function parseBusinessTrendJobResponse(payload: { data?: BusinessTrendDeepAnalysisJob } | BusinessTrendDeepAnalysisJob | undefined): BusinessTrendDeepAnalysisJob | null {
  if (!payload) return null
  if ('job_id' in payload || 'status' in payload) return payload as BusinessTrendDeepAnalysisJob
  return payload.data ?? null
}

function dateOnly(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function formatTime(value: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString('zh-CN', { hour12: false })
}

function sourceStatusClass(status?: string): string {
  if (status === 'used') return 'used'
  if (status === 'failed') return 'failed'
  return 'skipped'
}

function sourceStatusText(source: { status?: string; message?: string; items?: number }): string {
  if (source.status === 'used') return source.items ? `${source.items} 条` : '已纳入'
  return source.message || '已跳过'
}

function priorityText(priority?: string): string {
  if (priority === 'high') return '优先'
  if (priority === 'low') return '观察'
  return '常规'
}

function riskLevelText(level?: string): string {
  if (level === 'high') return '高'
  if (level === 'low') return '低'
  return '中'
}

onBeforeUnmount(() => {
  currentController?.abort()
  stopDeepPolling()
})

watch(
  () => props.modelValue,
  (open) => {
    if (!open) {
      currentController?.abort()
      stopDeepPolling()
    }
  },
)
</script>

<style scoped>
.trend-pilot-modal {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  color: #0f172a;
  letter-spacing: 0;
}

.trend-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid #dbe5ef;
  border-radius: 0.5rem;
  background: #f8fafc;
  padding: 0.85rem;
}

.trend-hero span,
.control-label {
  display: block;
  font-size: 0.72rem;
  font-weight: 750;
  color: #64748b;
}

.trend-hero h3 {
  margin: 0.15rem 0 0;
  font-size: 1.05rem;
  font-weight: 800;
  line-height: 1.35;
  color: #0f172a;
}

.trend-hero p {
  margin: 0.35rem 0 0;
  max-width: 54rem;
  font-size: 0.82rem;
  line-height: 1.6;
  color: #475569;
}

.trend-confidence {
  flex: 0 0 auto;
  border-radius: 999px;
  border: 1px solid #cbd5e1;
  background: #fff;
  padding: 0.25rem 0.55rem;
  font-size: 0.72rem;
  font-weight: 750;
  color: #475569;
  white-space: nowrap;
}

.trend-confidence--high {
  border-color: #86efac;
  background: #f0fdf4;
  color: #15803d;
}

.trend-confidence--low {
  border-color: #fed7aa;
  background: #fff7ed;
  color: #c2410c;
}

.trend-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}

.control-group {
  display: flex;
  min-width: 13rem;
  flex-direction: column;
  gap: 0.35rem;
}

.segmented {
  display: inline-flex;
  width: fit-content;
  overflow: hidden;
  border: 1px solid #d8e2ec;
  border-radius: 0.5rem;
  background: #fff;
}

.segmented--wide {
  width: 100%;
}

.segmented button {
  min-height: 2rem;
  border: 0;
  border-right: 1px solid #e2e8f0;
  background: transparent;
  padding: 0 0.7rem;
  font-size: 0.76rem;
  font-weight: 700;
  color: #475569;
  white-space: nowrap;
}

.segmented button:last-child {
  border-right: 0;
}

.segmented button.active {
  background: #0f172a;
  color: #fff;
}

.segmented button:disabled {
  cursor: not-allowed;
  color: #94a3b8;
}

.button-icon {
  margin-right: 0.3rem;
  width: 0.9rem;
  height: 0.9rem;
}

.source-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.source-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 1.8rem;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  background: #fff;
  padding: 0.18rem 0.55rem;
}

.source-pill strong {
  font-size: 0.72rem;
  color: #1e293b;
}

.source-pill small {
  font-size: 0.7rem;
  color: #64748b;
}

.source-pill--used {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.source-pill--failed {
  border-color: #fecaca;
  background: #fef2f2;
}

.source-pill--skipped {
  background: #f8fafc;
}

.deep-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid #bfdbfe;
  border-radius: 0.5rem;
  background: #eff6ff;
  padding: 0.65rem 0.75rem;
}

.deep-status strong {
  display: block;
  font-size: 0.78rem;
  font-weight: 800;
  color: #1e3a8a;
}

.deep-status p {
  margin: 0.15rem 0 0;
  font-size: 0.75rem;
  line-height: 1.5;
  color: #475569;
}

.deep-status--done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.deep-status--done strong {
  color: #15803d;
}

.deep-status--failed {
  border-color: #fecaca;
  background: #fef2f2;
}

.deep-status--failed strong {
  color: #b91c1c;
}

.trend-loading,
.trend-error,
.trend-idle {
  display: flex;
  min-height: 13rem;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border: 1px dashed #cbd5e1;
  border-radius: 0.5rem;
  background: #f8fafc;
  text-align: left;
}

.trend-loading-dot {
  width: 0.8rem;
  height: 0.8rem;
  border-radius: 999px;
  background: #2563eb;
  animation: trend-pulse 1s ease-in-out infinite;
}

.trend-loading strong,
.trend-idle strong {
  display: block;
  font-size: 0.9rem;
  color: #0f172a;
}

.trend-loading p,
.trend-idle p,
.trend-error p {
  margin: 0.2rem 0 0;
  color: #64748b;
}

.trend-error {
  flex-direction: column;
  color: #b91c1c;
  text-align: center;
}

.trend-idle {
  flex-direction: column;
  text-align: center;
}

.idle-icon {
  width: 1.4rem;
  height: 1.4rem;
  color: #2563eb;
}

.trend-content {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.trend-tabs {
  display: flex;
  gap: 0.4rem;
  overflow-x: auto;
}

.trend-tabs button {
  min-height: 2rem;
  border: 1px solid #d8e2ec;
  border-radius: 0.45rem;
  background: #fff;
  padding: 0 0.7rem;
  font-size: 0.76rem;
  font-weight: 750;
  color: #475569;
  white-space: nowrap;
}

.trend-tabs button.active {
  border-color: #0f172a;
  background: #0f172a;
  color: #fff;
}

.trend-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.65rem;
}

.trend-panel {
  min-height: 8.5rem;
  border: 1px solid #dbe5ef;
  border-radius: 0.5rem;
  background: #fff;
  padding: 0.75rem;
}

.trend-panel--direction {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.trend-panel--risk {
  border-color: #fed7aa;
  background: #fff7ed;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.6rem;
}

.panel-head h4 {
  margin: 0;
  font-size: 0.86rem;
  font-weight: 800;
  line-height: 1.35;
  color: #0f172a;
}

.panel-head span {
  flex: 0 0 auto;
  border-radius: 999px;
  background: #f1f5f9;
  padding: 0.12rem 0.45rem;
  font-size: 0.7rem;
  font-weight: 750;
  color: #475569;
}

.trend-panel p {
  margin: 0.45rem 0 0;
  font-size: 0.78rem;
  line-height: 1.55;
  color: #475569;
}

.trend-panel strong {
  display: block;
  margin-top: 0.5rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: #1d4ed8;
}

.keyword-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-top: 0.55rem;
}

.keyword-row small {
  border-radius: 999px;
  background: #f1f5f9;
  padding: 0.12rem 0.45rem;
  font-size: 0.68rem;
  color: #475569;
}

.compact-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin: 0.55rem 0 0;
  padding: 0;
  list-style: none;
}

.compact-list li {
  font-size: 0.73rem;
  line-height: 1.45;
  color: #64748b;
}

.evidence-list {
  display: grid;
  gap: 0.5rem;
}

.evidence-list article {
  border: 1px solid #dbe5ef;
  border-radius: 0.5rem;
  background: #fff;
  padding: 0.65rem 0.75rem;
}

.evidence-list div {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.45rem;
}

.evidence-list strong {
  font-size: 0.8rem;
  color: #0f172a;
}

.evidence-list span,
.evidence-list p {
  margin: 0;
  font-size: 0.73rem;
  line-height: 1.5;
  color: #64748b;
}

.trend-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-top: 1px solid #e5e7eb;
  padding: 0.75rem 1rem;
}

.trend-footer span {
  font-size: 0.72rem;
  color: #64748b;
}

.trend-footer div {
  display: flex;
  gap: 0.5rem;
}

@keyframes trend-pulse {
  0%,
  100% {
    opacity: 0.35;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1);
  }
}

@media (max-width: 720px) {
  .trend-hero,
  .trend-controls {
    align-items: stretch;
    flex-direction: column;
  }

  .control-group,
  .segmented {
    width: 100%;
  }

  .segmented button {
    flex: 1 1 0;
    padding: 0 0.45rem;
  }

  .trend-footer div {
    width: 100%;
  }

  .trend-footer div :deep(button) {
    flex: 1 1 0;
  }
}
</style>
