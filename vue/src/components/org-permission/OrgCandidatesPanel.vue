<template>
  <section class="right-panel">
    <h3 class="panel-title">候选成员</h3>
    <div class="tabs">
      <button
        type="button"
        class="tab"
        :class="{ active: tab === 'ungrouped' }"
        @click="tab = 'ungrouped'"
      >
        未分组成员
      </button>
      <button
        type="button"
        class="tab"
        :class="{ active: tab === 'search' }"
        @click="tab = 'search'"
      >
        搜索结果
      </button>
    </div>
    <BaseInput
      v-if="tab === 'search'"
      v-model="localSearch"
      label="筛选"
      placeholder="用户名 / 姓名"
      class="mb-2"
    />
    <div class="list-scroll">
      <template v-if="tab === 'ungrouped'">
        <BaseEmptyState
          v-if="!ungrouped.length"
          title="当前没有未分组成员"
          description="所有用户均已归属到小组，或暂无用户数据。"
        />
        <div v-for="u in ungrouped" v-else :key="u.id" class="user-row">
          <div class="user-meta">
            <span class="u-name">{{ displayName(u) }}</span>
            <span class="u-sub">{{ u.username ?? '—' }}</span>
            <span class="u-roles">{{ rolesText(u) }}</span>
          </div>
          <BaseButton
            size="sm"
            variant="secondary"
            :disabled="!canAct"
            @click="$emit('join', u)"
          >
            加入当前组
          </BaseButton>
        </div>
      </template>
      <template v-else>
        <BaseEmptyState
          v-if="!searchFiltered.length"
          title="未找到相关成员"
          description="尝试更换关键词，或确认用户已在系统中。"
        />
        <div v-for="u in searchFiltered" :key="u.id" class="user-row">
          <div class="user-meta">
            <span class="u-name">{{ displayName(u) }}</span>
            <span class="u-sub">{{ u.username ?? '—' }}</span>
            <span class="u-loc">{{ formatUserGroupDisplay(u) }}</span>
            <span class="u-roles">{{ rolesText(u) }}</span>
          </div>
          <div class="row-actions">
            <BaseButton
              v-if="actionState(u) === 'in_current_group'"
              size="sm"
              variant="ghost"
              disabled
            >
              已在本组
            </BaseButton>
            <BaseButton
              v-else-if="actionState(u) === 'join'"
              size="sm"
              variant="secondary"
              :disabled="!canAct"
              @click="$emit('join', u)"
            >
              加入当前组
            </BaseButton>
            <BaseButton
              v-else
              size="sm"
              variant="secondary"
              :disabled="!canAct"
              @click="$emit('move', u)"
            >
              移动到当前组
            </BaseButton>
            <BaseButton
              v-if="actionState(u) !== 'in_current_group' && hasLocation(u)"
              size="sm"
              variant="ghost"
              @click="$emit('locate', u)"
            >
              定位到所在组
            </BaseButton>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { OrgUser } from '@/domain/types/org-membership'
import type { CandidateUserActionState } from '@/domain/types/org-membership'
import {
  userDisplayName,
  formatUserGroupDisplay,
  filterCandidateUsers,
  userDepartmentLabel,
  userTeamLabel,
  parseGroupKey,
} from '@/domain/org-membership'
import { formatWorkflowRolesForDisplay } from '@/domain/user-workflow-roles'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'

const props = defineProps<{
  users: OrgUser[]
  selectedGroupKey: string | null
  /** 顶部搜索框同步到「搜索结果」tab */
  headerSearch: string
  canAct: boolean
}>()

defineEmits<{
  join: [u: OrgUser]
  move: [u: OrgUser]
  locate: [u: OrgUser]
}>()

const tab = ref<'ungrouped' | 'search'>('ungrouped')
const localSearch = ref('')

watch(
  () => props.headerSearch,
  (v) => {
    if (v.trim()) tab.value = 'search'
    localSearch.value = v
  },
)

const ungrouped = computed(() =>
  props.users.filter((u) => !userTeamLabel(u)),
)

const searchFiltered = computed(() =>
  filterCandidateUsers(localSearch.value || props.headerSearch, props.users),
)

function rolesText(u: OrgUser) {
  return formatWorkflowRolesForDisplay(u.roles)
}

function displayName(u: OrgUser) {
  return userDisplayName(u)
}

function actionState(u: OrgUser): CandidateUserActionState {
  if (!props.selectedGroupKey) return 'join'
  const { departmentName, teamName } = parseGroupKey(props.selectedGroupKey)
  if (userDepartmentLabel(u) === departmentName && userTeamLabel(u) === teamName) {
    return 'in_current_group'
  }
  if (userTeamLabel(u)) return 'move'
  return 'join'
}

function hasLocation(u: OrgUser) {
  return !!(userTeamLabel(u) && userDepartmentLabel(u))
}
</script>

<style scoped>
.right-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}
.panel-title {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #0f172a;
}
.tabs {
  display: flex;
  gap: 0.25rem;
  margin-bottom: 0.5rem;
}
.tab {
  flex: 1;
  padding: 0.35rem 0.25rem;
  font-size: 0.6875rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.375rem;
  background: #fff;
  cursor: pointer;
  color: #64748b;
}
.tab.active {
  border-color: #22c55e;
  background: #f0fdf4;
  color: #166534;
  font-weight: 600;
}
.list-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  padding: 0.35rem;
  background: #fafafa;
}
.user-row {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.45rem;
  margin-bottom: 0.35rem;
  background: #fff;
  border: 1px solid #f1f5f9;
  border-radius: 0.375rem;
}
.user-meta {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  font-size: 0.6875rem;
}
.u-name {
  font-weight: 600;
  color: #0f172a;
}
.u-sub,
.u-loc,
.u-roles {
  color: #64748b;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
