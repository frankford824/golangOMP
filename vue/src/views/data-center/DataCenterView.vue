<template>
  <div class="data-center-view">
    <div class="page-header">
      <div>
        <h2 class="page-title">数据中心</h2>
        <p class="page-subtitle">导出、业务追踪、绩效与排查统一入口</p>
      </div>
      <BaseButton variant="secondary" size="sm" @click="refreshToken++">刷新当前页</BaseButton>
    </div>

    <BaseEmptyState
      v-if="!visibleTabs.length"
      title="暂无数据中心权限"
      description="当前账号没有导出、追踪或绩效相关权限。"
    />

    <template v-else>
      <div class="module-tabs" aria-label="数据中心模块">
        <button
          v-for="tab in visibleTabs"
          :key="tab.key"
          type="button"
          class="module-tab"
          :class="{ active: activeTab === tab.key }"
          @click="setActiveTab(tab.key)"
        >
          <span>{{ tab.label }}</span>
          <small>{{ tab.hint }}</small>
        </button>
      </div>

      <section class="module-body">
        <KpiOverviewPanel v-if="activeTab === 'kpi'" :key="`kpi-${refreshToken}`" />
        <ExportCenterView v-else-if="activeTab === 'export'" :key="`export-${refreshToken}`" embedded />
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
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import KpiOverviewPanel from '@/components/data-center/KpiOverviewPanel.vue'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import ExportCenterView from '@/views/export/ExportCenterView.vue'
import LogsManagementView from '@/views/logs/LogsManagementView.vue'

type DataCenterTab = 'kpi' | 'export' | 'business' | 'operation' | 'permission' | 'server'

interface DataCenterTabItem {
  key: DataCenterTab
  label: string
  hint: string
}

const route = useRoute()
const router = useRouter()
const permissionsStore = usePermissionsStore()
const { can } = usePermission()

const refreshToken = ref(0)
const activeTab = ref<DataCenterTab>('kpi')

const canExport = computed(() => can('export.tasks') || permissionsStore.hasMenu('export_center'))
const canTrace = computed(() => can('logs.view') || permissionsStore.hasMenu('logs_center'))
const canServer = computed(() => can('logs.server.view'))
const canKpi = computed(
  () =>
    canTrace.value ||
    permissionsStore.hasMenu('kpi') ||
    permissionsStore.hasMenu('report_center') ||
    permissionsStore.hasMenu('finance'),
)

const visibleTabs = computed<DataCenterTabItem[]>(() => {
  const tabs: DataCenterTabItem[] = []
  if (canKpi.value) tabs.push({ key: 'kpi', label: '绩效看板', hint: '运营 / 设计 / 审核' })
  if (canExport.value) tabs.push({ key: 'export', label: '导出', hint: '任务与业务记录' })
  if (canTrace.value) tabs.push({ key: 'business', label: '业务追踪', hint: '人员与任务链路' })
  if (canTrace.value) tabs.push({ key: 'operation', label: '操作明细', hint: '任务 / 导出 / 集成' })
  if (canTrace.value) tabs.push({ key: 'permission', label: '权限明细', hint: '人员权限变更' })
  if (canServer.value) tabs.push({ key: 'server', label: '系统排查', hint: '服务器日志' })
  return tabs
})

function normalizeTab(raw: unknown): DataCenterTab | '' {
  const value = String(raw ?? '').trim()
  if (value === 'export-center' || value === 'exports') return 'export'
  if (value === 'logs' || value === 'trace') return 'business'
  if (['kpi', 'export', 'business', 'operation', 'permission', 'server'].includes(value)) {
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

watch(
  [() => route.query.tab, visibleTabs],
  () => {
    const requested = normalizeTab(route.query.tab)
    if (requested && canOpenTab(requested)) {
      activeTab.value = requested
      return
    }
    activeTab.value = firstVisibleTab()
  },
  { immediate: true },
)
</script>

<style scoped>
.data-center-view {
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
  gap: 1rem;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 700;
  color: #0f172a;
}
.page-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: #64748b;
}
.module-tabs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
  gap: 0.625rem;
}
.module-tab {
  min-height: 4.25rem;
  border: 1px solid #dbeafe;
  border-radius: 0.5rem;
  background: #fff;
  padding: 0.75rem;
  text-align: left;
  color: #334155;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    background-color 0.15s ease;
}
.module-tab span {
  display: block;
  font-size: 0.875rem;
  font-weight: 700;
}
.module-tab small {
  display: block;
  margin-top: 0.35rem;
  font-size: 0.6875rem;
  color: #64748b;
}
.module-tab.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 0 0 1px rgba(37, 99, 235, 0.18);
  color: #1d4ed8;
}
.module-body {
  min-width: 0;
}
@media (max-width: 720px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
