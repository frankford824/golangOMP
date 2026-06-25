<template>
  <section class="org-page">
    <header class="org-hero">
      <div class="org-hero__icon">
        <Building2 :size="24" aria-hidden="true" />
      </div>
      <div>
        <p>我的组织</p>
        <h1>{{ departmentLabel }}</h1>
        <span>{{ teamLabel }}</span>
      </div>
    </header>

    <div class="org-summary">
      <section class="org-card">
        <div class="org-card__icon org-card__icon--blue">
          <Building2 :size="20" aria-hidden="true" />
        </div>
        <span>所属部门</span>
        <strong>{{ departmentLabel }}</strong>
      </section>
      <section class="org-card">
        <div class="org-card__icon org-card__icon--green">
          <UsersRound :size="20" aria-hidden="true" />
        </div>
        <span>所属团队</span>
        <strong>{{ teamLabel }}</strong>
      </section>
      <section class="org-card">
        <div class="org-card__icon org-card__icon--amber">
          <BadgeCheck :size="20" aria-hidden="true" />
        </div>
        <span>当前角色</span>
        <strong>{{ roleText }}</strong>
      </section>
    </div>

    <section class="org-panel">
      <div class="section-heading">
        <div>
          <h2>可管理部门</h2>
          <p>{{ managedDepartments.length ? `共 ${managedDepartments.length} 个部门` : '暂无部门管理范围' }}</p>
        </div>
        <ShieldCheck :size="20" aria-hidden="true" />
      </div>
      <div v-if="managedDepartments.length" class="chip-list">
        <span v-for="item in managedDepartments" :key="item">{{ item }}</span>
      </div>
      <p v-else class="org-empty">当前账号未配置部门管理范围</p>
    </section>

    <section class="org-panel">
      <div class="section-heading">
        <div>
          <h2>可管理团队</h2>
          <p>{{ managedTeams.length ? `共 ${managedTeams.length} 个团队` : '暂无团队管理范围' }}</p>
        </div>
        <UsersRound :size="20" aria-hidden="true" />
      </div>
      <div v-if="managedTeams.length" class="chip-list chip-list--team">
        <span v-for="item in managedTeams" :key="item">{{ item }}</span>
      </div>
      <p v-else class="org-empty">当前账号未配置团队管理范围</p>
    </section>

    <p v-if="message" class="org-message">{{ message }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { BadgeCheck, Building2, ShieldCheck, UsersRound } from 'lucide-vue-next'
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
  if (!id) return '未配置部门'
  return permissionsStore.departments.find((d) => d.id === id)?.name ?? id
})

const teamLabel = computed(() => {
  if (orgProfile.value?.teams?.length) return orgProfile.value.teams.join('、')
  if (orgProfile.value?.team) return orgProfile.value.team
  if (permissionsStore.actorTeam) return permissionsStore.actorTeam
  const id = user.value?.groupId
  if (!id) return '未配置团队'
  return permissionsStore.groups.find((g) => g.id === id)?.name ?? id
})

const managedDepartments = computed(() => {
  const values = orgProfile.value?.managed_departments ?? permissionsStore.managedDepartments
  return values.filter((item) => String(item ?? '').trim() !== '')
})

const managedTeams = computed(() => {
  const values = orgProfile.value?.managed_teams ?? permissionsStore.managedTeams
  return values.filter((item) => String(item ?? '').trim() !== '')
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

<style scoped>
.org-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.org-hero,
.org-card,
.org-panel {
  border: 1px solid var(--v1-border);
  background: rgb(var(--yb-surface));
  box-shadow: 0 16px 36px -28px rgb(var(--yb-shadow) / 0.28);
}

.org-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  border-radius: 18px;
  padding: 22px;
  background:
    linear-gradient(135deg, rgb(var(--yb-teal-soft) / 0.95), rgb(var(--yb-surface) / 0.94) 54%),
    rgb(var(--yb-surface));
}

.org-hero__icon,
.org-card__icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
}

.org-hero__icon {
  width: 50px;
  height: 50px;
  border-radius: 14px;
  background: rgb(var(--yb-teal-mint));
  color: rgb(var(--yb-teal));
}

.org-hero p {
  margin: 0;
  color: rgb(var(--yb-teal));
  font-size: 12px;
  font-weight: 800;
}

.org-hero h1 {
  margin: 3px 0 0;
  color: var(--v1-text-primary);
  font-size: 24px;
  font-weight: 800;
  letter-spacing: 0;
}

.org-hero span {
  display: block;
  margin-top: 5px;
  color: var(--v1-text-secondary);
  font-size: 13px;
}

.org-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.org-card {
  min-width: 0;
  border-radius: 14px;
  padding: 16px;
}

.org-card__icon {
  width: 38px;
  height: 38px;
  margin-bottom: 12px;
  border-radius: 11px;
}

.org-card__icon--blue {
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand));
}

.org-card__icon--green {
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-emerald));
}

.org-card__icon--amber {
  background: rgb(var(--yb-warning-soft));
  color: rgb(var(--yb-warning));
}

.org-card span {
  display: block;
  color: var(--v1-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.org-card strong {
  display: block;
  overflow: hidden;
  margin-top: 6px;
  color: var(--v1-text-primary);
  font-size: 16px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-panel {
  border-radius: 16px;
  padding: 18px;
}

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  color: rgb(var(--yb-text-muted-strong));
}

.section-heading h2 {
  margin: 0;
  color: var(--v1-text-primary);
  font-size: 16px;
  font-weight: 800;
}

.section-heading p {
  margin: 4px 0 0;
  color: var(--v1-text-secondary);
  font-size: 12px;
}

.chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 9px;
  margin-top: 16px;
}

.chip-list span {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 999px;
  background: rgb(var(--yb-brand-soft));
  padding: 6px 10px;
  color: rgb(var(--yb-brand-strong));
  font-size: 12px;
  font-weight: 800;
}

.chip-list--team span {
  border-color: rgb(var(--yb-success-border-soft));
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-teal));
}

.org-empty {
  margin: 16px 0 0;
  border: 1px dashed rgb(var(--yb-text-disabled));
  border-radius: 12px;
  background: rgb(var(--yb-surface-subtle));
  padding: 18px;
  color: var(--v1-text-secondary);
  font-size: 13px;
  text-align: center;
}

.org-message {
  margin: 0;
  color: var(--v1-danger);
  font-size: 13px;
  font-weight: 700;
}

@media (max-width: 900px) {
  .org-summary {
    grid-template-columns: 1fr;
  }
}
</style>
