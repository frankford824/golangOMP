<template>
  <section class="info-block">
    <h4 class="text-sm font-semibold text-slate-900">成本预估（Mock）</h4>

    <template v-if="canEdit">
      <div class="mt-2 grid grid-cols-2 gap-3">
        <div>
          <label class="block text-[11px] text-slate-500 mb-1">长度（cm）</label>
          <BaseInput
            v-model="lengthInput"
            type="number"
            min="0"
            step="0.01"
            placeholder="例如 2.5"
          />
        </div>
        <div>
          <label class="block text-[11px] text-slate-500 mb-1">宽度（cm）</label>
          <BaseInput
            v-model="widthInput"
            type="number"
            min="0"
            step="0.01"
            placeholder="例如 1.8"
          />
        </div>
      </div>

      <div class="mt-3 grid grid-cols-2 gap-3 items-end">
        <div>
          <p class="text-[11px] text-slate-500">面积（㎡）</p>
          <p class="mt-1 text-sm font-medium text-slate-900">
            {{ area.toFixed(2) }}
          </p>
        </div>
        <div>
          <p class="text-[11px] text-slate-500">单价（元/㎡，写死）</p>
          <p class="mt-1 text-sm font-medium text-slate-900">
            {{ unitPrice.toFixed(2) }}
          </p>
        </div>
      </div>

      <div class="mt-3 grid grid-cols-2 gap-3 items-end">
        <div>
          <p class="text-[11px] text-slate-500">预估成本（自动）</p>
          <p class="mt-1 text-sm font-medium text-slate-900">
            {{ autoAmount.toFixed(2) }}
          </p>
        </div>
        <div>
          <label class="block text-[11px] text-slate-500 mb-1">手工覆盖金额（元，可选）</label>
          <BaseInput
            v-model="manualInput"
            type="number"
            min="0"
            step="0.01"
            placeholder="留空则使用预估值"
          />
        </div>
      </div>

      <div class="mt-3 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="text-[11px] text-slate-500">成本来源</span>
          <BaseTag v-if="isManual" variant="warning">手工</BaseTag>
          <BaseTag v-else variant="success">自动</BaseTag>
        </div>
        <div class="text-right">
          <p class="text-[11px] text-slate-500">最终成本（元）</p>
          <p class="mt-1 text-sm font-semibold text-emerald-700">
            {{ finalAmount.toFixed(2) }}
          </p>
        </div>
      </div>

      <p class="mt-2 text-[11px] text-slate-400">
        当前为预估金额，仅供参考。
      </p>
    </template>
    <div v-else class="mt-2">
      <p class="text-[11px] text-slate-500">最终成本（元）</p>
      <p class="mt-1 text-sm font-semibold text-slate-700">
        {{ finalAmount.toFixed(2) }}
      </p>
      <p class="mt-2 text-[11px] text-slate-400">无编辑权限，仅可查看。</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTag from '@/components/base/BaseTag.vue'
import { usePermission } from '@/composables/usePermission'

const { can } = usePermission()
const canEdit = computed(() => can('task.edit'))

const lengthInput = ref<string>('')
const widthInput = ref<string>('')
const manualInput = ref<string>('')

// 固定单价：10 元/㎡（mock）
const unitPrice = 10

function toNumber(value: string): number {
  const n = Number(value)
  return Number.isFinite(n) && n >= 0 ? n : 0
}

const area = computed(() => {
  const lengthCm = toNumber(lengthInput.value)
  const widthCm = toNumber(widthInput.value)
  const lengthM = lengthCm / 100
  const widthM = widthCm / 100
  return lengthM * widthM
})

const autoAmount = computed(() => area.value * unitPrice)

const manualAmount = computed(() => {
  const n = toNumber(manualInput.value)
  return n > 0 ? n : 0
})

const isManual = computed(() => manualAmount.value > 0)

const finalAmount = computed(() => (isManual.value ? manualAmount.value : autoAmount.value))
</script>

<style scoped>
.info-block {
  @apply p-4 bg-white border border-slate-200 rounded-lg;
}
</style>

