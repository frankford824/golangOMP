<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { money } from '../api'
import { feeDateTime, feeStatus } from '../feeDisplay'
import FeeTimeline from './FeeTimeline.vue'

type Doc = {
  id:string; state:string; applicant_id:number; applicant_name?:string; department_name?:string; team_name?:string
  applicant_employee_no?:number|null; submitted_at?:string; approved_at?:string; template:{ name:string; type:string }
  amount_cents:number; approved_amount_cents?:number|null; reason:string; site:string; channel:string; holder:string
  extra?:Record<string,string>; files?:string[]; history?:InstanceType<typeof FeeTimeline>['$props']['entries']
  payment?:{ amount_cents:number; date:string; receipt?:string; bank_reference?:string }|null
}
defineProps<{ document:Doc }>()
const printing = ref(false)
function end() { printing.value = false; window.removeEventListener('afterprint',end) }
async function archive() {
  printing.value = true
  await nextTick()
  window.addEventListener('afterprint',end)
  window.print()
}
onBeforeUnmount(end)
const amount = (cents?:number|null) => cents == null ? '—' : money((cents / 100).toFixed(2))
</script>
<template>
  <button type="button" @click="archive">打印 / 保存 PDF</button>
  <Teleport to="body">
    <article v-if="printing" id="oa-fee-print">
      <header><h1>费用审批归档单</h1><p>{{ document.template.name }} · {{ feeStatus(document.state) }}</p></header>
      <dl>
        <div><dt>单号</dt><dd>{{ document.id }}</dd></div>
        <div><dt>申请人</dt><dd>{{ document.applicant_name || '历史姓名未记录' }} · {{ document.applicant_employee_no ?? ('账号 #' + document.applicant_id) }}</dd></div>
        <div><dt>组织</dt><dd>{{ [document.department_name, document.team_name].filter(Boolean).join(' / ') || '历史组织未记录' }}</dd></div>
        <div><dt>申请日期</dt><dd>{{ feeDateTime(document.submitted_at || '') }}</dd></div>
        <div><dt>审批完成</dt><dd>{{ feeDateTime(document.approved_at || '') }}</dd></div>
        <div><dt>费用类型</dt><dd>{{ document.template.type }}</dd></div>
        <div><dt>申请金额</dt><dd>{{ amount(document.amount_cents) }}</dd></div>
        <div><dt>核准金额</dt><dd>{{ amount(document.approved_amount_cents) }}</dd></div>
        <div><dt>收款对象</dt><dd>{{ document.holder || '—' }}</dd></div>
        <div><dt>支付渠道</dt><dd>{{ ({ bank:'对公银行转账', person:'对私银行卡', wallet:'对私收款平台' } as Record<string,string>)[document.channel] || '—' }}</dd></div>
        <div><dt>用途</dt><dd>{{ document.reason || '—' }}</dd></div>
        <div v-for="(value, key) in document.extra" :key="key"><dt>{{ key }}</dt><dd>{{ value }}</dd></div>
        <template v-if="document.payment">
          <div><dt>实付日期</dt><dd>{{ document.payment.date }}</dd></div>
          <div><dt>实付金额</dt><dd>{{ amount(document.payment.amount_cents) }}</dd></div>
          <div><dt>交易流水号</dt><dd>{{ document.payment.bank_reference || '历史流水未记录' }}</dd></div>
        </template>
      </dl>
      <FeeTimeline :entries="document.history" />
      <h2>附件清单</h2><ol><li v-for="(file,i) in document.files" :key="file">凭证 {{ i+1 }} · {{ file }}</li><li v-if="document.payment?.receipt">付款回单 · {{ document.payment.receipt }}</li></ol>
    </article>
  </Teleport>
</template>
<style>
#oa-fee-print { display:none; }
@media print {
  body:has(> #oa-fee-print) { display:block; overflow:visible; height:auto; }
  body:has(> #oa-fee-print) > :not(#oa-fee-print) { display:none; }
  #oa-fee-print { display:block; margin:0 auto; width:100%; padding:8mm; color:var(--oa-ink); font:400 14px/1.5 var(--oa-font); }
  #oa-fee-print h1 { font-size:24px; }
  #oa-fee-print h2 { font-size:16px; }
  #oa-fee-print dl > div { display:grid; grid-template-columns:8em 1fr; border-bottom:1px solid var(--yb-line-hairline); padding:4px; }
  #oa-fee-print dd { margin:0; overflow-wrap:anywhere; white-space:pre-wrap; }
  #oa-fee-print h2, #oa-fee-print h3 { break-after:avoid; }
}
</style>
