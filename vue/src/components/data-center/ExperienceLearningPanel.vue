<template>
  <section class="experience-panel">
    <div class="panel-header">
      <div>
        <h3 class="panel-title">经验观测</h3>
        <p class="panel-subtitle">采集、标签、AI 反馈与 worker 状态</p>
      </div>
      <BaseButton variant="secondary" size="sm" :loading="loading" @click="load">
        <RefreshCw class="button-icon" aria-hidden="true" />
        刷新
      </BaseButton>
    </div>

    <BaseErrorState v-if="error" :title="error" @retry="load" />

    <template v-else>
      <div v-if="loading" class="metric-grid">
        <BaseSkeleton v-for="i in 8" :key="i" width="100%" height="4.75rem" />
      </div>

      <BaseEmptyState
        v-else-if="!runtimeFlags.ui_enabled"
        title="经验观测未启用"
        description="当前环境已关闭经验观测页面。"
      />

      <template v-else>
        <div class="flag-row" aria-label="经验开关">
          <span :class="flagClass(runtimeFlags.capture_enabled)">采集 {{ flagLabel(runtimeFlags.capture_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.worker_enabled)">Worker {{ flagLabel(runtimeFlags.worker_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.ai_feedback_enabled)">AI 反馈 {{ flagLabel(runtimeFlags.ai_feedback_enabled) }}</span>
          <span :class="flagClass(runtimeFlags.ui_enabled)">页面 {{ flagLabel(runtimeFlags.ui_enabled) }}</span>
        </div>

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
              <h4>样本池</h4>
              <span>{{ sampleTotal }} 条</span>
            </div>
            <div v-if="samples.length" class="table-scroll">
              <table class="experience-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>来源</th>
                    <th>动作</th>
                    <th>结果</th>
                    <th>任务</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in samples" :key="item.id">
                    <td>{{ shortDateTime(item.event_time) }}</td>
                    <td>
                      <strong>{{ item.source_type }}</strong>
                      <small>{{ item.source_id || '-' }}</small>
                    </td>
                    <td>{{ item.action }}</td>
                    <td>{{ item.outcome || '-' }}</td>
                    <td>{{ item.task_id ?? '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <BaseEmptyState v-else title="暂无样本" description="当前没有可展示的经验事件。" />
          </section>

          <section class="panel-block">
            <div class="block-header">
              <h4>标签质量</h4>
              <span>{{ tags.length }} 个</span>
            </div>
            <div v-if="tags.length" class="tag-list">
              <article v-for="tag in tags" :key="`${tag.scene}-${tag.code}-${tag.version}`" class="tag-item">
                <div>
                  <strong>{{ tag.name }}</strong>
                  <small>{{ tag.scene }} · {{ tag.code }} · v{{ tag.version }}</small>
                </div>
                <span :class="flagClass(tag.enabled)">{{ tag.enabled ? '启用' : '停用' }}</span>
              </article>
            </div>
            <BaseEmptyState v-else title="暂无标签" description="当前没有可展示的经验标签。" />
          </section>
        </div>
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
  type ExperienceEvent,
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

const emptyFlags: ExperienceRuntimeFlags = {
  ui_enabled: false,
  capture_enabled: false,
  ai_feedback_enabled: false,
  worker_enabled: false,
}

const loading = ref(false)
const error = ref('')
const configFlags = ref<ExperienceRuntimeFlags>(emptyFlags)
const stats = ref<ExperienceStats | null>(null)
const samples = ref<ExperienceEvent[]>([])
const sampleTotal = ref(0)
const tags = ref<ExperienceReasonTag[]>([])

const runtimeFlags = computed(() => stats.value?.flags ?? configFlags.value)

const metrics = computed<MetricItem[]>(() => {
  const data = stats.value
  return [
    {
      key: 'total_events',
      label: '经验事件',
      value: integerLabel(data?.total_events),
      hint: '已入样本池',
    },
    {
      key: 'outbox_queued',
      label: 'Outbox 积压',
      value: integerLabel((data?.outbox_queued ?? 0) + (data?.outbox_processing ?? 0)),
      hint: `${integerLabel(data?.outbox_dead_letter)} dead-letter`,
    },
    {
      key: 'capture_rate',
      label: '24h 成功率',
      value: percentLabel(data?.capture_success_rate_24h),
      hint: `${integerLabel(data?.outbox_failed_24h)} 次失败`,
    },
    {
      key: 'tag_coverage',
      label: '标签覆盖',
      value: percentLabel(data?.tag_coverage_rate),
      hint: `${integerLabel(data?.tag_enabled)} / ${integerLabel(data?.tag_total)} 标签启用`,
    },
    {
      key: 'ai_feedback',
      label: 'AI 反馈率',
      value: percentLabel(data?.ai_feedback_rate),
      hint: `${integerLabel(data?.ai_feedback_events)} / ${integerLabel(data?.ai_suggestion_events)} 反馈`,
    },
    {
      key: 'profiles',
      label: '任务画像',
      value: integerLabel(data?.task_profiles),
      hint: data?.latest_profile_rebuilt_at ? shortDateTime(data.latest_profile_rebuilt_at) : '尚未生成',
    },
    {
      key: 'asset_quality',
      label: '资产质量',
      value: integerLabel(data?.asset_quality_labels),
      hint: '质量标签数',
    },
    {
      key: 'generated_at',
      label: '统计时间',
      value: data?.generated_at ? shortTime(data.generated_at) : '-',
      hint: data?.generated_at ? shortDate(data.generated_at) : '-',
    },
  ]
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const configRes = await experienceApi.config()
    configFlags.value = configRes.data?.data ?? emptyFlags
    stats.value = null
    samples.value = []
    sampleTotal.value = 0
    tags.value = []
    if (!configFlags.value.ui_enabled) return

    const [statsRes, samplesRes, tagsRes] = await Promise.all([
      experienceApi.stats(),
      experienceApi.samples({ page: 1, page_size: 20 }),
      experienceApi.reasonTags(),
    ])
    stats.value = statsRes.data?.data ?? null
    const parsedSamples = samplesRes.data as PaginatedEnvelope<ExperienceEvent>
    samples.value = parsedSamples.data ?? []
    sampleTotal.value = Number(parsedSamples.pagination?.total ?? samples.value.length)
    tags.value = tagsRes.data?.data ?? []
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载经验观测失败'
  } finally {
    loading.value = false
  }
}

function integerLabel(value: unknown): string {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n)) return '0'
  return Math.round(n).toLocaleString('zh-CN')
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

function shortDateTime(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return `${d.toLocaleDateString('zh-CN')} ${d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
}

function shortDate(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleDateString('zh-CN')
}

function shortTime(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
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
.panel-subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  letter-spacing: 0;
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
  border-color: rgb(22 163 74 / 0.35);
  background: rgb(240 253 244);
  color: rgb(21 128 61);
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
  gap: 0.55rem;
}
.metric-card,
.panel-block {
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 0.35rem 1.1rem rgb(var(--yb-shadow-blue) / 0.06);
}
.metric-card {
  display: flex;
  min-height: 4.75rem;
  flex-direction: column;
  justify-content: center;
  gap: 0.15rem;
  padding: 0.65rem 0.75rem;
}
.metric-card span,
.metric-card small,
.tag-item small {
  font-size: 0.72rem;
  color: rgb(var(--yb-text-blue-gray));
  letter-spacing: 0;
}
.metric-card strong {
  font-size: 1.25rem;
  line-height: 1.2;
  color: rgb(var(--yb-text-deep));
  letter-spacing: 0;
}
.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(18rem, 0.65fr);
  gap: 0.75rem;
}
.panel-block {
  min-width: 0;
  padding: 0.75rem;
}
.block-header {
  margin-bottom: 0.55rem;
}
.block-header h4 {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 750;
  letter-spacing: 0;
}
.block-header span {
  font-size: 0.72rem;
  color: rgb(var(--yb-text-blue-gray));
}
.table-scroll {
  overflow-x: auto;
}
.experience-table {
  width: 100%;
  min-width: 42rem;
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
.experience-table td strong,
.tag-item strong {
  display: block;
  color: rgb(var(--yb-text-deep));
  line-height: 1.3;
}
.experience-table td small {
  display: block;
  margin-top: 0.15rem;
  color: rgb(var(--yb-text-blue-gray));
}
.tag-list {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}
.tag-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgb(var(--yb-border-blue) / 0.72);
  border-radius: 0.45rem;
  padding: 0.55rem 0.6rem;
}
@media (max-width: 920px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 640px) {
  .panel-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .metric-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
