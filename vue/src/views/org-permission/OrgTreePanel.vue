<template>
  <div class="org-tree-panel">
    <div class="org-tree-search">
      <input
        v-model.trim="searchKeyword"
        class="org-tree-search-input"
        type="search"
        placeholder="搜索部门 / 小组"
        aria-label="搜索组织"
      />
    </div>
    <div class="org-tree-scroll">
      <button
        v-if="showAllEntry"
        type="button"
        class="org-filter-item org-filter-item--all"
        :class="{ 'is-active': allActive }"
        @click="$emit('select-all')"
      >
        <span class="org-item-name">全部组织</span>
      </button>
      <div v-if="filteredEnabledTree.length" class="org-tree-section">
        <div class="org-tree-section-title">启用组织</div>
        <div v-for="dept in filteredEnabledTree" :key="'enabled-' + dept.value" class="org-filter-dept">
          <div class="org-filter-row org-filter-row--dept">
            <button
              v-if="dept.teams.length"
              type="button"
              class="org-tree-toggle"
              :aria-label="isExpanded(dept.value) ? `折叠 ${dept.label}` : `展开 ${dept.label}`"
              :aria-expanded="isExpanded(dept.value)"
              @click.stop="toggleExpanded(dept.value)"
            >
              {{ isExpanded(dept.value) ? '▾' : '▸' }}
            </button>
            <span v-else class="org-tree-toggle org-tree-toggle--placeholder" aria-hidden="true"></span>
            <button
              type="button"
              class="org-filter-item org-filter-item--dept"
              :class="{ 'is-active': isDepartmentActive(dept.value) }"
              @click="onSelectDepartment(dept)"
            >
              <span class="org-item-name">{{ dept.label }}</span>
              <span v-if="dept.memberCount != null" class="org-count-badge">{{ dept.memberCount }}</span>
            </button>
          </div>
          <div v-if="dept.teams.length && isExpanded(dept.value)" class="org-filter-teams">
            <div v-for="team in dept.teams" :key="`enabled-${dept.value}-${team.value}`" class="org-filter-row">
              <button
                type="button"
                class="org-filter-item org-filter-item--team"
                :class="{ 'is-active': isTeamActive(dept.value, team.value) }"
                @click="$emit('select-team', dept.value, team.value)"
              >
                <span class="org-item-name">{{ team.label }}</span>
                <span v-if="team.memberCount != null" class="org-count-badge">{{ team.memberCount }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
      <div v-if="filteredDisabledTree.length" class="org-tree-section org-tree-section--disabled">
        <button
          type="button"
          class="org-tree-section-toggle"
          :aria-expanded="showDisabledSection"
          @click="showDisabledSection = !showDisabledSection"
        >
          <span>{{ showDisabledSection ? '▾' : '▸' }} 停用组织（{{ disabledEntryCount }}）</span>
          <small>历史遗留的部门与小组，可合并或删除</small>
        </button>
        <template v-if="showDisabledSection">
          <div v-for="dept in filteredDisabledTree" :key="'disabled-' + dept.value" class="org-filter-dept">
            <div class="org-filter-row org-filter-row--dept">
              <span class="org-tree-toggle org-tree-toggle--placeholder" aria-hidden="true"></span>
              <button
                type="button"
                class="org-filter-item org-filter-item--dept"
                :class="{ 'is-active': isDepartmentActive(dept.value), 'is-disabled': !dept.enabled }"
                @click="onSelectDepartment(dept)"
              >
                <span class="org-item-name">{{ dept.label }}</span>
                <span v-if="dept.memberCount != null" class="org-count-badge">{{ dept.memberCount }}</span>
                <span v-if="!dept.enabled" class="org-state-pill org-state-pill--off">停用</span>
              </button>
            </div>
            <div v-if="dept.teams.length" class="org-filter-teams">
              <div v-for="team in dept.teams" :key="`disabled-${dept.value}-${team.value}`" class="org-filter-row">
                <button
                  type="button"
                  class="org-filter-item org-filter-item--team"
                  :class="{ 'is-active': isTeamActive(dept.value, team.value), 'is-disabled': !team.enabled }"
                  @click="$emit('select-team', dept.value, team.value)"
                >
                  <span class="org-item-name">{{ team.label }}</span>
                  <span v-if="team.memberCount != null" class="org-count-badge">{{ team.memberCount }}</span>
                  <span v-if="!team.enabled" class="org-state-pill org-state-pill--off">停用</span>
                </button>
              </div>
            </div>
          </div>
        </template>
      </div>
      <p v-if="searchKeyword && !filteredEnabledTree.length && !filteredDisabledTree.length" class="org-tree-empty">
        未找到匹配「{{ searchKeyword }}」的部门或小组。
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { OrgTreeDepartment } from './orgTreeTypes'

const props = defineProps<{
  enabledTree: OrgTreeDepartment[]
  disabledTree: OrgTreeDepartment[]
  selectedDepartment: string
  selectedTeam: string
  showAllEntry: boolean
  allActive: boolean
}>()

const emit = defineEmits<{
  (e: 'select-all'): void
  (e: 'select-department', department: string): void
  (e: 'select-team', department: string, team: string): void
}>()

const searchKeyword = ref('')
const showDisabledSection = ref(false)
// 默认全部折叠只看部门行,点选或搜索时展开,解决"树太长要拉很久"的问题。
const expandedDepartments = ref(new Set<string>())

const isSearching = computed(() => searchKeyword.value !== '')

function matchTree(tree: OrgTreeDepartment[]): OrgTreeDepartment[] {
  if (!isSearching.value) return tree
  const kw = searchKeyword.value.toLowerCase()
  const out: OrgTreeDepartment[] = []
  for (const dept of tree) {
    const deptHit = dept.label.toLowerCase().includes(kw)
    const teams = deptHit ? dept.teams : dept.teams.filter((team) => team.label.toLowerCase().includes(kw))
    if (deptHit || teams.length) out.push({ ...dept, teams })
  }
  return out
}

const filteredEnabledTree = computed(() => matchTree(props.enabledTree))
const filteredDisabledTree = computed(() => matchTree(props.disabledTree))
const disabledEntryCount = computed(() =>
  props.disabledTree.reduce((sum, dept) => sum + (dept.enabled ? 0 : 1) + dept.teams.filter((t) => !t.enabled).length, 0),
)

function isExpanded(department: string): boolean {
  if (isSearching.value) return true
  return expandedDepartments.value.has(department) || props.selectedDepartment === department
}

function toggleExpanded(department: string) {
  const next = new Set(expandedDepartments.value)
  if (isExpanded(department)) {
    next.delete(department)
    // 选中态本身会强制展开;折叠已选中的部门时记录一个显式排除无意义,
    // 直接忽略即可(保持选中部门始终可见其小组)。
    if (props.selectedDepartment === department) return
  } else {
    next.add(department)
  }
  expandedDepartments.value = next
}

function isDepartmentActive(department: string): boolean {
  return props.selectedDepartment === department && !props.selectedTeam
}

function isTeamActive(department: string, team: string): boolean {
  return props.selectedDepartment === department && props.selectedTeam === team
}

function onSelectDepartment(dept: OrgTreeDepartment) {
  emit('select-department', dept.value)
  const next = new Set(expandedDepartments.value)
  next.add(dept.value)
  expandedDepartments.value = next
}

watch(searchKeyword, () => {
  if (isSearching.value) showDisabledSection.value = true
})
</script>

<style scoped>
.org-tree-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.45rem;
}

