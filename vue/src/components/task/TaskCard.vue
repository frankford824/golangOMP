<template>
  <div
    class="tc"
    :class="{
      'tc--selected': selected,
      'tc--overdue': overdue,
      'tc--claimable': claimable,
      'tc--customization': customization,
    }"
  >
    <!-- 眉头:勾选 + 单号 + 类型标签 -->
    <div class="tc-eyebrow">
      <label class="tc-check" @click.stop>
        <input
          type="checkbox"
          class="tc-check-input"
          :aria-label="`选择任务 ${task.taskNo}`"
          :checked="selected"
          @change="emit('toggleSelect')"
        />
      </label>
      <span class="tc-no tc-copy-zone" data-card-copy-zone>{{ task.taskNo }}</span>
      <span class="tc-eyebrow-spacer"></span>
      <div class="tc-tags">
        <span class="tc-priority" :class="`tc-priority--${priorityMeta.tone}`">
          {{ priorityMeta.label }}
        </span>
        <span class="tc-tag" :class="categoryKind === 'custom' ? 'tc-tag--custom' : 'tc-tag--brand'">
          {{ categoryLabel }}
        </span>
        <WorkflowLaneTag v-if="showLaneTag" :lane="task.businessLane" />
        <TaskTypeBadge :type="task.businessType ?? task.taskType" />
      </div>
    </div>

    <!-- 悬停复制条 -->
    <div class="tc-copy-toolbar" data-card-no-nav="true" @click.stop>
      <button
        v-if="canCopyNo"
        type="button"
        :aria-label="`复制任务号 ${task.taskNo}`"
        title="复制任务号"
        @click="emit('copy', task.taskNo, '任务号')"
      >
        单号
      </button>
      <button
        v-if="canCopyTitle"
        type="button"
        :aria-label="`复制任务名称 ${title}`"
        title="复制任务名称"
        @click="emit('copy', title, '任务名称')"
      >
        名称
      </button>
      <button
        v-if="canCopySku"
        type="button"
        :aria-label="`复制 SKU ${sku}`"
        title="复制 SKU"
        @click="emit('copy', sku, 'SKU')"
      >
        SKU
      </button>
    </div>

    <!-- 标题 -->
    <div class="tc-title tc-copy-zone" :title="title" data-card-copy-zone>{{ title }}</div>

    <!-- 状态:圆点 + 文字 -->
    <div class="tc-status">
      <TaskMainStatusBadge
        v-if="task.mainStatus"
        :status="task.mainStatus"
        :label-override="statusLabelOverride"
      />
      <TaskStatusTag v-else :status="task.status" />
      <FilingStatusBadge
        v-if="task.filing_status || isRetouch"
        :status="task.filing_status"
        :task-type="task.businessType ?? task.taskType"
        :missing-fields-summary="task.missing_fields_summary_cn"
        :error-message="task.filing_error_message"
      />
      <span v-if="isBatch" class="tc-batch-count">共 {{ batchCount }} 项</span>
    </div>

    <!-- 批量预览:固定槽位(2 行),其余进弹窗 -->
    <div v-if="isBatch" class="tc-batch" data-card-no-nav="true" @click.stop>
      <div class="tc-batch-rows">
        <div
          v-for="item in batchPreview"
          :key="item.key"
          class="tc-batch-row"
          :title="item.summary"
        >
          <span class="tc-batch-index">{{ item.seq }}</span>
          <span class="tc-batch-text">{{ item.summary }}</span>
        </div>
      </div>
      <button
        v-if="hasMoreBatch"
        type="button"
        class="tc-batch-more"
        @click="emit('openBatch')"
      >
        查看全部 {{ batchCount }} 项 →
      </button>
    </div>

    <!-- 元信息:去盒,安静键值 -->
    <div class="tc-meta">
      <span class="tc-meta-key">归属</span>
      <span class="tc-meta-val" :title="ownership">{{ ownership }}</span>
      <span class="tc-meta-key">SKU</span>
      <span class="tc-meta-val tc-meta-sku tc-copy-zone" :title="sku" data-card-copy-zone>{{ sku }}</span>
      <template v-if="!isBatch">
        <span class="tc-meta-key">创建</span>
        <span class="tc-meta-val" :title="creator">{{ creator }}</span>
        <template v-if="showDesigner">
          <span class="tc-meta-key">设计</span>
          <span class="tc-meta-val" :title="designer">{{ designer }}</span>
        </template>
      </template>
    </div>

    <!-- 页脚:日期 + 接单 -->
    <div class="tc-footer">
      <span class="tc-time">更新 <b>{{ updatedText }}</b></span>
      <span class="tc-time" :class="{ 'tc-time--overdue': overdue }">
        截止 <b>{{ dueText }}</b><template v-if="overdue"> · 已逾期</template>
      </span>
      <span class="tc-footer-spacer"></span>
      <BaseButton
        v-if="claimable"
        size="sm"
        variant="secondary"
        :loading="claiming"
        :disabled="claimDisabled"
        @click.stop="emit('claim')"
      >
        {{ claimLabel }}
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '@/domain/types/task'
import TaskStatusTag from './TaskStatusTag.vue'
import TaskTypeBadge from './TaskTypeBadge.vue'
import WorkflowLaneTag from './WorkflowLaneTag.vue'
import TaskMainStatusBadge from '../business/TaskMainStatusBadge.vue'
import FilingStatusBadge from '../business/FilingStatusBadge.vue'
import BaseButton from '../base/BaseButton.vue'

