<template>
  <div class="type-section">
    <section class="form-row">
      <div class="form-card">
        <IIdSelector v-model="categoryModel" />
      </div>
      <div class="form-card">
        <BaseInput
          v-model="localForm.productName"
          label="产品名称"
          placeholder="新品产品名称"
          :maxlength="ERP_PRODUCT_NAME_MAX_LENGTH"
          :hint="erpProductNameHint(localForm.productName)"
          :error="erpProductNameError(localForm.productName)"
        />
        <p v-if="localForm.sku" class="form-hint">
          SKU：{{ localForm.sku }}（后端生成，预展示可用）
        </p>
        <p v-else class="form-hint danger">SKU 将由后端在创建任务时自动生成</p>
      </div>
    </section>

    <section class="form-card requirement-card">
      <BaseTextarea
        v-model="localForm.designRequirement"
        label="设计需求说明"
        :rows="4"
        placeholder="设计需求"
      />
    </section>

    <section class="form-row">
      <div class="form-card">
        <TaskSpecStructuredInput
          v-model="localForm.prefillSpecText"
          label="规格尺寸"
          placeholder="可按宽高或面积填写，用于系统自动计算成本"
          hint="例如宽高填写 100 × 200 厘米后，将提交为 100*200cm。"
        />
      </div>
      <div class="form-card upload-card">
        <label class="field-label">参考图（可选）</label>
        <ReferenceUploadPanel v-model="referenceRefsModel" compact />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskCreateFormModel } from '@/domain/types'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import TaskSpecStructuredInput from '@/components/task/TaskSpecStructuredInput.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import { ERP_PRODUCT_NAME_MAX_LENGTH, erpProductNameError, erpProductNameHint } from '@/domain/erp-product-name'

const props = defineProps<{
  form: TaskCreateFormModel
}>()

const emit = defineEmits<{
  'update:form': [TaskCreateFormModel]
}>()

const localForm = computed({
  get: () => props.form,
  set: (value: TaskCreateFormModel) => emit('update:form', value),
})
const referenceRefsModel = computed({
  get: () => localForm.value.referenceFileRefs,
  set: (value: (string | Record<string, unknown>)[]) => {
    localForm.value.referenceFileRefs = value
  },
})

const categoryModel = computed({
  get: () => localForm.value.category ?? '',
  set: (v: string) => {
    localForm.value.category = v === '' ? undefined : v
  },
})
</script>

<style scoped>
.type-section {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.form-row {
  display: contents;
}
.form-card {
  border: 1px solid rgb(var(--yb-border-context));
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: rgb(var(--yb-surface));
  min-height: 5.25rem;
}
.upload-card {
  background: rgb(var(--yb-surface-brand-row));
}
.field-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: rgb(var(--yb-text-slate));
}
.required {
  color: rgb(var(--yb-danger));
}
.form-hint {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-muted-strong));
}
.form-hint.danger {
  color: rgb(var(--yb-danger));
}
.form-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}
.form-card :deep(input),
.form-card :deep(.relative > div) {
  height: 2.75rem;
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
}
.form-card :deep(textarea) {
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  box-shadow: none;
  resize: vertical;
}

/* Phase 6: light embedded form skin (parent modal already light). Style-only. */
.form-card {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.upload-card {
  background: rgb(var(--yb-surface-soft));
  border-color: rgb(var(--yb-border));
}

.field-label {
  color: rgb(var(--yb-text-body));
}

.form-hint {
  color: rgb(var(--yb-text-muted));
}

.form-hint.danger,
.required {
  color: rgb(var(--yb-danger-text));
}

.form-card :deep(input),
.form-card :deep(.relative > div),
.form-card :deep(textarea) {
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: rgb(var(--yb-text-faint));
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}
@media (max-width: 760px) {
  .type-section {
    grid-template-columns: 1fr;
  }
}
</style>
