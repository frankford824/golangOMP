<template>
  <div ref="rootEl" class="supplement rounded-md border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface-soft))] p-3">
    <h5 class="mb-2 text-xs font-semibold text-[rgb(var(--yb-text-body-strong))]">补全仓库准入信息</h5>
    <p class="mb-3 text-xs text-[rgb(var(--yb-text-secondary))] leading-relaxed">
      补全产品分类、成本单价与采购单价等仓库准入信息，保存后任务数据会自动刷新。
    </p>
    <div class="grid gap-2 sm:grid-cols-2">
      <BaseSelect
        v-model="categoryModel"
        label="产品分类编码"
        placeholder="请选择分类"
        :options="categoryOptions"
      />
      <BaseInput
        v-model.number="costPriceInput"
        type="number"
        min="0"
        step="0.001"
        label="成本单价（CNY）"
        placeholder="结单条件「成本价已录入」看此项"
      />
      <BaseInput
        v-model="costOverrideReason"
        label="成本维护原因"
        placeholder="如：仓库维护成本价"
      />
      <BaseInput
        v-model.number="procurementPriceInput"
        type="number"
        min="0"
        step="0.001"
        label="采购单价（CNY）"
        placeholder="可与成本单价相同，用于仓库准入"
      />
      <BaseInput
        v-model.number="quantityInput"
        type="number"
        min="0"
        step="1"
        label="采购数量"
        placeholder="件数"
      />
    </div>
    <BaseTextarea v-model="specText" class="mt-2" label="规格说明" :rows="3" placeholder="规格、尺寸、工艺等" />
    <p class="mt-2 text-xs leading-relaxed text-[rgb(var(--yb-text-secondary))]">
      成本单价保存后将作为人工维护成本，并请求同步 ERP；后续仍可再次修改。
    </p>
    <p v-if="saveError" class="mt-2 text-xs text-[rgb(var(--yb-danger))]">{{ saveError }}</p>
    <div class="mt-3 flex flex-wrap gap-2">
      <BaseButton size="sm" variant="primary" :loading="saving" :disabled="saving" @click="save">
        保存并刷新任务
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import type { Task } from '@/domain/types/task'
import { useTasksStore } from '@/stores/tasks'
import { tasksApi } from '@/services/api/tasksApi'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { buildCategoryPatchFields } from '@/domain/category-payload'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{ task: Task }>()

const tasksStore = useTasksStore()
const { options: categoryOptions, load: loadCategoryOptions } = useCategoryOptions({ eager: false })
const rootEl = ref<HTMLElement | null>(null)
let categoryIo: IntersectionObserver | null = null

onMounted(() => {
  void nextTick(() => {
    const el = rootEl.value
    if (!el) return
    categoryIo = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            void loadCategoryOptions()
            categoryIo?.disconnect()
            categoryIo = null
          }
        }
      },
      { root: null, rootMargin: '120px', threshold: 0.01 },
    )
    categoryIo.observe(el)
  })
})

onBeforeUnmount(() => {
  categoryIo?.disconnect()
  categoryIo = null
})

const categoryModel = ref('')
const specText = ref('')
const costOverrideReason = ref('')
const quantityInput = ref<number | undefined>(undefined)
/** 对应 Task.costPrice / PATCH business-info.cost_price，结单面板「成本价已录入」依赖此项 */
const costPriceInput = ref<number | undefined>(undefined)
const procurementPriceInput = ref<number | undefined>(undefined)

const saving = ref(false)
const saveError = ref('')

function hydrateFromTask(t: Task) {
  categoryModel.value = t.newProductCategoryCode ?? t.categoryName ?? t.category ?? ''
  specText.value = t.designRequirement ?? ''
  costOverrideReason.value = ''
  const q = t.purchaseInfo?.quantity
  quantityInput.value = typeof q === 'number' && Number.isFinite(q) ? q : undefined
  const cp = t.costPrice?.amount
  const pp = t.purchaseInfo?.purchasePrice?.amount
  costPriceInput.value = typeof cp === 'number' && Number.isFinite(cp) ? cp : undefined
  procurementPriceInput.value =
    typeof pp === 'number' && Number.isFinite(pp)
      ? pp
      : typeof cp === 'number' && Number.isFinite(cp)
        ? cp
        : undefined
}

watch(
  () => props.task,
  (t) => hydrateFromTask(t),
  { immediate: true, deep: true },
)

function procurementPatchStatus(t: Task): string {
  if (t.procurementApiStatus && t.procurementApiStatus.trim()) return t.procurementApiStatus
  if (t.purchaseInfo?.status === 'Purchased') return 'completed'
  return 'draft'
}

async function save() {
  const id = props.task.id
  saveError.value = ''
  saving.value = true
  try {
    const cat = categoryModel.value.trim()
    const spec = specText.value.trim()
    const q = quantityInput.value
    const pp = procurementPriceInput.value
    const cost = costPriceInput.value

    const bizPatch: Record<string, unknown> = {}
    Object.assign(bizPatch, buildCategoryPatchFields(cat))
    if (spec) bizPatch.spec_text = spec
    if (typeof q === 'number' && Number.isFinite(q)) bizPatch.quantity = Math.trunc(q)
    if (typeof cost === 'number' && Number.isFinite(cost)) {
      const reason = costOverrideReason.value.trim() || '仓库维护成本价'
      bizPatch.cost_price = cost
      bizPatch.manual_cost_override = true
      bizPatch.manual_cost_override_reason = reason
      bizPatch.trigger_filing = true
      bizPatch.remark = reason
    }

    const procurementUnit =
      typeof pp === 'number' && Number.isFinite(pp)
        ? pp
        : typeof cost === 'number' && Number.isFinite(cost)
          ? cost
          : undefined
    const shouldPatchProcurement =
      (typeof procurementUnit === 'number' && Number.isFinite(procurementUnit)) ||
      (typeof q === 'number' && Number.isFinite(q))

    if (Object.keys(bizPatch).length === 0 && !shouldPatchProcurement) {
      saveError.value = '请至少填写或修改一项后再保存'
      return
    }

    if (Object.keys(bizPatch).length > 0) {
      await tasksApi.patchBusinessInfo(id, bizPatch)
    }

    if (shouldPatchProcurement) {
      await tasksApi.patchTaskProcurement(id, {
        status: procurementPatchStatus(props.task),
        ...(typeof procurementUnit === 'number' && Number.isFinite(procurementUnit)
          ? { procurement_price: procurementUnit }
          : {}),
        ...(typeof q === 'number' && Number.isFinite(q) ? { quantity: Math.trunc(q) } : {}),
      })
    }

    await tasksStore.loadTaskById(id)
    await tasksStore.forceRefreshList()
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}
</script>
