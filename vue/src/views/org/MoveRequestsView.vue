<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">异动申请</h1>
    <ul class="mt-3 space-y-2">
      <li
        v-for="item in items"
        :key="item.id"
        class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] px-3 py-2 text-sm text-[var(--v1-text-primary)]"
      >
        <div class="flex items-center justify-between gap-3">
          <span>{{ item.user_name }}：{{ item.from_department }} → {{ item.to_department }}（{{ item.status }}）</span>
          <span class="flex gap-2">
            <button type="button" class="text-xs text-[var(--v1-bg-primary)] underline" @click="approve(item.id)">通过</button>
            <button type="button" class="text-xs text-[var(--v1-danger)] underline" @click="reject(item.id)">驳回</button>
          </span>
        </div>
      </li>
    </ul>
    <p v-if="error" class="mt-3 text-xs text-[var(--v1-danger)]">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { orgMoveRequestsApi } from '@/services/api/orgApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'

const items = ref<Array<{ id: string; user_name: string; from_department: string; to_department: string; status: string }>>([])
const error = ref('')

async function load(): Promise<void> {
  const res = await orgMoveRequestsApi.list()
  items.value = (
    res.data as {
      items?: Array<{ id: string; user_name: string; from_department: string; to_department: string; status: string }>
    }
  ).items ?? []
}

async function approve(id: string): Promise<void> {
  try {
    await orgMoveRequestsApi.approve(id)
    await load()
  } catch (err) {
    error.value = resolveApiUserMessage(err)
  }
}

async function reject(id: string): Promise<void> {
  try {
    await orgMoveRequestsApi.reject(id, { reason: '前端驳回' })
    await load()
  } catch (err) {
    error.value = resolveApiUserMessage(err)
  }
}

onMounted(load)
</script>
