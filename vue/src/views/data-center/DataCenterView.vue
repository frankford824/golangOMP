<template>
  <div class="data-center-view">
    <div class="page-header">
      <div>
        <h2 class="page-title">数据中心</h2>
        <p class="page-subtitle">导出、业务追踪、绩效、经验观测与排查</p>
      </div>
      <div class="page-header-actions">
        <BaseButton v-if="canKpi" variant="secondary" size="sm" @click="businessTrendPilotOpen = true">
          <Sparkles class="button-icon" aria-hidden="true" />
          业务热点 AI 分析试验
        </BaseButton>
        <BaseButton variant="secondary" size="sm" @click="refreshToken++">刷新当前页</BaseButton>
      </div>
    </div>

    <BaseEmptyState
      v-if="!visibleTabs.length"
      title="暂无数据中心权限"
      description="当前账号没有导出、追踪或绩效相关权限。"
    />

    <template v-else>
      <div class="module-tabs" role="tablist" aria-label="数据中心模块" aria-orientation="horizontal">
        <button
          v-for="tab in visibleTabs"
          :key="tab.key"
          :id="tabButtonId(tab.key)"
          type="button"
          role="tab"
          class="module-tab"
          :class="{ active: activeTab === tab.key }"
          :aria-controls="tabPanelId(tab.key)"
          :aria-selected="activeTab === tab.key"
          :tabindex="activeTab === tab.key ? 0 : -1"
          @click="setActiveTab(tab.key)"
          @keydown="handleTabKeydown($event, tab.key)"
        >
          <span>{{ tab.label }}</span>
          <small>{{ tab.hint }}</small>
        </button>
      </div>

      <section
        class="module-body"
        role="tabpanel"
        :id="tabPanelId(activeTab)"
        :aria-labelledby="tabButtonId(activeTab)"
      >
        <KpiOverviewPanel v-if="activeTab === 'kpi'" :key="`kpi-${refreshToken}`" />
        <ExperienceLearningPanel v-else-if="activeTab === 'experience'" :refresh-token="refreshToken" />
        <LogsManagementView
          v-else-if="activeTab === 'business'"
          :key="`business-${refreshToken}`"
          embedded
          locked-tab="business"
        />
        <LogsManagementView
          v-else-if="activeTab === 'operation'"
          :key="`operation-${refreshToken}`"
          embedded
          locked-tab="operation"
        />
        <LogsManagementView
          v-else-if="activeTab === 'permission'"
          :key="`permission-${refreshToken}`"
          embedded
          locked-tab="permission"
        />
        <LogsManagementView
          v-else-if="activeTab === 'server'"
          :key="`server-${refreshToken}`"
          embedded
          locked-tab="server"
        />
      </section>
    </template>

    <BusinessTrendPilotModal v-model="businessTrendPilotOpen" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BusinessTrendPilotModal from '@/components/data-center/BusinessTrendPilotModal.vue'
import ExperienceLearningPanel from '@/components/data-center/ExperienceLearningPanel.vue'
import KpiOverviewPanel from '@/components/data-center/KpiOverviewPanel.vue'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import LogsManagementView from '@/views/logs/LogsManagementView.vue'

type DataCenterTab = 'kpi' | 'experience' | 'business' | 'operation' | 'permission' | 'server'

interface DataCenterTabItem {
  key: DataCenterTab
  label: string
  hint: string
}

const route = useRoute()
const router = useRouter()
const permissionsStore = usePermissionsStore()
const { can, canAccessModule, canAccessPage } = usePermission()

const refreshToken = ref(0)
const activeTab = ref<DataCenterTab>('kpi')
const businessTrendPilotOpen = ref(false)

const canTrace = computed(() => can('logs.view') || permissionsStore.hasMenu('logs_center'))
const canServer = computed(() => can('logs.server.view'))
const canKpi = computed(
  () =>
    canTrace.value ||
    permissionsStore.hasMenu('kpi') ||
    permissionsStore.hasMenu('report_center') ||
    permissionsStore.hasMenu('finance'),
)
const canExperience = computed(
  () =>
    can('reports.experience.view') ||
    canAccessPage('data_center_experience') ||
    canAccessModule('experience_learning'),
)

const visibleTabs = computed<DataCenterTabItem[]>(() => {
  const tabs: DataCenterTabItem[] = []
  if (canKpi.value) tabs.push({ key: 'kpi', label: '绩效看板', hint: '运营 / 设计 / 审核' })
  if (canExperience.value) tabs.push({ key: 'experience', label: '经验观测', hint: '监督样本 / 侧路治理' })
  if (canTrace.value) tabs.push({ key: 'business', label: '业务追踪', hint: '人员与任务链路' })
  if (canTrace.value) tabs.push({ key: 'operation', label: '操作明细', hint: '任务 / 导出 / 集成' })
  if (canTrace.value) tabs.push({ key: 'permission', label: '权限明细', hint: '人员权限变更' })
  if (canServer.value) tabs.push({ key: 'server', label: '系统排查', hint: '服务器日志' })
  return tabs
})

