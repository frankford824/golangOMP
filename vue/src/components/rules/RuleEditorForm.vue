<template>
  <div class="flex flex-col gap-3">
    <BaseInput
      v-model="form.name"
      label="规则名称"
    />
    <BaseSelect
      v-model="form.ruleType"
      label="规则类型"
      :options="ruleTypeOptions"
      class="w-full"
    />
    <BaseInput
      v-model="form.prefix"
      label="前缀"
    />
    <BaseInput
      v-model="form.dateFormat"
      label="日期格式"
      hint="例如：yyyyMMdd"
    />
    <BaseInput
      v-model="form.locationCode"
      label="地点编码"
    />
    <BaseInput
      v-model="form.bizTypeCode"
      label="业务类型编码"
    />
    <div class="flex items-center gap-2">
      <BaseInput
        v-model="sequenceDigitsInput"
        label="自增位数"
        type="number"
        class="w-24"
      />
      <span class="text-xs text-slate-400">1–6 位</span>
    </div>
    <label class="flex items-center gap-2 text-xs text-slate-700">
      <input
        v-model="form.dailyReset"
        type="checkbox"
        class="h-3 w-3 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
      />
      按日重置
    </label>
    <label class="flex items-center gap-2 text-xs text-slate-700">
      <input
        v-model="form.enabled"
        type="checkbox"
        class="h-3 w-3 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
      />
      启用
    </label>
  </div>
</template>

<script setup lang="ts">
/**
 * 组件职责：编号规则编辑表单，配置生成规则参数并实时预览 SKU 示例
 *
 * 核心业务规则（来自 Prompt.md）：
 *   - 新品开发必须按规则生成 SKU
 *   - 预览功能用于创建任务时实时生成示例 SKU
 *   - 支持 dailyReset、enabled 等开关
 *
 * 主要 Store：useRulesStore
 * 预留接口：POST /api/rules、GET /api/rules/preview (mock)
 *
 * 当前状态：已迁移 BaseInput/BaseSelect/原生 checkbox，watch props.rule 完整
 * 维护注意 / 风险点：
 *   - defineExpose({ form }) 暴露数据形状，RuleConfigView 等外部依赖此接口
 *   - 自增位数/日期格式变更需同步更新预览逻辑
 */
import { reactive, ref, watch } from 'vue'
import type { CodeRule } from '@/types'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'

const props = defineProps<{ rule: CodeRule | null }>()

const form = reactive({
  name: '',
  ruleType: 'taskNo' as CodeRule['ruleType'],
  prefix: '',
  dateFormat: '',
  locationCode: '',
  bizTypeCode: '',
  sequenceDigits: 3,
  dailyReset: true,
  enabled: true,
})

const sequenceDigitsInput = ref(String(form.sequenceDigits))

const ruleTypeOptions: BaseSelectOption[] = [
  { label: '任务号', value: 'taskNo' },
  { label: '新品 SKU', value: 'sku' },
  { label: '定制单号', value: 'outsourceNo' },
  { label: '交班单号', value: 'handoverNo' },
]

// 仅在切换规则（id 变化）时同步表单，避免保存后 rule 引用变化覆盖用户输入
watch(
  () => props.rule?.id,
  (ruleId) => {
    const r = props.rule
    if (r && ruleId) {
      form.name = r.name
      form.ruleType = r.ruleType
      form.prefix = r.prefix
      form.dateFormat = r.dateFormat
      form.locationCode = r.locationCode
      form.bizTypeCode = r.bizTypeCode
      form.sequenceDigits = r.sequenceDigits
      sequenceDigitsInput.value = String(r.sequenceDigits)
      form.dailyReset = r.dailyReset
      form.enabled = r.enabled
    }
  },
  { immediate: true },
)

watch(
  sequenceDigitsInput,
  (val) => {
    const n = Number(val)
    if (!Number.isNaN(n) && n >= 1 && n <= 6) {
      form.sequenceDigits = n
    }
  },
)

/** 获取当前表单快照，用于保存时确保读取最新值 */
function getFormSnapshot(): Record<string, unknown> {
  return {
    name: form.name,
    ruleType: form.ruleType,
    prefix: form.prefix,
    dateFormat: form.dateFormat,
    locationCode: form.locationCode,
    bizTypeCode: form.bizTypeCode,
    sequenceDigits: form.sequenceDigits,
    dailyReset: form.dailyReset,
    enabled: form.enabled,
  }
}

defineExpose({ form, getFormSnapshot })
</script>
