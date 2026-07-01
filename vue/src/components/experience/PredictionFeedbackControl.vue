<template>
  <div v-if="visible" class="prediction-feedback" @click.stop>
    <span class="prediction-feedback__section-title">建议反馈</span>
    <div class="prediction-feedback__segment" role="group" aria-label="建议反馈">
      <button
        v-for="option in feedbackOptions"
        :key="option.value"
        type="button"
        class="prediction-feedback__button"
        :class="{ 'prediction-feedback__button--active': selectedValue === option.value }"
        :aria-pressed="selectedValue === option.value"
        :disabled="saving"
        @click="submitFeedback(option.value)"
      >
        {{ option.label }}
      </button>
    </div>

    <div v-if="needsReason" class="prediction-feedback__reasons" aria-label="反馈原因">
      <span class="prediction-feedback__reason-title">反馈原因</span>
      <button
        v-for="reason in reasonOptions"
        :key="reason.code"
        type="button"
        class="prediction-feedback__chip"
        :class="{ 'prediction-feedback__chip--active': selectedReason === reason.code }"
        :aria-pressed="selectedReason === reason.code"
        :disabled="saving"
        @click="submitFeedback(selectedValue, reason.code)"
      >
        {{ reason.label }}
      </button>
    </div>

    <div v-if="microQuestionVisible" class="prediction-feedback__micro">
      <button
        type="button"
        class="prediction-feedback__micro-toggle"
        :aria-expanded="microQuestionOpen"
        :aria-controls="microQuestionReasonsId"
        :disabled="microQuestionLoading || microQuestionSaving || microQuestionSubmitted"
        @click="toggleMicroQuestion"
      >
        {{ microQuestionSubmitted ? '已记录选择，正式反馈未改变' : microQuestionOpen ? '收起补充原因' : '可选：补充没有采用的原因' }}
      </button>
    </div>

    <div
      v-if="microQuestionOpen"
      :id="microQuestionReasonsId"
      class="prediction-feedback__reasons"
      aria-label="补充原因"
    >
      <span class="prediction-feedback__reason-title">补充原因（不改变上方反馈）</span>
      <button
        v-for="reason in microQuestionReasonOptions"
        :key="reason.code"
        type="button"
        class="prediction-feedback__chip"
        :class="{ 'prediction-feedback__chip--active': selectedMicroReason === reason.code }"
        :aria-pressed="selectedMicroReason === reason.code"
        :disabled="microQuestionSaving"
        @click="submitMicroQuestion(reason.code)"
      >
        {{ reason.label }}
      </button>
    </div>

    <p v-if="errorText" class="prediction-feedback__error" role="status">{{ errorText }}</p>
    <p v-if="microQuestionError" class="prediction-feedback__error" role="status">{{ microQuestionError }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import {
  experienceApi,
  type AISuggestionFeedbackValue,
  type ExperienceClientConfig,
  type ExperienceMicroQuestionEligibility,
  type ExperienceReasonTag,
} from '@/services/api/experienceApi'
import {
  configureExperienceBehavior,
  recordExperienceBehavior,
} from '@/services/experienceBehavior'
import type { PredictionSuggestion } from '@/services/api/predictionsApi'

const feedbackOptions: Array<{ value: AISuggestionFeedbackValue; label: string }> = [
  { value: 'accepted', label: '有用' },
  { value: 'partially_accepted', label: '部分有用' },
  { value: 'rejected', label: '无用' },
]

type ReasonOption = { code: string; label: string }

const fallbackReasonOptions: ReasonOption[] = [
  { code: 'spec_mismatch', label: '规格不符' },
  { code: 'asset_mismatch', label: '资产不符' },
  { code: 'stage_not_applicable', label: '阶段不适用' },
  { code: 'missing_context', label: '缺上下文' },
  { code: 'outdated', label: '已过时' },
  { code: 'customer_special_case', label: '客户特例' },
  { code: 'already_handled', label: '已处理' },
  { code: 'not_relevant', label: '不相关' },
]

const fallbackMicroQuestionReasonOptions: ReasonOption[] = [
  { code: 'temporarily_not_needed', label: '暂时不需要' },
  { code: 'will_handle_later', label: '稍后处理' },
  { code: 'already_handled', label: '已处理' },
  { code: 'not_relevant', label: '不相关' },
  { code: 'missing_context', label: '缺上下文' },
  { code: 'stage_not_applicable', label: '阶段不适用' },
  { code: 'customer_special_case', label: '客户特例' },
  { code: 'suggestion_outdated', label: '建议已过时' },
]

const props = defineProps<{
  suggestion: PredictionSuggestion
  surface: string
  route?: string
  enabled?: boolean
}>()

const localEnabled = ref(false)
const behaviorEnabled = ref(false)
const microQuestionEnabled = ref(false)
const reasonOptions = ref<ReasonOption[]>(fallbackReasonOptions)
const microQuestionReasonOptions = ref<ReasonOption[]>(fallbackMicroQuestionReasonOptions)
const selectedValue = ref<AISuggestionFeedbackValue | ''>('')
const selectedReason = ref('')
const selectedMicroReason = ref('')
const saving = ref(false)
const microQuestionLoading = ref(false)
const microQuestionSaving = ref(false)
const microQuestionOpen = ref(false)
const microQuestionSubmitted = ref(false)
const errorText = ref('')
const microQuestionError = ref('')
const impressionRecorded = ref(false)
const microQuestionEligibility = ref<ExperienceMicroQuestionEligibility | null>(null)
const microQuestionReasonsId = `prediction-feedback-micro-reasons-${useId()}`

let clientConfigPromise: Promise<ExperienceClientConfig | null> | null = null
let reasonTagsPromise: Promise<ExperienceReasonTag[] | null> | null = null

const suggestionEventId = computed(() => props.suggestion.suggestion_event_id || '')
const effectiveRoute = computed(() => props.route || (typeof window !== 'undefined' ? window.location.pathname : ''))
const visible = computed(() => Boolean(suggestionEventId.value) && (props.enabled ?? localEnabled.value))
const needsReason = computed(() => selectedValue.value === 'partially_accepted' || selectedValue.value === 'rejected')
const hasNonPositiveFeedback = computed(() => selectedValue.value === 'partially_accepted' || selectedValue.value === 'rejected')
const suggestionAttributionEligible = computed(() => props.suggestion.attribution_eligible !== false)
const microQuestionVisible = computed(
  () =>
    visible.value &&
    microQuestionEnabled.value &&
    hasNonPositiveFeedback.value &&
    suggestionAttributionEligible.value &&
    Boolean(props.suggestion.target_type && props.suggestion.target_id),
)

onMounted(async () => {
  void loadReasonOptions()
  const config = await getExperienceClientConfig()
  behaviorEnabled.value = Boolean(config?.behavior_capture_enabled)
  configureExperienceBehavior(config)
  const surfaces = config?.enabled_surfaces ?? []
  const surfaceEnabled = surfaces.includes(props.surface)
  microQuestionEnabled.value = Boolean(config?.micro_question_enabled && surfaceEnabled)
  maybeRecordImpression()
  if (props.enabled !== undefined) return
  localEnabled.value = Boolean(config?.ai_feedback_enabled && surfaceEnabled)
})

watch(
  visible,
  (isVisible) => {
    if (isVisible) maybeRecordImpression()
  },
  { immediate: true },
)

watch(suggestionEventId, () => {
  selectedValue.value = ''
  selectedReason.value = ''
  selectedMicroReason.value = ''
  microQuestionOpen.value = false
  microQuestionSubmitted.value = false
  microQuestionEligibility.value = null
  microQuestionError.value = ''
  impressionRecorded.value = false
  maybeRecordImpression()
})

async function getExperienceClientConfig(): Promise<ExperienceClientConfig | null> {
  if (!clientConfigPromise) {
    clientConfigPromise = experienceApi
      .clientConfig()
      .then((res) => res.data?.data ?? null)
      .catch(() => null)
  }
  return clientConfigPromise
}

async function loadReasonOptions(): Promise<void> {
  if (!reasonTagsPromise) {
    reasonTagsPromise = experienceApi
      .reasonTags({ scene: 'ai_suggestion_feedback' })
      .then((res) => res.data?.data ?? null)
      .catch(() => null)
  }
  const tags = await reasonTagsPromise
  if (!tags?.length) return
  reasonOptions.value = tags.map((tag) => ({ code: tag.code, label: tag.name }))
}

async function submitFeedback(value: AISuggestionFeedbackValue | '', reasonCode = ''): Promise<void> {
  if (!value || !suggestionEventId.value || saving.value) return
  saving.value = true
  errorText.value = ''
  const previousValue = selectedValue.value
  const previousReason = selectedReason.value
  selectedValue.value = value
  selectedReason.value = reasonCode || ''

  const suggestion = props.suggestion
  try {
    await experienceApi.feedback(suggestionEventId.value, {
      suggestion_event_id: suggestionEventId.value,
      feedback_value: value,
      reason_code: reasonCode || undefined,
      outcome_source_type: props.surface,
      outcome_source_id: suggestion.target_id || suggestion.id,
      payload: {
        surface: props.surface,
        target_type: suggestion.target_type || '',
        target_id: suggestion.target_id || '',
        suggestion_id: suggestion.id,
        suggestion_type: String(suggestion.type || ''),
        source: suggestion.source || '',
        action_type: suggestion.action_type || '',
        action_label: suggestion.action_label || '',
        route: effectiveRoute.value,
      },
    })
  } catch {
    selectedValue.value = previousValue
    selectedReason.value = previousReason
    errorText.value = '反馈未保存'
  } finally {
    saving.value = false
  }
}

async function toggleMicroQuestion(): Promise<void> {
  if (microQuestionSubmitted.value || microQuestionLoading.value || microQuestionSaving.value) return
  microQuestionError.value = ''
  if (microQuestionOpen.value) {
    if (behaviorEnabled.value) recordBehavior('dismiss')
    void submitMicroQuestionDismissed()
    microQuestionOpen.value = false
    return
  }
  if (!microQuestionEligibility.value) {
    await loadMicroQuestionEligibility()
  }
  if (microQuestionEligibility.value && !microQuestionEligibility.value.eligible) {
    microQuestionError.value = microQuestionEligibilityMessage(microQuestionEligibility.value.reason)
    return
  }
  if (microQuestionEligibility.value?.eligible) {
    microQuestionOpen.value = true
    if (behaviorEnabled.value) recordBehavior('expand')
  }
}

async function loadMicroQuestionEligibility(): Promise<void> {
  if (!suggestionEventId.value || microQuestionLoading.value) return
  microQuestionLoading.value = true
  microQuestionError.value = ''
  try {
    const suggestion = props.suggestion
    const res = await experienceApi.microQuestionEligibility({
      suggestion_event_id: suggestionEventId.value,
      suggestion_stable_key: suggestion.suggestion_stable_key || undefined,
      surface: props.surface,
      target_type: suggestion.target_type || undefined,
      target_id: suggestion.target_id || undefined,
    })
    const eligibility = res.data?.data ?? null
    microQuestionEligibility.value = eligibility
    const tags = eligibility?.reason_tags ?? []
    if (tags.length) {
      microQuestionReasonOptions.value = tags.map((tag) => ({ code: tag.code, label: tag.name }))
    }
    if (!eligibility?.eligible) {
      microQuestionError.value = microQuestionEligibilityMessage(eligibility?.reason)
    }
  } catch {
    microQuestionError.value = '暂不处理原因未加载'
  } finally {
    microQuestionLoading.value = false
  }
}

async function submitMicroQuestionDismissed(): Promise<void> {
  if (!suggestionEventId.value || microQuestionSaving.value || microQuestionSubmitted.value) return
  const suggestion = props.suggestion
  const eligibility = microQuestionEligibility.value
  microQuestionSaving.value = true
  microQuestionError.value = ''
  try {
    await experienceApi.microQuestionAnswer({
      answer_event_key: eligibility?.answer_event_key,
      suggestion_event_id: suggestionEventId.value,
      suggestion_stable_key: suggestion.suggestion_stable_key || undefined,
      surface: props.surface,
      target_type: suggestion.target_type || '',
      target_id: suggestion.target_id || '',
      answer_value: 'dismissed',
      payload: {
        route: effectiveRoute.value,
        suggestion_id: suggestion.id,
        suggestion_type: String(suggestion.type || ''),
        source: suggestion.source || '',
        action_type: suggestion.action_type || '',
        action_label: suggestion.action_label || '',
      },
    })
    microQuestionSubmitted.value = true
  } catch {
    microQuestionError.value = ''
  } finally {
    microQuestionSaving.value = false
  }
}

async function submitMicroQuestion(reasonCode: string): Promise<void> {
  if (!reasonCode || !suggestionEventId.value || microQuestionSaving.value) return
  const suggestion = props.suggestion
  const eligibility = microQuestionEligibility.value
  microQuestionSaving.value = true
  microQuestionError.value = ''
  const previousReason = selectedMicroReason.value
  selectedMicroReason.value = reasonCode
  try {
    await experienceApi.microQuestionAnswer({
      answer_event_key: eligibility?.answer_event_key,
      suggestion_event_id: suggestionEventId.value,
      suggestion_stable_key: suggestion.suggestion_stable_key || undefined,
      surface: props.surface,
      target_type: suggestion.target_type || '',
      target_id: suggestion.target_id || '',
      answer_value: 'answered',
      reason_code: reasonCode,
      payload: {
        route: effectiveRoute.value,
        suggestion_id: suggestion.id,
        suggestion_type: String(suggestion.type || ''),
        source: suggestion.source || '',
        action_type: suggestion.action_type || '',
        action_label: suggestion.action_label || '',
      },
    })
    microQuestionSubmitted.value = true
    microQuestionOpen.value = false
  } catch {
    selectedMicroReason.value = previousReason
    microQuestionError.value = '暂不处理原因未保存'
  } finally {
    microQuestionSaving.value = false
  }
}

function microQuestionEligibilityMessage(reason?: string): string {
  if (reason === 'already_answered') return '暂不处理原因已记录'
  if (reason === 'rate_limited') return '今日追问已达上限'
  if (reason === 'no_supported_attribution') return '已记录反馈，暂不需要补充原因'
  return '暂不可记录原因'
}

function recordBehavior(action: 'impression' | 'click' | 'expand' | 'dismiss'): void {
  const suggestion = props.suggestion
  recordExperienceBehavior({
    action,
    surface: props.surface,
    target_type: suggestion.target_type || '',
    target_id: suggestion.target_id || '',
    suggestion_event_id: suggestionEventId.value,
    suggestion_stable_key: suggestion.suggestion_stable_key || '',
    component: 'PredictionFeedbackControl',
    payload: {
      suggestion_id: suggestion.id,
      suggestion_type: String(suggestion.type || ''),
      action_type: suggestion.action_type || '',
    },
  })
}

function maybeRecordImpression(): void {
  if (!visible.value || !behaviorEnabled.value || impressionRecorded.value) return
  impressionRecorded.value = true
  recordBehavior('impression')
}
</script>

<style scoped>
.prediction-feedback {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 0.4rem;
}

.prediction-feedback__segment,
.prediction-feedback__reasons,
.prediction-feedback__micro {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.prediction-feedback__section-title {
  color: rgb(var(--yb-text-muted));
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1.2;
}

.prediction-feedback__segment {
  padding-top: 0.15rem;
}

.prediction-feedback__button,
.prediction-feedback__chip,
.prediction-feedback__micro-toggle {
  min-height: 1.9rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-muted));
  font: inherit;
  font-size: 0.75rem;
  font-weight: 750;
  line-height: 1;
  cursor: pointer;
  transition: background-color 160ms ease, border-color 160ms ease, color 160ms ease, box-shadow 160ms ease;
}