function normalizeTab(raw: unknown): DataCenterTab | '' {
  const value = String(raw ?? '').trim()
  if (value === 'logs' || value === 'trace') return 'business'
  if (['kpi', 'experience', 'business', 'operation', 'permission', 'server'].includes(value)) {
    return value as DataCenterTab
  }
  return ''
}

function firstVisibleTab(): DataCenterTab {
  return visibleTabs.value[0]?.key ?? 'kpi'
}

function canOpenTab(tab: DataCenterTab): boolean {
  return visibleTabs.value.some((item) => item.key === tab)
}

function setActiveTab(tab: DataCenterTab) {
  if (!canOpenTab(tab)) return
  activeTab.value = tab
  const nextQuery = { ...route.query, tab }
  void router.replace({ path: route.path, query: nextQuery })
}

function tabButtonId(tab: DataCenterTab): string {
  return `data-center-tab-${tab}`
}

function tabPanelId(tab: DataCenterTab): string {
  return `data-center-panel-${tab}`
}

function focusTab(tab: DataCenterTab) {
  if (typeof document === 'undefined') return
  document.getElementById(tabButtonId(tab))?.focus()
}

function handleTabKeydown(event: KeyboardEvent, tab: DataCenterTab) {
  const currentIndex = visibleTabs.value.findIndex((item) => item.key === tab)
  if (currentIndex < 0) return

  const lastIndex = visibleTabs.value.length - 1
  let nextIndex = currentIndex
  if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
    nextIndex = currentIndex === lastIndex ? 0 : currentIndex + 1
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
    nextIndex = currentIndex === 0 ? lastIndex : currentIndex - 1
  } else if (event.key === 'Home') {
    nextIndex = 0
  } else if (event.key === 'End') {
    nextIndex = lastIndex
  } else {
    return
  }
  event.preventDefault()
  const nextTab = visibleTabs.value[nextIndex]?.key
  if (!nextTab) return
  setActiveTab(nextTab)
  requestAnimationFrame(() => focusTab(nextTab))
}

watch(
  [() => route.query.tab, visibleTabs],
  () => {
    const requested = normalizeTab(route.query.tab)
    if (requested && canOpenTab(requested)) {
      activeTab.value = requested
      return
    }
    const fallback = firstVisibleTab()
    activeTab.value = fallback
    if (visibleTabs.value.length > 0 && route.query.tab !== fallback) {
      void router.replace({ path: route.path, query: { ...route.query, tab: fallback } })
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.data-center-view {
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
  gap: 0.75rem;
  padding-bottom: 1.5rem;
  color: rgb(var(--yb-text-navy));
  font-family: var(--yb-font-text);
  letter-spacing: 0;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.125rem 0.125rem 0;
}
.page-header-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
}
.button-icon {
  margin-right: 0.3rem;
  width: 0.9rem;
  height: 0.9rem;
}
.page-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 750;
  line-height: 1.3;
  color: rgb(var(--yb-text-navy));
  letter-spacing: 0;
}
.page-subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.75rem;
  line-height: 1.4;
  color: rgb(var(--yb-text-muted-strong));
  letter-spacing: 0;
}
.module-tabs {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
  padding: 0.125rem;
  scrollbar-width: thin;
}
.module-tab {
  display: inline-flex;
  min-width: 11rem;
  min-height: 3.1rem;
  flex: 1 1 11rem;
  flex-direction: column;
  justify-content: center;
  border: 1px solid rgb(var(--yb-border-blue));
  border-radius: 0.5rem;
  background: rgb(var(--yb-surface));
  padding: 0.55rem 0.7rem;
  text-align: left;
  color: rgb(var(--yb-text-deep));
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    background-color 0.15s ease;
}
.module-tab span {
  display: block;
  font-size: 0.8125rem;
  font-weight: 750;
  line-height: 1.25;
  letter-spacing: 0;
}
.module-tab small {
  display: block;
  margin-top: 0.2rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.6875rem;
  line-height: 1.25;
  color: rgb(var(--yb-text-blue-gray));
  letter-spacing: 0;
}
.module-tab.active {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-surface-blue-soft));
  box-shadow: inset 0 0 0 1px rgb(var(--yb-brand) / 0.16);
  color: rgb(var(--yb-brand-strong));
}
.module-body {
  min-width: 0;
}
@media (max-width: 720px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .page-header-actions {
    width: 100%;
    justify-content: stretch;
  }

  .page-header-actions :deep(button) {
    flex: 1 1 9rem;
  }
}
</style>
