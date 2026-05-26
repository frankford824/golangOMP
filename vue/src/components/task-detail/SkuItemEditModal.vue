<template>
  <BaseModal
    :model-value="modelValue"
    title="编辑SKU子项"
    :show-confirm="false"
    cancel-text="关闭"
    panel-class="max-w-[min(760px,94vw)]"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <div class="sku-edit-body">
      <p v-if="errorText" class="sku-edit-error">{{ errorText }}</p>
      <div class="sku-edit-grid">
        <BaseInput
          v-model="form.productName"
          label="产品名称"
          placeholder="请输入产品名称"
          :maxlength="ERP_PRODUCT_NAME_MAX_LENGTH"
          :hint="erpProductNameHint(form.productName)"
        />
        <IIdSelector
          v-model="productIIdModel"
          label="款式编码"
          placeholder="搜索或选择款式编码"
        />
        <BaseTextarea
          v-model="form.designRequirement"
          class="sku-edit-span-2"
          label="设计要求"
          :rows="3"
          placeholder="请输入设计要求"
        />
      </div>

      <div class="sku-edit-refs">
        <p class="sku-edit-section-title">行级参考图</p>
        <AssetThumbStrip :items="referenceThumbs" empty-text="未上传" size="sm" />
      </div>

      <label class="sku-edit-check">
        <input v-model="form.triggerFiling" type="checkbox" />
        <span>保存后触发 ERP 同步评估</span>
      </label>
      <BaseInput v-model="form.remark" label="备注" placeholder="可选" />
    </div>

    <template #footer>
      <footer class="sku-edit-footer">
        <BaseButton variant="secondary" size="sm" :disabled="saving" @click="$emit('update:modelValue', false)">
          取消
        </BaseButton>
        <BaseButton variant="primary" size="sm" :loading="saving" :disabled="saving" @click="submit">
          保存
        </BaseButton>
      </footer>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import type { TaskSkuItem } from '@/domain/types/task'
import { ERP_PRODUCT_NAME_MAX_LENGTH, erpProductNameHint, isErpProductNameTooLong } from '@/domain/erp-product-name'
import { tasksApi } from '@/services/api/tasksApi'

const props = defineProps<{
  modelValue: boolean
  taskId: string
  skuItem: TaskSkuItem | null
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  saved: []
}>()

const saving = ref(false)
const errorText = ref('')
const form = ref({
  productName: '',
  productIId: '',
  designRequirement: '',
  triggerFiling: false,
  remark: '',
})

const productIIdModel = computed({
  get: () => form.value.productIId ?? '',
  set: (value: string | undefined) => {
    form.value.productIId = String(value ?? '').trim()
  },
})

const referenceThumbs = computed((): AssetThumbItem[] => {
  const refs = props.skuItem?.referenceFileRefs ?? []
  return refs
    .map((ref, idx) => {
      const src = String(ref.download_url ?? '').trim()
      if (!src) return null
      return {
        key: `sku-edit-ref-${props.skuItem?.id ?? props.skuItem?.skuCode ?? 'row'}-${idx}`,
        src,
        alt: String(ref.filename ?? `参考图 ${idx + 1}`),
        label: String(ref.filename ?? `参考图 ${idx + 1}`),
      }
    })
    .filter((row) => row != null) as AssetThumbItem[]
})

watch(
  () => [props.modelValue, props.skuItem] as const,
  () => {
    if (!props.modelValue || !props.skuItem) return
    form.value = {
      productName: String(props.skuItem.productNameSnapshot ?? ''),
      productIId: String(props.skuItem.productIId ?? ''),
      designRequirement: String(props.skuItem.designRequirement ?? ''),
      triggerFiling: false,
      remark: '',
    }
    errorText.value = ''
  },
  { immediate: true },
)

async function submit() {
  if (!props.skuItem) return
  if (isErpProductNameTooLong(form.value.productName)) {
    errorText.value = `产品名称不能超过 ${ERP_PRODUCT_NAME_MAX_LENGTH} 个字符，请精简后再提交，避免同步聚水潭失败`
    return
  }
  saving.value = true
  errorText.value = ''
  const payload = {
    product_name: form.value.productName.trim() || null,
    product_i_id: form.value.productIId.trim() || null,
    design_requirement: form.value.designRequirement.trim() || null,
    reference_file_refs: props.skuItem.referenceFileRefs ?? [],
    trigger_filing: form.value.triggerFiling === true || undefined,
    remark: form.value.remark.trim() || undefined,
  }
  try {
    if (typeof props.skuItem.id === 'number') {
      await tasksApi.patchSkuItem(props.taskId, props.skuItem.id, payload)
    } else {
      throw new Error('missing_sku_item_id')
    }
  } catch {
    // 后端尚未开放 sku-items PATCH 时，降级为任务级接口。
    try {
      await tasksApi.patchProductInfo(props.taskId, {
        product_name: payload.product_name,
        i_id: payload.product_i_id,
        trigger_filing: payload.trigger_filing,
        remark: payload.remark,
      })
      await tasksApi.patchBusinessInfo(props.taskId, {
        design_requirement: payload.design_requirement,
        remark: payload.remark,
      })
    } catch (err) {
      errorText.value = err instanceof Error ? err.message : '保存失败'
      return
    }
  } finally {
    saving.value = false
  }
  emit('saved')
  emit('update:modelValue', false)
}
</script>

<style scoped>
.sku-edit-body {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.sku-edit-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.sku-edit-span-2 {
  grid-column: 1 / -1;
}

.sku-edit-section-title {
  margin: 0 0 0.35rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: #475467;
}

.sku-edit-check {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
  color: #475467;
}

.sku-edit-error {
  margin: 0;
  padding: 0.5rem 0.6rem;
  border: 1px solid #fecaca;
  border-radius: 0.5rem;
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.8125rem;
}

.sku-edit-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 680px) {
  .sku-edit-grid {
    grid-template-columns: 1fr;
  }
}
</style>
