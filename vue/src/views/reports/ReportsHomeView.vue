<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">报表中心（L1）</h1>
    <div v-if="loadError" class="mt-3 text-xs text-[var(--v1-danger)]">{{ loadError }}</div>
    <div v-else class="mt-3 grid grid-cols-1 gap-2 md:grid-cols-5">
      <article
        v-for="(card, idx) in cardItems"
        :key="card.key ?? String(idx)"
        class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3"
      >
        <p class="text-xs text-[var(--v1-text-muted)]">{{ card.title }}</p>
        <p class="text-lg font-semibold text-[var(--v1-text-primary)]">
          {{ card.value }}<span v-if="card.unit" class="text-sm font-normal text-[var(--v1-text-muted)]">{{ card.unit }}</span>
        </p>
      </article>
    </div>
    <div class="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-2">
      <router-link class="rounded-lg border border-[var(--v1-border)] p-3 text-sm" to="/reports/task-throughput">
        任务吞吐报表
      </router-link>
      <router-link class="rounded-lg border border-[var(--v1-border)] p-3 text-sm" to="/reports/module-dwell">
        模块停留报表
      </router-link>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { reportsApi } from '@/services/api/reportsApi'

interface L1Card {
  key?: string
  title?: string
  value?: string | number
  unit?: string
}

const loadError = ref('')
const cards = ref<L1Card[]>([])

const cardItems = computed(() => cards.value)

let timer: number | undefined

async function load(): Promise<void> {
  loadError.value = ''
  try {
    const res = await reportsApi.l1Cards()
    const body = res.data as { data?: L1Card[] } | L1Card[] | undefined
    const list = Array.isArray(body) ? body : body?.data
    cards.value = Array.isArray(list) ? list : []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
    cards.value = []
  }
}

onMounted(() => {
  void load()
  timer = window.setInterval(load, 30_000)
})

onBeforeUnmount(() => {
  if (timer !== undefined) window.clearInterval(timer)
})
</script>
