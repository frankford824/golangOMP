<template>
  <section
    class="detail-block h-full flex flex-col rounded-lg border border-gray-200 bg-white shadow-sm p-6"
    :class="{ 'basic-info--compact': compactFields }"
  >
    <div class="block-header">
      <div class="flex items-center gap-2">
        <span class="block-icon">I</span>
        <h3 class="block-title">基础信息</h3>
      </div>
    </div>
    <dl class="info-grid">
      <div class="info-row">
        <dt>任务号</dt>
        <dd class="font-mono text-sm">{{ task.taskNo }}</dd>
      </div>
      <div class="info-row">
        <dt>任务类型</dt>
        <dd><TaskTypeBadge :type="task.businessType ?? task.taskType" /></dd>
      </div>
      <div class="info-row">
        <dt>优先级</dt>
        <dd :class="priorityClass">{{ priorityLabel }}</dd>
      </div>
      <div v-if="showRequesterRow" class="info-row">
        <dt>发起人</dt>
        <dd>{{ requesterDisplay }}</dd>
      </div>
      <div class="info-row">
        <dt>创建人</dt>
        <dd>{{ creatorDisplay }}</dd>
      </div>
      <div class="info-row">
        <dt>设计师</dt>
        <dd>{{ designerDisplay }}</dd>
      </div>
      <div v-if="showCurrentHandlerRow" class="info-row">
        <dt>当前处理人</dt>
        <dd>{{ currentHandlerDisplay }}</dd>
      </div>
      <div class="ownership-block">
        <p class="ownership-block-title">组织归属</p>
        <dl class="ownership-inner">
          <template v-if="ownership.ownerDepartment || ownership.ownerOrgTeam">
            <div v-if="ownership.ownerDepartment" class="info-row">
              <dt>部门</dt>
              <dd>{{ ownership.ownerDepartment }}</dd>
            </div>
            <div v-if="ownership.ownerOrgTeam" class="info-row">
              <dt>团队</dt>
              <dd>{{ ownership.ownerOrgTeam }}</dd>
            </div>
          </template>
          <div v-if="ownership.legacyOwnerTeam" class="info-row legacy-row">
            <dt>任务兼容组</dt>
            <dd class="legacy-value">{{ ownership.legacyOwnerTeam }}</dd>
          </div>
          <div
            v-if="!ownership.ownerDepartment && !ownership.ownerOrgTeam && !ownership.legacyOwnerTeam"
            class="info-row"
          >
            <dt>归属</dt>
            <dd>{{ ownershipDisplay.primary }}</dd>
          </div>
        </dl>
        <p v-if="ownershipDisplay.usesFallback" class="ownership-note">当前归属信息来源于早期数据；主数据完善后将优先展示部门与团队。</p>
      </div>
      <div class="info-row">
        <dt>截止时间</dt>
        <dd :class="task.dueAt && isOverdue ? 'text-red-600 font-medium' : ''">
          {{ task.dueAt ? formatDate(task.dueAt) : '无截止时间' }}
          <span v-if="isOverdue" class="text-xs text-red-500 ml-1">已逾期</span>
        </dd>
      </div>
      <div v-if="task.isBatchTask !== undefined" class="info-row">
        <dt>批量任务</dt>
        <dd>{{ task.isBatchTask ? '是' : '否' }}</dd>
      </div>
      <div v-if="displayedBatchProductCount != null" class="info-row">
        <dt>商品数</dt>
        <dd>{{ displayedBatchProductCount }}</dd>
      </div>
      <div v-if="task.primarySkuCode" class="info-row">
        <dt>主展示 SKU</dt>
        <dd class="font-mono text-sm">{{ task.primarySkuCode }}</dd>
      </div>
      <div v-if="task.skuGenerationStatus" class="info-row">
        <dt>SKU 生成状态</dt>
        <dd>{{ skuGenerationStatusDisplay }}</dd>
      </div>
      <div v-if="task.productSelectionSourceMatchType" class="info-row">
        <dt>选品来源</dt>
        <dd>{{ productSelectionMatchDisplay }}</dd>
      </div>
      <div v-if="task.erpProductSkuId" class="info-row">
        <dt>ERP SKU ID</dt>
        <dd class="font-mono text-sm">{{ task.erpProductSkuId }}</dd>
      </div>
      <div v-if="task.note" class="info-row">
        <dt>备注</dt>
        <dd>{{ task.note }}</dd>
      </div>
    </dl>
  </section>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import type { ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'
import { isDoneStatus } from '@/domain/task-actions'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import TaskTypeBadge from '@/components/task/TaskTypeBadge.vue'
import { formatDateOnlyBeijing, isOverdueByBeijingDay as checkOverdue } from '@/utils/date'
import { getTaskOwnershipDisplay } from '@/domain/task-ownership'
import { parallelProductTabCount } from '@/domain/task-batch-assets'
import {
  productSelectionSourceMatchTypeLabelCn,
  skuGenerationStatusLabelCn,
} from '@/domain/mappers/read-model-labels-cn'
import {
  dashDisplay,
  isCurrentHandlerDistinctFromDesigner,
  isRequesterDistinctFromCreator,
  taskCreatorDisplayName,
  taskCurrentHandlerDisplayName,
  taskDesignerDisplayName,
} from '@/domain/task-actors'

withDefaults(
  defineProps<{
    /** 任务详情：收紧标签与内容间距，减少横向留白 */
    compactFields?: boolean
  }>(),
  { compactFields: false },
)

const injected = inject<ComputedRef<Task | null>>(TASK_DETAIL_KEY)
if (!injected) throw new Error('[TaskBasicInfoBlock] 必须在 TaskDetailView 内使用')

const task = computed(() => injected.value!)

const requesterDisplay = computed(() => dashDisplay(task.value.requesterName))

const creatorDisplay = computed(() => taskCreatorDisplayName(task.value))

const designerDisplay = computed(() => taskDesignerDisplayName(task.value))

const currentHandlerDisplay = computed(() => taskCurrentHandlerDisplayName(task.value))

const showRequesterRow = computed(() => isRequesterDistinctFromCreator(task.value))
const showCurrentHandlerRow = computed(() => isCurrentHandlerDistinctFromDesigner(task.value))

/** 与「商品与编码信息」Tab 数量一致；批量且存在 sku_items 时用并列行数，否则回退 batch_item_count */
const displayedBatchProductCount = computed(() => {
  const t = task.value
  if (t.isBatchTask === true && (t.skuItems?.length ?? 0) > 0) return parallelProductTabCount(t)
  if (t.batchItemCount != null) return t.batchItemCount
  return null
})

const ownershipDisplay = computed(() => getTaskOwnershipDisplay(task.value))

/** 详情区块用：canonical 有值时展示部门/团队行；legacy 单独一行 */
const ownership = computed(() => {
  const t = task.value
  const dept = t.ownerDepartment?.trim()
  const team = t.ownerOrgTeam?.trim()
  const legacy = t.groupName?.trim()
  return {
    ownerDepartment: dept || undefined,
    ownerOrgTeam: team || undefined,
    legacyOwnerTeam: legacy || undefined,
  }
})

// v1.21：low | normal | high | critical；展示兼容历史 medium/urgent
const priorityLabel = computed(() => {
  const m = {
    low: '低',
    medium: '中',
    normal: '普通',
    high: '高',
    urgent: '加急',
    critical: '加急',
  } as const
  return m[task.value.priority as keyof typeof m] ?? task.value.priority
})

const priorityClass = computed(() => {
  const priority = String(task.value.priority ?? '')
  if (priority === 'high' || priority === 'critical' || priority === 'urgent') return 'text-red-600 font-medium'
  if (priority === 'medium' || priority === 'normal') return 'text-amber-600'
  return 'text-slate-500'
})

const isOverdue = computed(() => checkOverdue(task.value.dueAt, isDoneStatus(task.value)))

const skuGenerationStatusDisplay = computed(() =>
  skuGenerationStatusLabelCn(task.value.skuGenerationStatus),
)

const productSelectionMatchDisplay = computed(() =>
  productSelectionSourceMatchTypeLabelCn(task.value.productSelectionSourceMatchType),
)

function formatDate(iso: string): string {
  return formatDateOnlyBeijing(iso)
}
</script>

<style scoped>
.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.25rem;
}
.block-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 0.375rem;
  background: rgb(248 250 252);
  color: rgb(148 163 184);
  font-size: 0.75rem;
  flex-shrink: 0;
}
.block-title { font-size: 0.875rem; font-weight: 600; color: rgb(30 41 59); margin: 0; }
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin: 0;
}
.info-row {
  display: grid;
  grid-template-columns: minmax(4.5rem, max-content) minmax(0, 1fr);
  column-gap: 0.5rem;
  align-items: baseline;
}
dt,
.field-label,
.row-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(75 85 99);
  margin: 0;
  text-align: left;
}
dd {
  font-size: 0.875rem;
  color: rgb(17 24 39);
  margin: 0;
  min-width: 0;
  text-align: right;
  word-break: break-word;
}
.ownership-block {
  padding: 0.5rem 0;
  border-top: 1px solid rgb(241 245 249);
  border-bottom: 1px solid rgb(241 245 249);
}
.ownership-block-title {
  margin: 0 0 0.375rem;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgb(71 85 105);
}
.ownership-inner { margin: 0; display: flex; flex-direction: column; gap: 0.5rem; }
.legacy-row dt { color: rgb(148 163 184); }
.legacy-value { color: rgb(100 116 139); font-size: 0.8125rem; }
.ownership-note {
  margin: 0.375rem 0 0;
  font-size: 0.6875rem;
  color: rgb(148 163 184);
  line-height: 1.4;
}
.basic-info--compact .info-row {
  column-gap: 0.4rem;
}
.basic-info--compact dt {
  max-width: 8.5rem;
}
.basic-info--compact dd {
  min-width: 0;
}
</style>
