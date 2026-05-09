/**
 * 页面职责：规则及模板配置页
 * v0.6 对齐：FRONTEND_ALIGNMENT_v0.5(1).md G 节、I 节
 * 
 * 核心业务规则（来自 Prompt.md）：
 *   - 编号规则用于新品开发生成 SKU、定制单号等
 *   - 支持预览功能，实时生成示例 SKU 用于任务创建
 *   - 规则字段：名称、前缀、日期格式、地点编码、业务类型、自增位、dailyReset、enabled
 * 
 * 主要 Store：useRulesStore
 * 预留接口：GET /api/rules、POST /api/rules、GET /api/rules/preview (mock)
 * 
 * 当前状态：已迁移 Base 组件，列表与编辑弹窗响应式完整
 * 维护注意 / 风险点：
 *   - 预览逻辑依赖实时计算，规则变更需同步测试预览结果
 *   - 编辑弹窗使用 RuleEditorForm，改动需检查 defineExpose 兼容
 */
<template>
  <div class="rule-config-view">
    <div class="page-header">
      <h2 class="page-title">规则及模板</h2>
    </div>
    <div class="content-grid">
      <div class="list-section">
        <h4>规则列表</h4>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>规则名称</th>
                <th>类型</th>
                <th>前缀</th>
                <th>启用</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rulesStore.list"
                :key="row.id"
                class="row-click"
                :class="{ selected: selectedRuleId === row.id }"
                @click="selectRule(row)"
              >
                <td>{{ row.name }}</td>
                <td>{{ ruleTypeLabel(row) }}</td>
                <td>{{ row.prefix }}</td>
                <td>{{ row.enabled ? '是' : '否' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="edit-section">
        <h4>规则编辑</h4>
        <RuleEditorForm ref="editorRef" :rule="selectedRule" />
        <div class="edit-actions">
          <p v-if="saveError" class="save-error">{{ saveError }}</p>
          <BaseButton
            variant="primary"
            size="sm"
            :disabled="!selectedRule || saving"
            :loading="saving"
            @click="saveRule"
          >
            保存规则
          </BaseButton>
        </div>
        <RulePreviewPanel :rule="previewRule" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { CodeRule } from '@/types'
import { useRulesStore } from '@/stores/rules'
import RuleEditorForm from '@/components/rules/RuleEditorForm.vue'
import RulePreviewPanel from '@/components/rules/RulePreviewPanel.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const rulesStore = useRulesStore()
const selectedRuleId = ref<string | null>(null)
const editorRef = ref<InstanceType<typeof RuleEditorForm> | null>(null)
const saving = ref(false)
const saveError = ref('')

const selectedRule = computed(() =>
  selectedRuleId.value ? rulesStore.getById(selectedRuleId.value) ?? null : null
)

/** 合并 store 规则与表单编辑中的值，用于预览（编辑时实时反映未保存的修改） */
const previewRule = computed(() => {
  const base = selectedRule.value
  const editor = editorRef.value
  if (!base) return null
  const form = (editor as { form?: Record<string, unknown> } | undefined)?.form
  if (!form) return base
  return {
    ...base,
    name: form.name ?? base.name,
    ruleType: form.ruleType ?? base.ruleType,
    prefix: form.prefix ?? base.prefix,
    dateFormat: form.dateFormat ?? base.dateFormat,
    locationCode: form.locationCode ?? base.locationCode,
    bizTypeCode: form.bizTypeCode ?? base.bizTypeCode,
    sequenceDigits: form.sequenceDigits ?? base.sequenceDigits,
    dailyReset: form.dailyReset ?? base.dailyReset,
    enabled: form.enabled ?? base.enabled,
  } as CodeRule
})

onMounted(() => rulesStore.loadRules())

function selectRule(rule: CodeRule) {
  selectedRuleId.value = rule.id
  saveError.value = ''
}

async function saveRule() {
  const editor = editorRef.value
  const rule = selectedRule.value
  if (!editor || !rule) return
  // 使用 getFormSnapshot 确保读取表单最新值，避免 watch 等导致 prefix 被覆盖
  const getSnapshot = (editor as { getFormSnapshot?: () => Record<string, unknown> }).getFormSnapshot
  const formSnapshot = getSnapshot ? getSnapshot() : (editor as { form: Record<string, unknown> }).form
  const toSave = { ...rule, ...formSnapshot } as CodeRule
  if (import.meta.env.DEV) {
    console.log('[saveRule] formSnapshot.prefix:', formSnapshot.prefix, 'toSave.prefix:', toSave.prefix)
  }
  saving.value = true
  saveError.value = ''
  try {
    await rulesStore.saveRule(toSave)
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '保存失败，请稍后重试'
  } finally {
    saving.value = false
  }
}

/** v0.6 对齐：模板类型与 ruleType 的展示标签 */
function ruleTypeLabel(row: CodeRule) {
  const templateLabels: Record<string, string> = {
    'cost-pricing': '成本核算',
    'product-code': '产品编码',
    'short-name': '商品简称',
  }
  if (templateLabels[row.id]) return templateLabels[row.id]
  const m: Record<CodeRule['ruleType'], string> = {
    taskNo: '任务号',
    sku: '新品 SKU',
    outsourceNo: '定制单号',
    handoverNo: '交班单号',
  }
  return m[row.ruleType] ?? row.ruleType
}
</script>

<style scoped>
.rule-config-view {
  padding: 0;
}
.page-header {
  margin-bottom: 1rem;
}
.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: #0f172a;
}
.content-grid {
  display: grid;
  grid-template-columns: 1fr 360px;
  gap: 1.5rem;
}
.list-section,
.edit-section {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 1rem;
}
.list-section h4,
.edit-section h4 {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0f172a;
}
.table-wrap {
  overflow-x: auto;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.data-table th,
.data-table td {
  padding: 0.5rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid #e2e8f0;
}
.data-table th {
  background: #f8fafc;
  font-weight: 600;
  color: #334155;
}
.data-table tbody tr:nth-child(even) {
  background: #fafafa;
}
.row-click {
  cursor: pointer;
}
.row-click:hover,
.row-click.selected {
  background: #f0fdf4 !important;
}
.edit-actions {
  margin: 0.75rem 0;
}
.save-error {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  color: #dc2626;
}
.edit-section .preview-panel {
  margin-bottom: 1rem;
}
</style>
