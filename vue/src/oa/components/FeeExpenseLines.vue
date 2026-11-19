<script setup lang="ts">
import BaseSelect from '@/components/base/BaseSelect.vue'
import OaDateField from './OaDateField.vue'
import { dateInChina } from '../api'
import { newExpenseLine, type ExpenseInput } from '../feeApplication'

const lines = defineModel<ExpenseInput[]>({ required: true })
defineProps<{ error?: string }>()
const emit = defineEmits<{ edit: [] }>()
const categories = ['交通', '住宿', '餐饮', '办公', '采购', '服务费', '其他']
const categoryOptions = (current: string) => [...new Set([...categories, ...(current ? [current] : [])])]
  .map(value => ({ value, label: value }))
</script>

<template>
  <fieldset class="oa-fieldset fee-lines" :aria-describedby="error ? 'fee-error-items' : undefined">
    <legend>费用明细</legend>
    <fieldset v-for="(line, index) in lines" :key="line.key" class="oa-fieldset oa-form-row">
      <legend>明细 {{ index + 1 }}</legend>
      <BaseSelect :model-value="line.category" label="费用科目 *" placeholder="选择科目" :options="categoryOptions(line.category)" @update:model-value="line.category = String($event); emit('edit')" />
      <OaDateField v-model="line.occurred_on" label="发生日期" :max="dateInChina()" required @update:model-value="emit('edit')" />
      <label><span>含税金额（元）<i aria-hidden="true">*</i></span><input v-model="line.amount" inputmode="decimal" placeholder="0.00" required /></label>
      <label><span>税额（元）</span><input v-model="line.tax" inputmode="decimal" placeholder="0.00" /></label>
      <label><span>发票号码</span><input v-model="line.invoice_no" maxlength="64" autocomplete="off" /></label>
      <label><span>明细说明</span><input v-model="line.description" maxlength="1000" /></label>
      <div v-if="lines.length > 1" class="oa-actions"><button type="button" @click="lines = lines.filter(item => item.key !== line.key)">删除第 {{ index + 1 }} 条</button></div>
    </fieldset>
    <div class="oa-actions"><button type="button" :disabled="lines.length >= 100" @click="lines = [...lines, newExpenseLine()]">＋ 添加费用明细</button></div>
    <small v-if="error" id="fee-error-items" class="fee-error" role="alert">{{ error }}</small>
  </fieldset>
</template>

<style scoped>
.fee-lines { display: grid; gap: 12px; }
</style>
