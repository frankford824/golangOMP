<template>
  <DesktopSlot target="tabs">
    <nav v-if="canOpenOverview || canOpenDesign" class="oa-task-board-tabs" aria-label="任务看板">
      <RouterLink v-if="canOpenOverview" :to="overviewTarget" :aria-current="active === 'overview' ? 'page' : undefined">
        任务运营总览
      </RouterLink>
      <RouterLink v-if="canOpenDesign" :to="designTarget" :aria-current="active === 'design' ? 'page' : undefined">
        设计部门产能
      </RouterLink>
    </nav>
  </DesktopSlot>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import { isUnassignedDesignDepartment } from '@/oa/taskBoardAccess'
import DesktopSlot from './DesktopSlot.vue'

const props = withDefaults(defineProps<{
  active: 'overview' | 'design'
  overviewTarget?: string
}>(), { overviewTarget: '/?workspace=tasks' })

const route = useRoute()
const permissions = usePermissionsStore()
const { can, hasRole, canAccessMenu } = usePermission()
const canOpenOverview = computed(() => canAccessMenu('dashboard') && can('task.view'))
const canOpenDesign = computed(() =>
  canAccessMenu('dashboard') && can('report.view') && !isUnassignedDesignDepartment(permissions) &&
  ['department_admin', 'admin', 'super_admin'].some(hasRole),
)
const overviewTarget = computed(() => props.active === 'overview' ? route.fullPath : props.overviewTarget)
const designTarget = computed(() => props.active === 'design' ? route.fullPath : {
  name: 'DesignDepartmentDashboard',
  query: { taskBoardFrom: route.fullPath },
})
</script>

<style scoped>
.oa-task-board-tabs {
  align-items: stretch;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.oa-task-board-tabs a {
  align-items: center;
  border-bottom: 2px solid transparent;
  color: rgb(var(--yb-text-muted-strong));
  display: inline-flex;
  font-size: 13px;
  min-height: 36px;
  padding-inline: 12px;
  white-space: nowrap;
}
.oa-task-board-tabs a:hover,
.oa-task-board-tabs a[aria-current=page] {
  color: var(--oa-accent);
}
.oa-task-board-tabs a[aria-current=page] {
  border-bottom-color: var(--oa-accent);
  font-weight: 500;
}
</style>
