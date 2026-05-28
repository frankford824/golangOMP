<template>
  <div
    v-if="showBlock"
    class="supplement rounded-lg border border-stone-200 bg-stone-50 mb-3"
    :class="compact ? 'supplement--compact p-2' : 'p-3'"
  >
    <h5 class="text-xs font-semibold text-stone-800" :class="compact ? 'mb-0.5' : 'mb-1'">补全结单所需信息</h5>
    <p
      class="text-xs text-stone-600 leading-relaxed"
      :class="compact ? 'mb-2 line-clamp-2' : 'mb-3'"
      :title="compact ? hintFull : undefined"
    >
      {{ compact ? hintShort : hintFull }}
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
        label="成本价（CNY）"
        placeholder="请输入产品成本价"
      />
      <BaseInput
        v-model="costOverrideReason"
        label="成本维护原因"
        placeholder="如：仓库维护成本价"
      />
    </div>
    <BaseTextarea
      v-model="specText"
      class="mt-2"
      label="规格说明"
      :rows="compact ? 2 : 3"
      placeholder="请填写规格、尺寸、工艺等信息"
    />
    <p class="mt-2 text-xs leading-relaxed text-stone-600">
      成本价保存后将作为人工维护成本，并请求同步 ERP；后续仍可再次修改。
    </p>
    <div
      class="rounded-md border border-slate-200 bg-white/80 text-xs text-slate-700"
      :class="compact ? 'mt-2 p-1.5' : 'mt-3 p-2'"
    >
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span class="font-medium text-slate-800">建档 / ERP</span>
        <span v-if="task.filing_status" class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[0.65rem]">
          {{ task.filing_status }}
        </span>
        <span v-if="task.last_filed_at" class="text-slate-500">
          最近建档：{{ task.last_filed_at }}
        </span>
        <BaseButton
          size="sm"
          variant="secondary"
          class="ms-auto"
          :loading="filingLoading"
          :disabled="filingLoading"
          @click="retryFiling"
        >
          重试建档同步
        </BaseButton>
        <BaseButton size="sm" variant="primary" :loading="saving" :disabled="saving" @click="save">
          保存并刷新
        </BaseButton>
      </div>
      <p v-if="task.filing_error_message" class="mt-1 text-red-600">{{ task.filing_error_message }}</p>
    </div>
    <p v-if="saveError" class="mt-2 text-xs text-red-600">{{ saveError }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { Task } from '@/domain/types/task'
import { useTasksStore } from '@/stores/tasks'
import { tasksApi } from '@/services/api/tasksApi'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { usePermission } from '@/composables/usePermission'
import { isTaskCloseFlowTerminal } from '@/domain/task-close-eligibility'
import { buildCategoryPatchFields } from '@/domain/category-payload'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = withDefaults(
  defineProps<{
    task: Task
    /** 任务详情右栏：压缩说明与操作行，减少纵向占位 */
    compact?: boolean
  }>(),
  { compact: false },
)

const hintFull =
  '结单前需补齐任务主档信息（产品分类、规格、成本价等）并完成建档。保存后任务数据会自动刷新。'
const hintShort = '结单前请补齐类目、规格、成本价等主档；保存后刷新。'

const { can } = usePermission()
const tasksStore = useTasksStore()
const { options: categoryOptions, load: loadCategoryOptions } = useCategoryOptions({ eager: false })
const categoryOptionsPrimed = ref(false)

const showBlock = computed(() => {
  const t = props.task
  if (isTaskCloseFlowTerminal(t)) return false
  return (
    (can('task.edit') || can('task.close')) &&
    !isPurchaseTask(t) &&
    (isNewProduct(t) || isOriginal(t))
  )
})

watch(
  showBlock,
  (visible) => {
    if (!visible || categoryOptionsPrimed.value) return
    categoryOptionsPrimed.value = true
    void loadCategoryOptions()
  },
  { immediate: true },
)

function isPurchaseTask(t: Task): boolean {
  return t.businessType === 'PURCHASE_TASK' || t.taskType === 'PURCHASE_TASK'
}
function isNewProduct(t: Task): boolean {
  return t.businessType === 'NEW_PRODUCT_DEV' || t.taskType === 'NEW_PRODUCT_DEV'
}
function isOriginal(t: Task): boolean {
  return t.businessType === 'ORIGINAL_PRODUCT_DEV' || t.taskType === 'ORIGINAL_PRODUCT_DEV'
}

const categoryModel = ref('')
const specText = ref('')
const costOverrideReason = ref('')
const costPriceInput = ref<number | undefined>(undefined)

const saving = ref(false)
const filingLoading = ref(false)
const saveError = ref('')

function hydrateFromTask(t: Task) {
  const cat = isNewProduct(t)
    ? (t.newProductCategoryCode ?? t.categoryName ?? t.category ?? '')
    : (t.erpIId ?? t.erpCategoryName ?? t.erpCategoryCode ?? t.categoryName ?? t.category ?? '')
  categoryModel.value = typeof cat === 'string' ? cat : ''
  specText.value = t.designRequirement ?? ''
  costOverrideReason.value = ''
  const cp = t.costPrice?.amount
  const npc = t.newProductCostUnitPrice
  const n = typeof cp === 'number' && Number.isFinite(cp) ? cp : typeof npc === 'number' && Number.isFinite(npc) ? npc : undefined
  costPriceInput.value = n
}

watch(
  () => props.task,
  (t) => hydrateFromTask(t),
  { immediate: true, deep: true },
)

async function save() {
  const id = props.task.id
  saveError.value = ''
  saving.value = true
  try {
    const cat = categoryModel.value.trim()
    const spec = specText.value.trim()
    const cost = costPriceInput.value
    const reason = costOverrideReason.value.trim() || '仓库维护成本价'

    const bizPatch: Record<string, unknown> = {}
    Object.assign(bizPatch, buildCategoryPatchFields(cat))
    if (spec) bizPatch.spec_text = spec
    if (typeof cost === 'number' && Number.isFinite(cost)) {
      bizPatch.cost_price = cost
      bizPatch.manual_cost_override = true
      bizPatch.manual_cost_override_reason = reason
      bizPatch.trigger_filing = true
      bizPatch.remark = reason
    }

    if (Object.keys(bizPatch).length === 0) {
      saveError.value = '请至少填写或修改一项后再保存'
      return
    }

    await tasksApi.patchBusinessInfo(id, bizPatch)
    await tasksStore.loadTaskById(id)
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function retryFiling() {
  const id = props.task.id
  filingLoading.value = true
  saveError.value = ''
  try {
    await tasksApi.retryFiling(id)
    await tasksStore.loadTaskById(id)
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '建档重试失败'
  } finally {
    filingLoading.value = false
  }
}
</script>
