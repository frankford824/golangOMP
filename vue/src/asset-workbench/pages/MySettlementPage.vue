<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Gift, RefreshCw, WalletCards } from 'lucide-vue-next'

import { assetWorkbenchApi, type MySettlementMonthRow, type MySettlementResult } from '@aw/shared/api/assetWorkbenchApi'
import { formatInt, formatMoney } from '@aw/shared/format/number'

const loading = ref(false)
const error = ref('')
const result = ref<MySettlementResult | null>(null)

const currentAmount = computed(() => formatMoney(result.value?.estimated_net_amount ?? 0))
const months = computed(() => result.value?.months ?? [])

function formatMonthLabel(month: string) {
  const [year, value] = month.split('-')
  if (!year || !value) return month
  return `${year}年${value}月`
}

function normalPieceworkAmount(month: MySettlementMonthRow) {
  return month.gross_amount - month.deduction_amount + month.welfare_amount + month.adjustment_amount
}

function supplementAmount(month: MySettlementMonthRow) {
  return month.supplement_amount
}

function supplementLabel(month: MySettlementMonthRow) {
  return month.supplement_amount > 0 ? '有补交' : '0'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    result.value = await assetWorkbenchApi.mySettlement()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '收入加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="aw-page-stack aw-simple-income">
    <div class="aw-console-hero">
      <div class="aw-console-hero__head">
        <div>
          <p class="aw-eyebrow">我的收入</p>
          <h1 class="aw-console-hero__title">这个月能拿</h1>
        </div>
        <button class="aw-console-button" type="button" :disabled="loading" @click="load">
          <RefreshCw :size="16" aria-hidden="true" />
          <span>刷新</span>
        </button>
      </div>
      <div class="aw-simple-income__amount">{{ currentAmount }}</div>
      <p class="aw-simple-income__hint">这里只显示你的金额。每个月固定两条：正常作品工资、补交作品工资。</p>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>

    <div class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">按月查看</p>
          <h3>我的明细</h3>
        </div>
      </div>
      <div v-if="months.length" class="aw-simple-income__list">
        <article v-for="month in months" :key="month.business_month" class="aw-simple-income__row">
          <header class="aw-simple-income__month">
            <div>
              <strong>{{ formatMonthLabel(month.business_month) }}</strong>
              <small>{{ month.confirmed ? '这个月已经确认' : '这个月还在累计' }}</small>
            </div>
            <span class="aw-chip" :class="month.confirmed ? 'aw-chip--success' : 'aw-chip--warn'">
              {{ month.confirmed ? '已确认' : '进行中' }}
            </span>
          </header>

          <div class="aw-pay-slip-list" aria-label="工资条">
            <section class="aw-pay-slip">
              <div class="aw-pay-slip__icon" aria-hidden="true">
                <WalletCards :size="20" />
              </div>
              <div class="aw-pay-slip__copy">
                <strong>正常作品工资</strong>
                <span>{{ formatInt(month.item_count) }} 个作品 · {{ formatInt(month.page_count) }} 张</span>
                <small v-if="month.deduction_amount > 0">已扣 {{ formatMoney(month.deduction_amount) }}</small>
              </div>
              <b class="aw-cell-money">{{ formatMoney(normalPieceworkAmount(month)) }}</b>
            </section>

            <section class="aw-pay-slip">
              <div class="aw-pay-slip__icon aw-pay-slip__icon--supplement" aria-hidden="true">
                <Gift :size="20" />
              </div>
              <div class="aw-pay-slip__copy">
                <strong>补交作品工资</strong>
                <span>漏交后补的作品会单独列在这里</span>
                <small>{{ supplementLabel(month) }}</small>
              </div>
              <b class="aw-cell-money">{{ formatMoney(supplementAmount(month)) }}</b>
            </section>
          </div>

          <footer class="aw-simple-income__total">
            <span>这个月合计</span>
            <strong>{{ formatMoney(month.net_amount) }}</strong>
          </footer>
        </article>
      </div>
      <div v-else class="aw-empty-state">
        <h3>还没有收入记录</h3>
        <p>交过作品后，这里会显示每个月的金额。</p>
      </div>
    </div>
  </section>
</template>
