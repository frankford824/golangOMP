<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw } from 'lucide-vue-next'

import { assetWorkbenchApi, type MySettlementResult } from '@aw/shared/api/assetWorkbenchApi'
import { formatMoney } from '@aw/shared/format/number'

const loading = ref(false)
const error = ref('')
const result = ref<MySettlementResult | null>(null)

const currentAmount = computed(() => formatMoney(result.value?.estimated_net_amount ?? 0))

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
      <p class="aw-simple-income__hint">这里只显示你的金额。月底确认后，历史月份会固定下来。</p>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>

    <div class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">按月查看</p>
          <h3>我的明细</h3>
        </div>
      </div>
      <div v-if="result?.months.length" class="aw-simple-income__list">
        <article v-for="month in result.months" :key="month.business_month" class="aw-simple-income__row">
          <div>
            <strong>{{ month.business_month }}</strong>
            <span class="aw-chip" :class="month.confirmed ? 'aw-chip--success' : 'aw-chip--warn'">
              {{ month.confirmed ? '已确认' : '进行中' }}
            </span>
          </div>
          <dl>
            <div>
              <dt>作品</dt>
              <dd>{{ month.item_count }}</dd>
            </div>
            <div>
              <dt>页数</dt>
              <dd>{{ month.page_count }}</dd>
            </div>
            <div>
              <dt>扣款</dt>
              <dd>{{ formatMoney(month.deduction_amount) }}</dd>
            </div>
            <div>
              <dt>补录</dt>
              <dd>{{ formatMoney(month.supplement_amount) }}</dd>
            </div>
            <div>
              <dt>合计</dt>
              <dd class="aw-cell-money">{{ formatMoney(month.net_amount) }}</dd>
            </div>
          </dl>
        </article>
      </div>
      <div v-else class="aw-empty-state">
        <h3>还没有收入记录</h3>
        <p>交过作品后，这里会显示每个月的金额。</p>
      </div>
    </div>
  </section>
</template>
