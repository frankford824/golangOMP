<template>
  <div v-if="visible" class="prediction-feedback" @click.stop>
    <div class="prediction-feedback__segment" role="group" aria-label="建议反馈">
      <button
        v-for="option in feedbackOptions"
        :key="option.value"
        type="button"
        class="prediction-feedback__button"
        :class="{ 'prediction-feedback__button--active': selectedValue === option.value }"
        :disabled="saving"
        @click="submitFeedback(option.value)"
      >
        {{ option.label }}
      </button>
    </div>

    <div v-if="needsReason" class="prediction-feedback__reasons" aria-label="反馈原因">
      <button
        v-for="reason in reasonOptions"
        :key="reason.code"
        type="button"
        class="prediction-feedback__chip"
        :class="{ 'prediction-feedback__chip--active': selectedReason === reason.code }"
        :disabled="saving"
        @click="submitFeedback(selectedValue, reason.code)"
      >
        {{ reason.label }}
      </button>
    </div>

    <p v-if="errorText" class="prediction-feedback__error" role="status">{{ errorText }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  experienceApi,
  type AISuggestionFeedbackValue,
  type ExperienceClientConfig,
  type ExperienceReasonTag,
} from '@/services/api/experienceApi'
import { recordExperienceBehavior } from '@/services/experienceBehavior'
import type { PredictionSuggestion } from '@/services/api/predictionsApi'

const feedbackOptions: Array<{ value: AISuggestionFeedbackValue; label: string }> = [
  { value: 'accepted', label: '有用' },
  { value: 'partially_accepted', label: '部分' },
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

const props = defineProps<{
  suggestion: PredictionSuggestion
  surface: string
  route?: string
  enabled?: boolean
}>()

const localEnabled = ref(false)
const behaviorEnabled = ref(false)
const reasonOptions = ref<ReasonOption[]>(fallbackReasonOptions)
const selectedValue = ref<AISuggestionFeedbackValue | ''>('')
const selectedReason = ref('')
const saving = ref(false)
const errorText = ref('')
const impressionRecorded = ref(false)

let clientConfigPromise: Promise<ExperienceClientConfig | null> | null = null
let reasonTagsPromise: Promise<ExperienceReasonTag[] | null> | null = null

const suggestionEventId = computed(() => props.suggestion.suggestion_event_id || '')
const effectiveRoute = computed(() => props.route || (typeof window !== 'undefined' ? window.location.pathname : ''))
const visible = computed(() => Boolean(suggestionEventId.value) && (props.enabled ?? localEnabled.value))
const needsReason = computed(() => selectedValue.value === 'partially_accepted' || selectedValue.value === 'rejected')

onMounted(async () => {
  void loadReasonOptions()
  const config = await getExperienceClientConfig()
  behaviorEnabled.value = Boolean(config?.behavior_capture_enabled)
  maybeRecordImpression()
  if (props.enabled !== undefined) return
  const surfaces = config?.enabled_surfaces ?? []
  const surfaceEnabled = surfaces.length === 0 || surfaces.includes(props.surface)
  localEnabled.value = Boolean(config?.ai_feedback_enabled && surfaceEnabled)
})

watch(
  visible,
  (isVisible) => {
    if (isVisible) maybeRecordImpression()
  },
  { immediate: true },
)

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
  selectedValue.value = value
  if (reasonCode) selectedReason.value = reasonCode
  if (value === 'accepted') selectedReason.value = ''

  const suggestion = props.suggestion
  if (behaviorEnabled.value) recordBehavior('click')
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
    errorText.value = '反馈未保存'
  } finally {
    saving.value = false
  }
}

function recordBehavior(action: 'impression' | 'click'): void {
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
.prediction-feedback__reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.prediction-feedback__segment {
  padding-top: 0.15rem;
}

.prediction-feedback__button,
.prediction-feedback__chip {
  min-height: 1.65rem;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-muted));
  font: inherit;
  font-size: 0.6875rem;
  font-weight: 750;
  line-height: 1;
  cursor: pointer;
  transition: background-color 160ms ease, border-color 160ms ease, color 160ms ease, box-shadow 160ms ease;
}

.prediction-feedback__button {
  border-radius: 999px;
  padding: 0.15rem 0.5rem;
}

.prediction-feedback__chip {
  border-radius: 0.45rem;
  padding: 0.18rem 0.42rem;
}

.prediction-feedback__button:hover,
.prediction-feedback__chip:hover {
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
.prediction-feedback__chip:disabled {
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
