<script setup lang="ts">
import { money } from '../api'
import { feeDateTime } from '../feeDisplay'

type Event = {
  action: string; comment?: string; at?: string; created_at?: string; operator_id?: number; actor_id?: number
  operator_name?: string; operator_no?: number | null; operator_department?: string; node_name?: string
  before_amount_cents?: number | null; after_amount_cents?: number | null; adjustment_reason?: string
  bank_reference?: string; payment_date?: string
}
defineProps<{ entries?: Event[] }>()
const labels: Record<string, string> = { submit:'提交申请', approve:'同意', return:'退回修改', reject:'拒绝', resubmit:'重新提交', payment:'登记付款', withdraw:'撤回', void:'作废' }
const amount = (value: number) => money((value / 100).toFixed(2))
</script>
<template>
  <section class="fee-audit" aria-label="操作记录"><h3>操作记录</h3>
    <p v-if="!entries?.length">暂无可查看的操作记录</p>
    <ol v-else>
      <li v-for="(entry, index) in entries" :key="index">
        <div><strong>{{ labels[entry.action] ?? entry.action }}</strong>
          <span v-if="entry.node_name">{{ entry.node_name }}</span>
          <time>{{ feeDateTime(entry.created_at || entry.at || '') }}</time>
        </div>
        <p>{{ entry.operator_name || ('账号 #' + (entry.operator_id || entry.actor_id || '—') + '（未留存姓名）') }}
          <span v-if="entry.operator_department"> · {{ entry.operator_department }}</span><span v-if="entry.operator_no != null"> · 工号 {{ entry.operator_no }}</span>
        </p>
        <p v-if="entry.comment">意见　{{ entry.comment }}</p>
        <p v-if="entry.before_amount_cents != null && entry.after_amount_cents != null && entry.before_amount_cents !== entry.after_amount_cents">
          金额　{{ amount(entry.before_amount_cents) }} → {{ amount(entry.after_amount_cents) }}
        </p>
        <p v-if="entry.adjustment_reason">核减说明　{{ entry.adjustment_reason }}</p>
        <p v-if="entry.bank_reference">交易流水　{{ entry.bank_reference }}</p>
      </li>
    </ol>
  </section>
</template>
<style scoped>
.fee-audit h3 { margin:12px 0 8px; }
.fee-audit ol { list-style:none; margin:0 0 8px 4px; padding:0; }
.fee-audit li { position:relative; border-left:1px solid var(--yb-line-structure); padding:0 0 12px 16px; break-inside:avoid; }
.fee-audit li::before { content:''; position:absolute; left:-4px; top:7px; width:7px; height:7px; border-radius:50%; background:var(--oa-accent); }
.fee-audit li > div { display:flex; flex-wrap:wrap; gap:8px; align-items:baseline; }
.fee-audit p { margin:3px 0 0; white-space:pre-wrap; overflow-wrap:anywhere; }
.fee-audit time { color:var(--oa-secondary); font-family:var(--yb-font-mono); font-size:12px; }
</style>
