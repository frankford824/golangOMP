<template>
  <div class="type-section">
    <section class="form-card product-card">
      <div>
        <label class="field-label">选择产品 <span class="required">*</span></label>
        <BaseButton
          variant="primary"
          class="product-picker-btn"
          @click="showProductPicker = true"
        >
          {{ localForm.productName || '从 ERP 选择产品' }}
        </BaseButton>
        <p v-if="localForm.sku" class="form-hint">SKU: {{ localForm.sku }}（不可手填）</p>
      </div>

      <div v-if="localForm.productId" class="selected-product-card">
        <div class="selected-thumb">
          <img
            v-if="localForm.productImageUrl"
            :src="localForm.productImageUrl"
            alt="产品图片"
            class="selected-thumb-img"
            @error="onImageError($event)"
          />
          <div v-else class="selected-thumb-placeholder">无图</div>
        </div>
        <div class="selected-info">
          <div class="selected-name" :title="localForm.productName || '未命名产品'">
            {{ localForm.productName || '未命名产品' }}
          </div>
          <div class="selected-meta">
            <span>SKU：{{ localForm.sku || '无 SKU' }}</span>
            <span>
              分类：{{ localForm.productCategoryName || localForm.productCategoryCode || '未分类' }}
            </span>
          </div>
        </div>
      </div>
    </section>

    <section class="form-card requirement-card">
      <BaseTextarea
        v-model="localForm.designRequirement"
        label="修改要求"
        :rows="4"
        placeholder="请填写本次修改的具体要求"
      />
    </section>

    <div class="form-card">
      <BaseInput
        v-model="localForm.prefillSpecText"
        label="产品尺寸（可选）"
        placeholder="默认使用 ERP 尺寸，可按需覆盖"
      />
    </div>
    <div class="form-card upload-card">
      <label class="field-label">参考图（可选）</label>
      <ReferenceUploadPanel v-model="referenceRefsModel" compact />
    </div>

    <ProductPickerDialog
      v-model="showProductPicker"
      @select="onProductSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TaskCreateFormModel } from '@/domain/types'
import type { Product } from '@/types'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'
import ProductPickerDialog from '@/components/products/ProductPickerDialog.vue'

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

const showProductPicker = ref(false)
const referenceRefsModel = computed({
  get: () => localForm.value.referenceFileRefs,
  set: (value: (string | Record<string, unknown>)[]) => {
    localForm.value.referenceFileRefs = value
  },
})

function onProductSelect(p: Product) {
  localForm.value.productId = p.id
  localForm.value.productName = p.name
  localForm.value.sku = p.sku
  localForm.value.productImageUrl = p.imageUrl
  localForm.value.productCategoryName = p.category
  localForm.value.productCategoryCode = p.categoryCode
  // ERP 返回 product_id 可能为 SKU 字符串（非数字），后端需 erp_product 快照解析
  localForm.value.erpProductSnapshot = {
    product_id: p.id,
    sku_code: p.sku ?? '',
    name: p.name ?? '',
    product_name: p.name ?? '',
    category_code: p.categoryCode ?? '',
    category_name: p.category ?? '',
    image_url: p.imageUrl ?? '',
  }
  showProductPicker.value = false
}

function onImageError(event: Event) {
  const target = event.target as HTMLImageElement | null
  if (!target) return
  const wrapper = target.parentElement
  if (wrapper) {
    target.remove()
    if (!wrapper.querySelector('.selected-thumb-placeholder')) {
      const placeholder = document.createElement('div')
      placeholder.className = 'selected-thumb-placeholder'
      placeholder.textContent = '无图'
      wrapper.appendChild(placeholder)
    }
  }
}
</script>

<style scoped>
.type-section {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.form-card {
  border: 1px solid #e6eaf0;
  border-radius: 0.875rem;
  padding: 0.75rem;
  background: #fff;
  min-height: 5.25rem;
}
.product-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(220px, 0.9fr);
  gap: 0.75rem;
  align-items: start;
  grid-column: 1 / -1;
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
.product-picker-btn {
  width: min(100%, 20rem);
  height: 2.75rem;
}
.form-hint {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.selected-product-card {
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  display: flex;
  gap: 0.75rem;
  align-items: center;
  min-height: 2.75rem;
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
.selected-thumb {
  width: 42px;
  height: 42px;
  border-radius: 6px;
  background: #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}
.selected-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.selected-thumb-placeholder {
  font-size: 0.75rem;
  color: #94a3b8;
}
.selected-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.selected-name {
  font-size: 0.875rem;
  font-weight: 500;
  color: #0f172a;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.selected-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  font-size: 0.75rem;
  color: #64748b;
}

/* Apple Music / iOS liquid glass create-task embedded form skin. Style-only. */
.form-card {
  border-color: rgba(148, 163, 184, 0.20);
  background:
    linear-gradient(145deg, rgba(22, 31, 47, 0.92), rgba(9, 14, 23, 0.96));
  color: #dce7f7;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.upload-card {
  background:
    radial-gradient(circle at 0% 0%, rgba(100, 210, 255, 0.10), transparent 12rem),
    linear-gradient(145deg, rgba(22, 31, 47, 0.94), rgba(9, 14, 23, 0.96));
}

.field-label {
  color: #cbd8ec;
}

.form-hint,
.selected-meta {
  color: #8fa0b8;
}

.selected-product-card,
.selected-thumb {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(8, 13, 22, 0.72);
}

.selected-name {
  color: #f8fbff;
}

.selected-thumb-placeholder {
  color: #8fa0b8;
}

.form-card :deep(input),
.form-card :deep(.relative > div),
.form-card :deep(textarea) {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(7, 12, 20, 0.82);
  color: #f8fbff;
}

.form-card :deep(input::placeholder),
.form-card :deep(textarea::placeholder) {
  color: #64748b;
}

.form-card :deep(input:focus),
.form-card :deep(textarea:focus) {
  border-color: rgba(125, 211, 252, 0.62);
  box-shadow: 0 0 0 3px rgba(100, 210, 255, 0.12);
}
@media (max-width: 760px) {
  .type-section,
  .product-card {
    grid-template-columns: 1fr;
  }
  .product-card {
    grid-column: auto;
  }
}
</style>