.prediction-feedback__button {
  border-radius: 999px;
  padding: 0.22rem 0.55rem;
}

.prediction-feedback__chip {
  border-radius: 0.45rem;
  padding: 0.22rem 0.46rem;
}

.prediction-feedback__reason-title {
  align-self: center;
  color: rgb(var(--yb-text-muted));
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1.2;
}

.prediction-feedback__micro-toggle {
  border-radius: 999px;
  padding: 0.22rem 0.55rem;
  background: rgb(var(--yb-surface-muted));
}

@media (max-width: 640px) {
  .prediction-feedback__button,
  .prediction-feedback__chip,
  .prediction-feedback__micro-toggle {
    min-height: 2.5rem;
  }
}

.prediction-feedback__button:hover,
.prediction-feedback__chip:hover,
.prediction-feedback__micro-toggle:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
}

.prediction-feedback__button--active,
.prediction-feedback__chip--active {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  box-shadow: 0 0 0 2px rgb(var(--yb-brand) / 0.08);
}

.prediction-feedback__button:disabled,
.prediction-feedback__chip:disabled,
.prediction-feedback__micro-toggle:disabled {
  cursor: wait;
  opacity: 0.68;
}

.prediction-feedback__error {
  margin: 0;
  color: rgb(var(--yb-danger));
  font-size: 0.6875rem;
  line-height: 1.25;
}
</style>