export interface TaskCardBatchPreviewItem {
  key: string
  seq: number | string
  summary: string
}

const props = defineProps<{
  task: Task
  selected: boolean
  overdue: boolean
  claimable: boolean
  customization: boolean
  title: string
  sku: string
  ownership: string
  creator: string
  designer: string
  showDesigner: boolean
  categoryLabel: string
  categoryKind: 'normal' | 'custom'
  showLaneTag: boolean
  statusLabelOverride?: string
  isRetouch: boolean
  isBatch: boolean
  batchCount: number
  batchPreview: TaskCardBatchPreviewItem[]
  hasMoreBatch: boolean
  claiming: boolean
  claimDisabled: boolean
  claimLabel: string
  canCopyNo: boolean
  canCopyTitle: boolean
  canCopySku: boolean
  updatedText: string
  dueText: string
}>()

const priorityMeta = computed(() => ({
  low: { label: '低优先级', tone: 'low' },
  normal: { label: '普通', tone: 'normal' },
  high: { label: '高优先级', tone: 'high' },
  critical: { label: '加急', tone: 'critical' },
}[props.task.priority] ?? { label: '普通', tone: 'normal' }))

const emit = defineEmits<{
  toggleSelect: []
  copy: [value: string, label: string]
  claim: []
  openBatch: []
}>()
</script>

<style scoped>
.tc {
  --tc-rail: rgb(var(--yb-brand));
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 7px;
  height: 14.2rem;
  padding: 11px 13px 10px 14px;
  background: rgb(var(--yb-surface));
  border: 1px solid rgb(var(--yb-border));
  border-radius: 13px;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.04);
  cursor: pointer;
  overflow: hidden;
  user-select: text;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}
.tc::before {
  content: '';
  position: absolute;
  inset: 9px auto 9px 0;
  width: 3px;
  border-radius: 3px;
  background: var(--tc-rail);
  transition: inset 0.18s ease;
}
.tc:hover {
  transform: translateY(-2px);
  border-color: rgb(var(--yb-brand) / 0.45);
  box-shadow: 0 14px 30px -16px rgb(var(--yb-shadow-stone) / 0.22);
}
.tc:hover::before {
  inset: 0 auto 0 0;
}
.tc--customization {
  --tc-rail: rgb(var(--yb-purple-text));
}
.tc--overdue {
  --tc-rail: rgb(var(--yb-danger));
  border-color: rgb(var(--yb-warning-border));
}
.tc--selected {
  border-color: rgb(var(--yb-brand) / 0.55);
  box-shadow: 0 0 0 1px rgb(var(--yb-brand) / 0.35), 0 10px 24px -14px rgb(var(--yb-brand) / 0.4);
}
.tc--selected::before {
  width: 4px;
}

/* 眉头 */
.tc-eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 17px;
}
.tc-eyebrow-spacer {
  flex: 1;
}
.tc-check {
  display: inline-flex;
  flex: none;
  align-items: center;
  user-select: none;
}
.tc-check-input {
  width: 15px;
  height: 15px;
  border-radius: 5px;
  cursor: pointer;
  accent-color: rgb(var(--yb-brand));
}
.tc-no {
  font-family: var(--yb-font-data);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: rgb(var(--yb-text-faint));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tc-tags {
  display: flex;
  flex: none;
  align-items: center;
  gap: 6px;
  user-select: none;
}

/* 标签 */
.tc-tag {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 6px;
  padding: 2px 7px;
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text-muted));
  font-size: 10.5px;
  font-weight: 700;
  line-height: 1.5;
  white-space: nowrap;
}
.tc-tag--brand {
  background: rgb(var(--yb-brand) / 0.08);
  border-color: rgb(var(--yb-brand) / 0.18);
  color: rgb(var(--yb-brand-strong));
}
.tc-tag--custom {
  background: rgb(var(--yb-purple-text) / 0.1);
  border-color: rgb(var(--yb-purple-text) / 0.22);
  color: rgb(var(--yb-purple-text));
}
.tc-tags :deep(.task-type-badge),
.tc-tags :deep(.lane-tag) {
  font-size: 10.5px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 6px;
  line-height: 1.5;
}
.tc-priority {
  border: 1px solid transparent;
  border-radius: 6px;
  padding: 2px 7px;
  font-size: 10.5px;
  font-weight: 750;
  line-height: 1.5;
  white-space: nowrap;
}
.tc-priority--low {
  background: rgb(var(--yb-surface-soft));
  border-color: rgb(var(--yb-border));
  color: rgb(var(--yb-text-muted));
}
.tc-priority--normal {
  background: rgb(var(--yb-brand) / 0.07);
  border-color: rgb(var(--yb-brand) / 0.14);
  color: rgb(var(--yb-brand-strong));
}
.tc-priority--high {
  background: rgb(var(--yb-warning-soft));
  border-color: rgb(var(--yb-warning-border));
  color: rgb(var(--yb-warning-text));
}
.tc-priority--critical {
  background: rgb(var(--yb-danger-soft));
  border-color: rgb(var(--yb-danger) / 0.25);
  color: rgb(var(--yb-danger));
}

