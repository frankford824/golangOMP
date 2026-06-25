<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">团队管理</h1>
    <ul class="mt-3 space-y-2">
      <li
        v-for="item in items"
        :key="item.id"
        class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] px-3 py-2 text-sm text-[var(--v1-text-primary)]"
      >
        {{ item.name }}（{{ item.code }}）
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchOrgOwnershipOptions } from '@/services/api/orgApi'

const items = ref<Array<{ id: string; code: string; name: string }>>([])

onMounted(async () => {
  const res = await fetchOrgOwnershipOptions()
  items.value = res.teamRecords.map((item) => ({
    id: item.id,
    code: item.departmentId,
    name: item.name,
  }))
})
</script>
