<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">用户管理</h1>
    <table class="mt-3 w-full border-collapse text-sm">
      <thead class="bg-[var(--v1-bg-surface-soft)] text-[var(--v1-text-secondary)]">
        <tr>
          <th class="px-2 py-1 text-left">姓名</th>
          <th class="px-2 py-1 text-left">部门</th>
          <th class="px-2 py-1 text-left">团队</th>
          <th class="px-2 py-1 text-left">角色</th>
          <th class="px-2 py-1 text-left">状态</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in users" :key="item.id" class="border-t border-[var(--v1-border)]">
          <td class="px-2 py-1">{{ item.name }}</td>
          <td class="px-2 py-1">{{ item.department }}</td>
          <td class="px-2 py-1">{{ item.team }}</td>
          <td class="px-2 py-1">{{ formatUserRoleForDisplay(item.role) }}</td>
          <td class="px-2 py-1">{{ item.is_active ? '启用' : '停用' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { formatUserRoleForDisplay } from '@/domain/user-workflow-roles'
import { usersApi } from '@/services/api/usersApi'

const users = ref<Array<{ id: string; name: string; department: string; team: string; role: string; is_active: boolean }>>([])

onMounted(async () => {
  const res = await usersApi.list()
  users.value = (res.data as { items?: Array<{ id: string; name: string; department: string; team: string; role: string; is_active: boolean }> }).items ?? []
})
</script>
