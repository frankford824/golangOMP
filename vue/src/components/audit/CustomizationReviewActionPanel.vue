/**
 * 审核工作台「定制 lane」操作面板。
 *
 * 与 `AuditActionPanel`（普通 A/B 审核）互斥：当选中任务处于
 * `PendingCustomizationReview` 或 `PendingEffectReview` 时，父组件渲染本组件。
 *
 * 职责：
 *   - 展示一个主按钮「提交定制审核」/「提交效果审核」（按 mode 切文案），
 *     点开共享 `CustomizationReviewForm` 弹窗；
 *   - 初审（initial）→ `POST /v1/tasks/:id/customization/review`
 *   - 二次效果审核（effect）→ 先查 `customization_jobs` 取 job_id，再
 *     `POST /v1/customization-jobs/:jobId/effect-review`；
 *   - 保留交班 / 转交 / 接手三个既有入口，由父组件处理（与普通面板一致）。
 *
 * 门禁原则：前端不推导 task_status，mode 完全由父组件按后端快照决定；
 * 若 effect-review 没有关联 job，展示明确错误而非静默失败或伪造成功。
 */
<template>
  <div class="customization-review-action-panel">
    <h4>{{ panelTitle }}</h4>
    <p v-if="contextHint" class="hint">{{ contextHint }}</p>
    <p class="rule-hint">{{ ruleHint }}</p>

    <p v-if="staleNotice" class="stale-notice">{{ staleNotice }}</p>
    <p v-if="actionError" class="error-inline">{{ actionError }}</p>
    <p v-if="disabledReason" class="disabled-reason">{{ disabledReason }}</p>

    <div class="action-buttons">
      <BaseButton
        variant="primary"
        size="sm"
        :disabled="actionsDisabled"
        :loading="submitting"
        @click="onOpenDialog"
      >
        {{ primaryButtonLabel }}
      </BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || handoverDisabled"
        @click="$emit('handover')"
      >
        发起交班
      </BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || transferDisabled"
        @click="$emit('transfer')"
      >
        转交
      </BaseButton>
      <BaseButton
        size="sm"
        variant="secondary"
        :disabled="actionsDisabled || takeoverDisabled"
        @click="$emit('takeover')"
      >
        接手交班任务
      </BaseButton>
    </div>

    <CustomizationReviewForm
      v-model="dialogOpen"
      :mode="mode"
      :default-reviewer-id="currentUserId"
      :loading="submitting"
      :error="actionError"
      @submit="onFormSubmit"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Task } from '@/domain/types/task'
import { usePermission } from '@/composables/usePermission'
import { useTasksStore } from '@/stores/tasks'
import {
  listCustomizationJobs,
  submitCustomizationEffectReview,
} from '@/services/api/customizationApi'
import type { CustomizationJobRaw } from '@/services/apiTypes'
import {
  denyCodeText,
  formatTaskActionDenyMessage,
  isCustomizationStageMismatch,
} from '@/domain/task-action-deny'
import { useSubmitGuard } from '@/composables/useSubmitGuard'
import BaseButton from '@/components/base/BaseButton.vue'
import CustomizationReviewForm, {
  type CustomizationReviewPayload,
  type CustomizationReviewMode,
} from '@/components/customization/CustomizationReviewForm.vue'

const props = withDefaults(
  defineProps<{
    task: Task
    mode: CustomizationReviewMode
    actionsDisabled?: boolean
    disabledReason?: string
    handoverDisabled?: boolean
    transferDisabled?: boolean
    takeoverDisabled?: boolean
    contextHint?: string
  }>(),
  {
    actionsDisabled: false,
    disabledReason: '',
    handoverDisabled: false,
    transferDisabled: false,
    takeoverDisabled: false,
    contextHint: '',
  },
)

const emit = defineEmits<{
  success: []
  handover: []
  transfer: []
  takeover: []
}>()

const { currentUser } = usePermission()
const tasksStore = useTasksStore()

const currentUserId = computed(() => currentUser.value?.id ?? null)
const dialogOpen = ref(false)
const { submitting, guard: submitGuard } = useSubmitGuard()
const actionError = ref('')
const staleNotice = ref('')

