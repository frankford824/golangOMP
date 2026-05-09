<template>
  <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
    <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">我的组织</h1>
    <dl class="mt-3 space-y-2 text-sm text-[var(--v1-text-secondary)]">
      <div class="flex items-center justify-between">
        <dt>部门</dt>
        <dd>{{ departmentLabel }}</dd>
      </div>
      <div class="flex items-center justify-between">
        <dt>团队</dt>
        <dd>{{ teamLabel }}</dd>
      </div>
      <div class="flex items-center justify-between">
        <dt>角色</dt>
        <dd>{{ roleText }}</dd>
      </div>
      <div class="flex items-center justify-between">
        <dt>可管理部门</dt>
        <dd>{{ managedDepartmentLabel }}</dd>
      </div>
      <div class="flex items-center justify-between">
        <dt>可管理团队</dt>
        <dd>{{ managedTeamLabel }}</dd>
      </div>
    </dl>
    <p v-if="message" class="mt-3 text-xs text-[var(--v1-danger)]">{{ message }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'
import { meApi } from '@/services/api/meApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import type { MeOrgProfile } from '@/services/v1Types'
import { formatUserRoleForDisplay, formatWorkflowRolesForDisplay } from '@/domain/user-workflow-roles'

const permissionsStore = usePermissionsStore()
const user = computed(() => permissionsStore.currentUser)
const orgProfile = ref<MeOrgProfile | null>(null)
const message = ref('')
const roleText = computed(() => {
  const fromOrg = orgProfile.value?.roles
  if (fromOrg?.length) return formatWorkflowRolesForDisplay(fromOrg)
  return formatUserRoleForDisplay(user.value?.role)
})

const departmentLabel = computed(() => {
  if (orgProfile.value?.department) return orgProfile.value.department
  if (permissionsStore.actorDepartment) return permissionsStore.actorDepartment
  const id = user.value?.departmentId
  if (!id) return '-'
  return permissionsStore.departments.find((d) => d.id === id)?.name ?? id
})

const teamLabel = computed(() => {
  if (orgProfile.value?.teams?.length) return orgProfile.value.teams.join('、')
  const orgTeam = orgProfile.value?.team
  if (orgTeam) return orgTeam
  if (permissionsStore.actorTeam) return permissionsStore.actorTeam
  const id = user.value?.groupId
  if (!id) return '-'
  return permissionsStore.groups.find((g) => g.id === id)?.name ?? id
})

const managedDepartmentLabel = computed(() => {
  const values = orgProfile.value?.managed_departments ?? permissionsStore.managedDepartments
  return values.length ? values.join('、') : '-'
})

const managedTeamLabel = computed(() => {
  const values = orgProfile.value?.managed_teams ?? permissionsStore.managedTeams
  return values.length ? values.join('、') : '-'
})

function unwrapOrg(raw: unknown): MeOrgProfile | null {
  if (!raw || typeof raw !== 'object') return null
  const root = raw as { data?: unknown }
  const payload = root.data && typeof root.data === 'object' ? root.data : raw
  return payload as MeOrgProfile
}

async function loadMyOrg(): Promise<void> {
  try {
    const res = await meApi.getMyOrg()
    orgProfile.value = unwrapOrg(res.data)
  } catch (error) {
    message.value = resolveApiUserMessage(error, { fallback: '我的组织信息加载失败' })
  }
}

onMounted(loadMyOrg)
</script>
