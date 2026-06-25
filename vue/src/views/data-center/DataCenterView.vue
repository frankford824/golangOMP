<template>
  <div class="data-center-view">
    <div class="page-header">
      <div>
        <h2 class="page-title">数据中心</h2>
        <p class="page-subtitle">导出、业务追踪、绩效与排查</p>
      </div>
      <div class="page-header-actions">
        <BaseButton v-if="canKpi" variant="secondary" size="sm" @click="businessTrendPilotOpen = true">
          <Sparkles class="button-icon" aria-hidden="true" />
          业务热点 AI 试验
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
const businessTrendPilotOpen = ref(false)

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