.org-tree-search-input {
  width: 100%;
  min-height: 2rem;
  padding: 0.3rem 0.6rem;
  border: 1px solid rgb(var(--yb-border-zinc));
  border-radius: 0.45rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-zinc));
  font-size: 0.75rem;
}

.org-tree-search-input:focus {
  outline: none;
  border-color: rgb(var(--yb-text-zinc-faint));
}

.org-tree-scroll {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.45rem;
}

.org-tree-section {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.25rem;
}

.org-tree-section-title {
  margin: 0.2rem 0 0.05rem;
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.6875rem;
  font-weight: 700;
}

.org-tree-section--disabled {
  padding-top: 0.35rem;
  border-top: 1px dashed rgb(var(--yb-border-zinc));
}

.org-tree-section-toggle {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
  width: 100%;
  padding: 0.3rem 0.35rem;
  border: none;
  border-radius: 0.4rem;
  background: transparent;
  color: rgb(var(--yb-text-zinc-soft));
  cursor: pointer;
  font-size: 0.6875rem;
  font-weight: 700;
  text-align: left;
}

.org-tree-section-toggle:hover {
  background: rgb(var(--yb-surface-row-even));
}

.org-tree-section-toggle small {
  color: rgb(var(--yb-text-zinc-faint));
  font-weight: 500;
}

.org-filter-dept {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.org-filter-teams {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  margin-left: 1.45rem;
  padding-left: 0.55rem;
  border-left: 1px solid rgb(var(--yb-border-zinc));
}

.org-filter-row {
  display: block;
}

.org-filter-row--dept {
  display: flex;
  align-items: center;
  gap: 0.15rem;
}

.org-tree-toggle {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 1.15rem;
  min-height: 2rem;
  border: none;
  background: transparent;
  color: rgb(var(--yb-text-zinc-faint));
  cursor: pointer;
  font-size: 0.7rem;
}

.org-tree-toggle--placeholder {
  cursor: default;
}

.org-filter-item {
  display: flex;
  width: 100%;
  min-height: 2rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.35rem;
  border: 1px solid transparent;
  border-radius: 0.45rem;
  background: transparent;
  color: rgb(var(--yb-text-zinc));
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1.35;
  padding: 0.35rem 0.55rem;
  text-align: left;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    color 0.15s ease;
}

.org-filter-item .org-item-name {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-filter-item.is-disabled {
  color: rgb(var(--yb-text-zinc-faint));
}

.org-filter-item:hover {
  background: rgb(var(--yb-surface-row-even));
  border-color: rgb(var(--yb-border-zinc));
}

.org-filter-item.is-active {
  background: rgb(var(--yb-success-soft));
  border-color: rgb(var(--yb-success-border));
  color: rgb(var(--yb-success-deep));
}

.org-filter-item--team {
  min-height: 1.75rem;
  font-size: 0.75rem;
  color: rgb(var(--yb-text-zinc-soft));
}

.org-count-badge {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  min-width: 1.35rem;
  min-height: 1rem;
  padding: 0.05rem 0.3rem;
  border-radius: 9999px;
  background: rgb(var(--yb-surface-neutral-muted));
  color: rgb(var(--yb-text-zinc-soft));
  font-size: 0.625rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.org-filter-item.is-active .org-count-badge {
  background: rgb(var(--yb-success-border));
  color: rgb(var(--yb-success-deep));
}

.org-state-pill {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  min-height: 1rem;
  padding: 0.05rem 0.35rem;
  border-radius: 9999px;
  background: rgb(var(--yb-success-soft));
  color: rgb(var(--yb-success-deep));
  font-size: 0.625rem;
  font-weight: 700;
}

.org-state-pill--off {
  background: rgb(var(--yb-surface-neutral-muted));
  color: rgb(var(--yb-text-zinc-soft));
}

.org-tree-empty {
  margin: 0.4rem 0.2rem;
  color: rgb(var(--yb-text-zinc-faint));
  font-size: 0.75rem;
}
</style>
