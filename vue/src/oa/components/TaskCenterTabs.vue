<template>
  <nav v-if="tabs.length" class="task-tabs" aria-label="任务中心列表范围">
    <BaseButton
      v-for="tab in tabs"
      :key="tab.value"
      size="sm"
      :variant="activeTab === tab.value ? 'secondary' : 'ghost'"
      :class="{ 'task-tab-active': activeTab === tab.value }"
      :aria-pressed="activeTab === tab.value"
      @click="select(tab.value)"
    >
      {{ tab.label }}
    </BaseButton>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import { usePermission } from '@/composables/usePermission'
import { oaEnabled } from '@/oa/access'
import { listTabFromQuery, taskCenterTabLocation, taskListReturnSource, type TaskCenterTab } from '@/oa/taskNavigation'

const props = defineProps<{ active?: TaskCenterTab }>()
const emit = defineEmits<{ select: [tab: TaskCenterTab] }>()
const route = useRoute()
const router = useRouter()
const { can, canAccessMenu } = usePermission()
const canOpenList = computed(() => !oaEnabled || canAccessMenu('task_list'))
const canOpenDrafts = computed(() => oaEnabled && can('task.create'))
const tabOptions: ReadonlyArray<{ label: string; value: TaskCenterTab }> = [
  { label: '全任务', value: 'all' },
  { label: '未指派任务', value: 'pool' },
  { label: '我的任务', value: 'mine' },
  { label: '草稿箱', value: 'drafts' },
  { label: '已归档', value: 'archived' },
  { label: '已终止', value: 'terminated' },
]
const tabs = computed(() => tabOptions.filter(tab => tab.value === 'drafts' ? canOpenDrafts.value : canOpenList.value))
const activeTab = computed(() => props.active ?? (route.name === 'MeTaskDrafts' ? 'drafts' : undefined))

function select(tab: TaskCenterTab) {
  if (tab === activeTab.value) return
  if (props.active) {
    emit('select', tab)
    return
  }
  if (!canOpenList.value && tab === 'drafts') {
    void router.push('/me/task-drafts')
    return
  }
  const source = taskListReturnSource(route.query.taskFrom)
    || taskListReturnSource(router.options.history.state?.back)
    || '/tasks'
  const resolved = router.resolve(source)
  void router.push(taskCenterTabLocation(tab, resolved.query, listTabFromQuery(resolved.query), resolved.hash))
}
</script>

<style scoped>
.task-tabs { display: flex; align-items: stretch; gap: 6px; overflow-x: auto; white-space: nowrap; }
.task-tabs :deep(button) { flex: none; }
:global(html.oa-live-ui #app .oa-bar-tabs .task-tabs) { align-self: stretch; gap: 0; }
:global(html.oa-live-ui #app .oa-bar-tabs .task-tabs button) {
  min-height: 100%; padding: 0 8px; border: 0; border-bottom: 2px solid transparent;
  border-radius: 0; background: transparent; box-shadow: none;
}
:global(html.oa-live-ui #app .oa-bar-tabs .task-tabs .task-tab-active) {
  border: 0; border-bottom: 2px solid rgb(var(--yb-brand)); border-radius: 0;
  background: transparent; color: rgb(var(--yb-brand)); box-shadow: none;
}
:global(html.oa-live-ui #app .oa-bar-tabs .task-tabs button:not(.task-tab-active):hover) {
  color: rgb(var(--yb-brand)); background: rgb(var(--yb-brand-soft)); border-color: transparent;
}
:global(html.oa-live-ui #app .oa-bar-tabs .task-tabs button > span) { display: contents; }
</style>