const panelTitle = computed(() =>
  props.mode === 'effect' ? '定制效果审核' : '定制审核',
)

const primaryButtonLabel = computed(() =>
  props.mode === 'effect' ? '提交效果审核' : '提交定制审核',
)

const ruleHint = computed(() =>
  props.mode === 'effect'
    ? '二次审核：提交后将绑定到对应定制任务，通过与否会自动推进任务状态。'
    : '定制初审：提交后将记录本次审核结论，选择“退回设计”可把稿件退回设计师返修。',
)

function onOpenDialog() {
  if (props.actionsDisabled) return
  actionError.value = ''
  staleNotice.value = ''
  dialogOpen.value = true
}

// 切到另一条任务时清空上一轮的提示，避免「刚提交过的 stale 提示」串到新任务上。
watch(
  () => props.task.id,
  () => {
    actionError.value = ''
    staleNotice.value = ''
  },
)

/**
 * 从 `listCustomizationJobs` 取与当前任务关联的 customization job id。
 * 与 `AuditOutsourceBlock.loadLatestCustomizationJob` 保持一致，避免两个入口
 * 对 job 归属的解释产生分歧。
 */
async function resolveCustomizationJobId(): Promise<number | string | null> {
  const body = await listCustomizationJobs({
    task_id: props.task.id,
    page: 1,
    page_size: 1,
  })
  const root = (body && typeof body === 'object' ? body : {}) as { data?: unknown }
  const list = Array.isArray(root.data) ? (root.data as CustomizationJobRaw[]) : []
  const first = list[0]
  if (!first) return null
  const raw = (first as { id?: unknown }).id
  if (raw == null) return null
  return typeof raw === 'number' || typeof raw === 'string' ? raw : null
}

/**
 * 提交入口：`useSubmitGuard` 在代码层拦截二次点击，与按钮 `:disabled+:loading`
 * 构成双保险。对 `customization_stage_mismatch`（后端已感知到该任务已被并发
 * 推进）走「静默成功 + 刷新列表」分支：关闭弹窗 + 通知父组件刷新 lane，
 * 再给用户一条 neutral-tone 提示，而不是原始 403 文案。
 */
async function onFormSubmit(payload: CustomizationReviewPayload) {
  actionError.value = ''
  staleNotice.value = ''
  await submitGuard(async () => {
    try {
      if (props.mode === 'initial') {
        await tasksStore.submitCustomizationReview(props.task.id, payload)
      } else {
        const jobId = await resolveCustomizationJobId()
        if (jobId == null) {
          throw new Error('当前任务未关联 customization job，请先刷新定制列表或联系后端核对。')
        }
        await submitCustomizationEffectReview(jobId, payload)
      }
      dialogOpen.value = false
      emit('success')
    } catch (e) {
      if (isCustomizationStageMismatch(e)) {
        dialogOpen.value = false
        staleNotice.value = denyCodeText('customization_stage_mismatch') ?? '该任务已被审核处理过，正在刷新列表…'
        emit('success')
        return
      }
      actionError.value = formatTaskActionDenyMessage(e, '定制审核提交失败')
    }
  })
}
</script>

<style scoped>
.customization-review-action-panel {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.customization-review-action-panel h4 {
  margin: 0 0 0.25rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(var(--yb-indigo-action));
}
.hint {
  margin: 0;
  padding: 0.35rem 0.5rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-brand-deep));
  background: rgb(var(--yb-brand-soft));
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 6px;
  line-height: 1.45;
}
.stale-notice {
  margin: 0;
  padding: 0.35rem 0.5rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-warning-dark));
  background: rgb(var(--yb-warning-soft));
  border: 1px solid rgb(var(--yb-warning-border-soft));
  border-radius: 6px;
  line-height: 1.45;
}
.rule-hint {
  margin: 0;
  font-size: 0.6875rem;
  color: rgb(var(--yb-text-muted-strong));
  line-height: 1.45;
}
.error-inline {
  margin: 0;
  padding: 0.35rem 0.5rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
  border-radius: 6px;
}
.disabled-reason {
  margin: 0;
  font-size: 0.6875rem;
  color: rgb(var(--yb-text-placeholder));
}
.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  margin-top: 0.25rem;
}
</style>
