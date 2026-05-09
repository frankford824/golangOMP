<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">组织总览</h1>
    <div class="mt-3 grid grid-cols-1 gap-2 md:grid-cols-3">
      <article class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3">
        <p class="text-xs text-[var(--v1-text-muted)]">部门数</p>
        <p class="text-lg font-semibold text-[var(--v1-text-primary)]">{{ departments.length }}</p>
      </article>
      <article class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3">
        <p class="text-xs text-[var(--v1-text-muted)]">团队数</p>
        <p class="text-lg font-semibold text-[var(--v1-text-primary)]">{{ teams.length }}</p>
      </article>
      <article class="rounded-lg border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] p-3">
        <p class="text-xs text-[var(--v1-text-muted)]">成员数</p>
        <p class="text-lg font-semibold text-[var(--v1-text-primary)]">{{ users.length }}</p>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchOrgOwnershipOptions } from '@/services/api/orgApi'
import { usersApi } from '@/services/api/usersApi'

const departments = ref<Array<{ id: string }>>([])
const teams = ref<Array<{ id: string }>>([])
const users = ref<Array<{ id: string }>>([])

onMounted(async () => {
  const [orgOptions, userRes] = await Promise.all([
    fetchOrgOwnershipOptions(),
    usersApi.list(),
  ])
  departments.value = orgOptions.departmentRecords
  teams.value = orgOptions.teamRecords
  users.value = (userRes.data as { items?: Array<{ id: string }> }).items ?? []
})
</script>