/* 标题 */
.tc-title {
  font-size: 13.5px;
  font-weight: 680;
  line-height: 1.4;
  color: rgb(var(--yb-text));
  min-height: calc(1.4em * 2);
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  overflow-wrap: anywhere;
  word-break: break-word;
}

/* 状态:圆点 + 文字(复用现有徽章组件,内部 pill 改写为扁平点) */
.tc-status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 14px;
}
.tc-status :deep(.inline-flex) {
  gap: 7px;
  padding: 0;
  border-radius: 0;
  border-color: transparent;
  background: transparent;
  font-size: 11.5px;
  font-weight: 650;
}
.tc-status :deep(.inline-flex)::before {
  content: '';
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex: none;
  background: currentColor;
}
.tc-batch-count {
  font-size: 11.5px;
  font-weight: 600;
  color: rgb(var(--yb-text-muted));
}

/* 批量预览 */
.tc-batch {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-left: 1px;
  padding-left: 10px;
  border-left: 2px solid rgb(var(--yb-border));
}
.tc-batch-rows {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 3.1em;
  overflow: hidden;
}
.tc-batch-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 11.5px;
  color: rgb(var(--yb-text-muted));
}
.tc-batch-index {
  min-width: 14px;
  font-family: var(--yb-font-data);
  font-size: 10px;
  font-weight: 700;
  color: rgb(var(--yb-text-faint));
}
.tc-batch-text {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.tc-batch-more {
  align-self: flex-start;
  padding: 2px 0 0;
  border: 0;
  background: none;
  color: rgb(var(--yb-brand));
  font-size: 11px;
  font-weight: 650;
  cursor: pointer;
}
.tc-batch-more:hover,
.tc-batch-more:focus-visible {
  color: rgb(var(--yb-brand-strong));
  text-decoration: underline;
  outline: none;
}

/* 元信息 */
.tc-meta {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr) max-content minmax(0, 1fr);
  gap: 4px 8px;
  align-items: baseline;
  font-size: 11px;
}
.tc-meta-key {
  color: rgb(var(--yb-text-faint));
  font-weight: 600;
  white-space: nowrap;
}
.tc-meta-val {
  min-width: 0;
  color: rgb(var(--yb-text-body));
  font-weight: 550;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tc-meta-sku {
  font-family: var(--yb-font-data);
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: rgb(var(--yb-indigo-text));
}

/* 页脚 */
.tc-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid rgb(var(--yb-border-subtle));
}
.tc-footer-spacer {
  flex: 1;
}
.tc-time {
  font-size: 11px;
  font-weight: 550;
  line-height: 1.35;
  color: rgb(var(--yb-text-faint));
  white-space: nowrap;
}
.tc-time b {
  font-weight: 600;
  color: rgb(var(--yb-text-muted));
}
.tc-time--overdue,
.tc-time--overdue b {
  color: rgb(var(--yb-danger));
  font-weight: 650;
}
.tc-footer :deep(button) {
  min-height: 1.7rem;
}

/* 复制条(悬停显现) */
.tc-copy-zone {
  position: relative;
  min-width: 0;
  cursor: text;
  user-select: text;
}
.tc-copy-toolbar {
  position: absolute;
  top: 2.1rem;
  right: 0.7rem;
  z-index: 8;
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  padding: 0.16rem;
  border: 1px solid rgb(var(--yb-overlay-slate) / 0.32);
  border-radius: 999px;
  background: rgb(var(--yb-surface) / 0.96);
  box-shadow: 0 10px 28px rgb(var(--yb-shadow) / 0.12);
  opacity: 0;
  pointer-events: none;
  transform: translateY(-0.25rem);
  transition: opacity 0.14s ease, transform 0.14s ease;
  user-select: none;
}
.tc:hover .tc-copy-toolbar,
.tc:focus-within .tc-copy-toolbar {
  opacity: 1;
  pointer-events: auto;
  transform: translateY(0);
}
.tc-copy-toolbar button {
  min-height: 1.45rem;
  padding: 0.14rem 0.46rem;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--yb-text-soft));
  font-size: 0.68rem;
  font-weight: 850;
  line-height: 1;
  cursor: pointer;
}
.tc-copy-toolbar button:hover,
.tc-copy-toolbar button:focus-visible {
  background: rgb(var(--yb-brand) / 0.1);
  color: rgb(var(--yb-brand-strong));
  outline: none;
}
</style>
