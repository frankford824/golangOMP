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
        <BaseTextarea
          v-model="localForm.prefillSpecText"
          label="规格尺寸"
          :rows="2"
          placeholder="请输入规格尺寸"
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
  border: 1px solid #e6eaf0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
  min-height: 5.25rem;
}
.upload-card {
  background: #eef5ff;
}
.field-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #334155;
}
.required {
  color: #dc2626;
}
.form-hint {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.form-hint.danger {
  color: #dc2626;
}
.form-card :deep(.flex.flex-col.gap-1) {
  gap: 0.4rem;
}
.form-card :deep(input),
.form-card :deep(.relative > div) {
  height: 2.75rem;
  border-radius: 0.75rem;
  background: #f8fafc;
}
.form-card :deep(textarea) {
  border-radius: 0.75rem;
  background: #f8fafc;
  box-shadow: none;
  resize: vertical;
}

/* Phase 6: light embedded form skin (parent modal already light). Style-only. */
.form-card {
  border-color: #e5e7eb;
  background: #ffffff;
  color: #111827;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
}

.upload-card {
  background: #f9fafb;
  border-color: #e5e7eb;
}

.field-label {
  color: #374151;
}

.form-hint {
  color: #6b7280;
}

.form-hint.danger,
.required {
  color: #b91c1c;
}

.form-card :deep(input),
.form-card :deep(.relative > div),
.form-card :deep(textarea) {
  border-color: #d1d5db;
  background: #ffffff;
  color: #111827;
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: #9ca3af;
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}
@media (max-width: 760px) {
  .type-section {
    grid-template-columns: 1fr;
  }
}
</style>
