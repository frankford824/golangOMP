<template>
  <section class="tree-panel">
    <h3 class="panel-title">部门与小组</h3>
    <p class="panel-hint">点击小组查看成员；点击部门行展开或收起。</p>
    <div class="tree-scroll">
      <div v-for="dept in tree" :key="dept.name" class="dept-block">
        <div
          class="dept-row"
          :class="{ 'is-open': expandedNames.has(dept.name) }"
          @click="$emit('toggle-dept', dept.name)"
        >
          <span class="chevron">{{ expandedNames.has(dept.name) ? '▼' : '▶' }}</span>
          <span class="dept-name">{{ dept.name }}</span>
          <button
            type="button"
            class="more-btn"
            title="更多"
            @click.stop="openMenu = openMenu === `d:${dept.name}` ? null : `d:${dept.name}`"
          >
            ⋯
          </button>
          <div v-if="openMenu === `d:${dept.name}`" class="menu" @click.stop>
            <button type="button" class="menu-item" @click="emitCreateGroup(dept)">创建小组</button>
            <button
              v-if="dept.storeDepartmentId"
              type="button"
              class="menu-item"
              @click="$emit('rename-dept', dept)"
            >
              重命名
            </button>
            <button
              v-if="dept.storeDepartmentId"
              type="button"
              class="menu-item danger"
              @click="$emit('delete-dept', dept)"
            >
              停用部门
            </button>
          </div>
        </div>
        <div v-show="expandedNames.has(dept.name)" class="group-list">
          <button
            v-for="g in dept.groups"
            :key="g.key"
            type="button"
            class="group-row"
            :class="{ active: selectedGroupKey === g.key }"
            @click="$emit('select-group', g.key)"
          >
            <span class="group-name">{{ g.teamName }}</span>
            <span class="count-badge">{{ groupCounts[g.key] ?? 0 }}</span>
            <span
              class="more-btn"
              role="button"
              tabindex="0"
              @click.stop="openMenu = openMenu === `g:${g.key}` ? null : `g:${g.key}`"
              @keydown.enter.prevent="openMenu = openMenu === `g:${g.key}` ? null : `g:${g.key}`"
            >
              ⋯
            </span>
            <div v-if="openMenu === `g:${g.key}`" class="menu menu-right" @click.stop>
              <button
                v-if="g.storeGroupId"
                type="button"
                class="menu-item"
                @click="$emit('rename-group', g)"
              >
                重命名
              </button>
              <button
                v-if="g.storeGroupId"
                type="button"
                class="menu-item"
                @click="$emit('move-group-dept', g)"
              >
                移动到其他部门
              </button>
              <button
                v-if="g.storeGroupId"
                type="button"
                class="menu-item danger"
                @click="$emit('delete-group', g)"
              >
                停用小组
              </button>
            </div>
          </button>
          <p v-if="!dept.groups.length" class="empty-groups">暂无小组</p>
        </div>
      </div>
      <BaseEmptyState
        v-if="!tree.length"
        title="暂无组织节点"
        description="请通过「新增」添加部门，或等待后端返回组织选项。"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { OrgTreeDepartmentNode, OrgTreeGroupNode } from '@/domain/types/org-membership'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'

defineProps<{
  tree: OrgTreeDepartmentNode[]
  selectedGroupKey: string | null
  expandedNames: Set<string>
  groupCounts: Record<string, number>
}>()

const emit = defineEmits<{
  'select-group': [key: string]
  'toggle-dept': [name: string]
  'create-group': [dept: OrgTreeDepartmentNode]
  'rename-dept': [dept: OrgTreeDepartmentNode]
  'delete-dept': [dept: OrgTreeDepartmentNode]
  'rename-group': [g: OrgTreeGroupNode]
  'move-group-dept': [g: OrgTreeGroupNode]
  'delete-group': [g: OrgTreeGroupNode]
}>()

const openMenu = ref<string | null>(null)

function emitCreateGroup(dept: OrgTreeDepartmentNode) {
  openMenu.value = null
  emit('create-group', dept)
}

function onDocClick() {
  openMenu.value = null
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.tree-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}
.panel-title {
  margin: 0 0 0.25rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #0f172a;
}
.panel-hint {
  margin: 0 0 0.5rem;
  font-size: 0.6875rem;
  color: #64748b;
  line-height: 1.35;
}
.tree-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #fafafa;
  padding: 0.375rem;
}
.dept-block + .dept-block {
  margin-top: 0.25rem;
}
.dept-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.5rem;
  border-radius: 0.375rem;
  cursor: pointer;
  position: relative;
  user-select: none;
}
.dept-row:hover {
  background: #f1f5f9;
}
.chevron {
  font-size: 0.65rem;
  color: #64748b;
  width: 0.75rem;
}
.dept-name {
  flex: 1;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #0f172a;
  text-align: left;
}
.group-list {
  padding: 0 0 0.25rem 0.5rem;
}
.group-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  width: 100%;
  padding: 0.3rem 0.45rem;
  margin-bottom: 2px;
  border: 1px solid transparent;
  border-radius: 0.375rem;
  background: #fff;
  cursor: pointer;
  font: inherit;
  text-align: left;
  position: relative;
}
.group-row:hover {
  border-color: #e2e8f0;
}
.group-row.active {
  border-color: #22c55e;
  background: #f0fdf4;
}
.group-name {
  flex: 1;
  min-width: 0;
  font-size: 0.75rem;
  color: #334155;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.count-badge {
  font-size: 0.625rem;
  font-weight: 600;
  color: #475569;
  background: #e2e8f0;
  border-radius: 9999px;
  padding: 0.05rem 0.35rem;
  flex-shrink: 0;
}
.more-btn {
  flex-shrink: 0;
  width: 1.5rem;
  height: 1.5rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: #94a3b8;
  border-radius: 0.25rem;
  cursor: pointer;
  font-size: 1rem;
  line-height: 1;
}
.more-btn:hover {
  background: #f1f5f9;
  color: #475569;
}
.menu {
  position: absolute;
  right: 0.25rem;
  top: 100%;
  margin-top: 2px;
  z-index: 20;
  min-width: 9rem;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
  padding: 0.25rem;
}
.menu-right {
  right: 0.25rem;
  left: auto;
}
.menu-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 0.35rem 0.5rem;
  font-size: 0.75rem;
  border: none;
  background: none;
  border-radius: 0.25rem;
  cursor: pointer;
  color: #334155;
}
.menu-item:hover {
  background: #f8fafc;
}
.menu-item.danger {
  color: #b91c1c;
}
.empty-groups {
  font-size: 0.6875rem;
  color: #94a3b8;
  padding: 0.25rem 0.5rem;
  margin: 0;
}
</style>
