<template>
  <section class="center-panel">
    <div class="center-head">
      <div>
        <h3 class="panel-title">组内成员</h3>
        <p v-if="pathLabel" class="path-line">{{ pathLabel }}</p>
        <p v-else class="path-line muted">请在左侧选择小组</p>
        <p v-if="pathLabel" class="count-line">共 {{ members.length }} 人</p>
      </div>
      <div class="head-actions">
        <BaseButton size="sm" variant="primary" :disabled="!canAddMember" @click="$emit('add-member')">
          添加成员
        </BaseButton>
        <BaseButton size="sm" variant="secondary" disabled title="敬请期待">批量导入</BaseButton>
        <BaseButton size="sm" variant="ghost" disabled title="敬请期待">更多</BaseButton>
      </div>
    </div>
    <BaseEmptyState
      v-if="!selectedGroupKey"
      title="请选择小组"
      description="在左侧组织树中点击一个小组，查看其中的真实成员。"
    />
    <BaseEmptyState
      v-else-if="members.length === 0"
      title="当前组还没有成员，先添加成员吧"
      description="使用「添加成员」或右侧候选区将用户加入本组。"
    >
      <BaseButton size="sm" variant="primary" class="mt-2" @click="$emit('add-member')">
        添加成员
      </BaseButton>
    </BaseEmptyState>
    <div v-else class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>用户名</th>
            <th>姓名</th>
            <th>角色</th>
            <th v-if="showJoined">加入时间</th>
            <th class="col-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in members" :key="u.id">
            <td class="mono">{{ u.username ?? '—' }}</td>
            <td>{{ displayName(u) }}</td>
            <td class="roles">{{ rolesText(u) }}</td>
            <td v-if="showJoined">{{ joinedText(u) }}</td>
            <td class="col-actions">
              <button type="button" class="link" @click="$emit('move', u)">移动</button>
              <button type="button" class="link danger" @click="$emit('remove', u)">移除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { OrgUser } from '@/domain/types/org-membership'
import { userDisplayName } from '@/domain/org-membership'
import { formatWorkflowRolesForDisplay } from '@/domain/user-workflow-roles'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'

const props = defineProps<{
  selectedGroupKey: string | null
  pathLabel: string
  members: OrgUser[]
}>()

defineEmits<{
  'add-member': []
  move: [u: OrgUser]
  remove: [u: OrgUser]
}>()

const canAddMember = computed(() => !!props.selectedGroupKey)

const showJoined = computed(() => props.members.some((u) => !!(u.joined_at || u.created_at)))

function displayName(u: OrgUser) {
  return userDisplayName(u)
}

function rolesText(u: OrgUser) {
  return formatWorkflowRolesForDisplay(u.roles)
}

function joinedText(u: OrgUser) {
  return u.joined_at || u.created_at || '—'
}
</script>

<style scoped>
.center-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}
.center-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}
.panel-title {
  margin: 0 0 0.125rem;
  font-size: 0.9375rem;
  font-weight: 700;
  color: #0f172a;
}
.path-line {
  margin: 0;
  font-size: 0.8125rem;
  color: #334155;
  font-weight: 500;
}
.path-line.muted {
  color: #94a3b8;
  font-weight: 400;
}
.count-line {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: #64748b;
}
.head-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.75rem;
}
.data-table th,
.data-table td {
  border-bottom: 1px solid #f1f5f9;
  padding: 0.5rem 0.65rem;
  text-align: left;
}
.data-table th {
  background: #f8fafc;
  font-weight: 600;
  color: #475569;
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 0.6875rem;
}
.roles {
  color: #64748b;
  max-width: 12rem;
}
.col-actions {
  white-space: nowrap;
  width: 6.5rem;
}
.link {
  background: none;
  border: none;
  color: #2563eb;
  cursor: pointer;
  font-size: 0.75rem;
  margin-right: 0.5rem;
  padding: 0;
  text-decoration: underline;
}
.link.danger {
  color: #b91c1c;
}
</style>
