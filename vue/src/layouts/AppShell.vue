<template>
  <div class="app-shell bg-[#f5f6f8] font-body text-gray-900 flex overflow-hidden h-[100dvh]">
    <!-- Light Slim Sidebar -->
    <aside
      class="sidebar hidden md:flex flex-col h-full w-20 bg-white border-r border-gray-200 transition-all duration-300 overflow-hidden group hover:w-64"
      @transitionend="onSidebarTransitionEnd"
    >
      <div class="flex items-center gap-3 px-6 h-16 mb-4">
        <div class="w-8 h-8 rounded-lg bg-blue-50 border border-blue-100 flex items-center justify-center flex-shrink-0">
          <span class="material-symbols-outlined text-blue-600 text-lg">architecture</span>
        </div>
        <div class="opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">
          <h2 class="font-headline font-extrabold text-gray-900 text-sm tracking-tight">永箔运营</h2>
        </div>
      </div>
      <nav class="flex-1 px-4 space-y-4 overflow-y-auto custom-scrollbar">
        <template v-if="workbenchMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">工作台</div>
          <router-link
            v-for="menu in workbenchMenus"
            :key="menu.key"
            :to="resolveMenuTo(menu)"
            :active-class="menu.exact ? '' : 'active'"
            :exact-active-class="menu.exact ? 'active' : ''"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-gray-500 hover:text-gray-900 hover:bg-gray-100 sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menu.label }}</span>
            <span v-if="menu.badge && menu.badge() > 0" class="nav-badge-p2">{{ menu.badge() }}</span>
          </router-link>
        </template>
        <template v-if="businessMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">业务处理</div>
          <router-link
            v-for="menu in businessMenus"
            :key="menu.key"
            :to="menu.to"
            active-class="active"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-gray-500 hover:text-gray-900 hover:bg-gray-100 sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menu.label }}</span>
          </router-link>
        </template>
        <template v-if="dataMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">数据与配置</div>
          <router-link
            v-for="menu in dataMenus"
            :key="menu.key"
            :to="menu.to"
            active-class="active"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-gray-500 hover:text-gray-900 hover:bg-gray-100 sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menu.label }}</span>
            <span v-if="menu.badge" class="nav-badge-p2">P2</span>
          </router-link>
        </template>
      </nav>
      <div class="mt-auto p-4" />
    </aside>

    <main class="app-main flex-1 flex flex-col h-full overflow-hidden relative min-w-0">
      <!-- TopNavBar: Light solid bar -->
      <header class="flex justify-between items-center px-8 py-4 bg-white border-b border-gray-200 z-10">
        <div class="flex min-w-0 items-center gap-3 md:gap-12">
          <button
            type="button"
            class="mobile-menu-button md:hidden"
            aria-label="打开导航"
            @click="mobileSidebarOpen = true"
          >
            <span class="material-symbols-outlined">menu</span>
          </button>
          <span class="min-w-0 truncate text-lg font-headline font-extrabold tracking-tighter text-gray-900 uppercase">永箔运营管理系统</span>
        </div>
        <div class="flex min-w-0 items-center gap-3 sm:gap-6">
          <div class="relative hidden sm:block">
            <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 !text-lg pointer-events-none">search</span>
            <input
              type="text"
              readonly
              placeholder="Ctrl+K 搜索任务、SKU、产品"
              class="bg-gray-50 border border-gray-200 rounded-full pl-10 pr-4 py-1.5 text-xs w-64 text-gray-900 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-all placeholder:text-gray-400"
              @focus="openSearch"
              @click="openSearch"
            />
          </div>
          <div class="flex items-center gap-4">
            <NotificationBadge @open="notificationOpen = true" />
            <AvatarDropdown />
          </div>
        </div>
      </header>

      <div class="flex-1 overflow-auto custom-scrollbar isolate">
        <main class="content">
          <router-view v-slot="{ Component, route }">
            <component v-if="Component" :is="Component" :key="route.path" />
          </router-view>
        </main>
      </div>
    </main>
    <GlobalSearchOverlay v-model:open="searchOpen" />
    <NotificationCenter v-model:open="notificationOpen" />
    <BaseLoadingOverlay
      :active="routeLoading"
      fixed
      teleport
      label="页面加载中"
      description="正在切换业务页面"
    />
    <Teleport to="body">
      <transition name="mobile-sidebar">
        <div
          v-if="mobileSidebarOpen"
          class="mobile-sidebar-backdrop md:hidden"
          @click.self="closeMobileSidebar"
        >
          <aside class="mobile-sidebar-panel" aria-label="移动端导航">
            <div class="mobile-sidebar-header">
              <div class="mobile-brand">
                <span class="material-symbols-outlined">architecture</span>
                <strong>永箔运营</strong>
              </div>
              <button type="button" class="mobile-close-button" aria-label="关闭导航" @click="closeMobileSidebar">
                <span class="material-symbols-outlined">close</span>
              </button>
            </div>
            <nav class="mobile-sidebar-nav custom-scrollbar">
              <template v-for="section in menuSections" :key="section.key">
                <div class="mobile-nav-section-label">{{ section.label }}</div>
                <router-link
                  v-for="menu in section.menus"
                  :key="menu.key"
                  :to="resolveMenuTo(menu)"
                  :active-class="menu.exact ? '' : 'active'"
                  :exact-active-class="menu.exact ? 'active' : ''"
                  class="mobile-nav-item"
                  @click="closeMobileSidebar"
                >
                  <span class="material-symbols-outlined">{{ menu.icon }}</span>
                  <span>{{ menu.label }}</span>
                  <span v-if="menu.badge && menu.badge() > 0" class="mobile-nav-badge">{{ menu.badge() }}</span>
                </router-link>
              </template>
            </nav>
          </aside>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePermissionsStore } from '@/stores/permissions'
import GlobalSearchOverlay from '@/components/global-search/GlobalSearchOverlay.vue'
import NotificationBadge from '@/components/notification/NotificationBadge.vue'
import NotificationCenter from '@/components/notification/NotificationCenter.vue'
import AvatarDropdown from '@/components/layout/AvatarDropdown.vue'
import BaseLoadingOverlay from '@/components/base/BaseLoadingOverlay.vue'

const permissionsStore = usePermissionsStore()
const router = useRouter()

const searchOpen = ref(false)
const notificationOpen = ref(false)
const mobileSidebarOpen = ref(false)
const routeLoading = ref(false)
let routeLoadingTimer: ReturnType<typeof setTimeout> | null = null
let removeRouteBeforeGuard: (() => void) | null = null
let removeRouteAfterHook: (() => void) | null = null
let removeRouteErrorHook: (() => void) | null = null

interface MenuConfig {
  key: string
  label: string
  to: string
  exact?: boolean
  /** 老后端/老前端键兼容：任一命中即视为该菜单的 canonical key */
  aliases?: string[]
  section: 'workbench' | 'business' | 'data'
  icon: string
  badge?: () => number
}

// 侧边栏完全以后端 `frontend_access.menus` 为 SoT；
// 这里的 `MENU_CONFIG` 只保留显示元数据（label/icon/to/section）与兼容别名，
// 不再做 page/module 二次门禁。
const MENU_CONFIG: MenuConfig[] = [
  // v4.2 修复：老板要求 + Dashboard 使用精确高亮，避免 '/' 前缀匹配导致所有页面都高亮
  {
    key: 'dashboard',
    label: '主页',
    to: '/',
    exact: true,
    section: 'workbench',
    icon: 'grid_view',
  },
  {
    key: 'task_list',
    label: '任务中心',
    to: '/tasks',
    aliases: ['my_tasks', 'task_center'],
    section: 'workbench',
    icon: 'reorder',
  },
  {
    key: 'resource_management',
    label: '资产管理',
    to: '/asset-center',
    aliases: ['assets_index'],
    section: 'data',
    icon: 'perm_media',
  },
  {
    key: 'org_admin',
    label: '组织',
    to: '/org',
    aliases: ['org_permission'],
    section: 'data',
    icon: 'groups',
  },
  {
    key: 'report_center',
    label: '数据中心',
    to: '/data-center',
    aliases: ['kpi', 'finance', 'export_center', 'export_jobs', 'logs_center', 'logs_manage'],
    section: 'data',
    icon: 'analytics',
  },
  {
    key: 'audit_log',
    label: '审计日志',
    to: '/audit-log',
    section: 'data',
    icon: 'history',
  },
  {
    key: 'finance',
    label: '财务核算',
    to: '/finance',
    section: 'data',
    icon: 'account_balance',
    badge: () => 0,
  },
  {
    key: 'rules',
    label: '规则及模板',
    to: '/rules',
    section: 'data',
    icon: 'settings_input_component',
  },
  {
    key: 'user_admin',
    label: '用户与角色',
    to: '/users',
    aliases: ['user_manage'],
    section: 'data',
    icon: 'person',
  },
]

// v1.8 Round I：菜单可见性完全由后端 `frontend_access.menus` 下发决定，
// 不再因 SuperAdmin / HRAdmin 这两个角色名进行本地兜底。SuperAdmin 的菜单
// 由后端 menu policy 显式赋予 MENU_CONFIG 的全部 key；若将来新增 key，后端也
// 必须同步下发，前端不再硬编码放行。
const visibleMenus = computed(() => {
  if (!permissionsStore.currentUser) return []
  const userMenus = permissionsStore.menus
  const alwaysVisibleKeys = new Set(['task_list'])
  return MENU_CONFIG.filter((menu) =>
    alwaysVisibleKeys.has(menu.key) ||
    (menu.key === 'report_center' && permissionsStore.currentUser?.role === 'super_admin') ||
    userMenus.includes(menu.key) ||
    (menu.aliases ?? []).some((alias) => userMenus.includes(alias)),
  )
})

const workbenchMenus = computed(() => visibleMenus.value.filter((m) => m.section === 'workbench'))
const businessMenus = computed(() => visibleMenus.value.filter((m) => m.section === 'business'))
const dataMenus = computed(() => visibleMenus.value.filter((m) => m.section === 'data'))
const menuSections = computed(() =>
  [
    { key: 'workbench', label: '工作台', menus: workbenchMenus.value },
    { key: 'business', label: '业务处理', menus: businessMenus.value },
    { key: 'data', label: '数据与配置', menus: dataMenus.value },
  ].filter((section) => section.menus.length > 0),
)

const CUSTOMIZATION_REVIEWER_ROLES = [
  'CustomizationReviewer',
  'customization_reviewer',
  'customizationreviewer',
] as const
const NORMAL_AUDIT_ROLES = ['Audit_A', 'Audit_B', 'audit_a', 'audit_b', 'auditor'] as const

function resolveMenuTo(menu: MenuConfig) {
  if (menu.key !== 'task_list') return menu.to

  const canReviewCustomization = permissionsStore.hasAnyRole(CUSTOMIZATION_REVIEWER_ROLES)
  const canReviewNormal = permissionsStore.hasAnyRole(NORMAL_AUDIT_ROLES)
  if (canReviewCustomization && !canReviewNormal) {
    return {
      path: '/tasks',
      query: {
        task_category: 'customization',
        status: 'PendingCustomizationReview,PendingEffectReview',
      },
    }
  }
  if (canReviewNormal && !canReviewCustomization) {
    return {
      path: '/tasks',
      query: {
        task_category: 'normal',
        status: 'PendingAuditA,PendingAuditB',
      },
    }
  }
  if (canReviewCustomization && canReviewNormal) {
    return {
      path: '/tasks',
      query: {
        status: 'PendingAuditA,PendingAuditB,PendingCustomizationReview,PendingEffectReview',
      },
    }
  }
  return menu.to
}

function openSearch() {
  searchOpen.value = true
}

function closeMobileSidebar() {
  mobileSidebarOpen.value = false
}

function startRouteLoading() {
  if (routeLoadingTimer) clearTimeout(routeLoadingTimer)
  routeLoadingTimer = setTimeout(() => {
    routeLoading.value = true
  }, 140)
}

function stopRouteLoading() {
  if (routeLoadingTimer) {
    clearTimeout(routeLoadingTimer)
    routeLoadingTimer = null
  }
  routeLoading.value = false
  closeMobileSidebar()
}

function onGlobalKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    searchOpen.value = true
  }
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown)
  removeRouteBeforeGuard = router.beforeEach((_to, _from, next) => {
    startRouteLoading()
    next()
  })
  removeRouteAfterHook = router.afterEach(() => {
    stopRouteLoading()
  })
  removeRouteErrorHook = router.onError(() => {
    stopRouteLoading()
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
  stopRouteLoading()
  removeRouteBeforeGuard?.()
  removeRouteAfterHook?.()
  removeRouteErrorHook?.()
})

function onSidebarTransitionEnd(e: TransitionEvent) {
  if (e.propertyName === 'width') {
    window.dispatchEvent(new CustomEvent('layout-change'))
  }
}
</script>

<style scoped>
.nav-section-label {
  @apply px-4 pt-3 pb-1 text-[10px] font-headline font-bold uppercase tracking-[0.2em] text-stone-500;
}

.nav-item.active {
  @apply bg-blue-50 text-blue-700 border border-blue-100;
}

.nav-badge-p2 {
  @apply inline-flex items-center ml-1 px-1.5 text-[10px] font-semibold rounded-full border border-gray-200 bg-gray-100 text-gray-600;
}

.sidebar-item-hover:hover .material-symbols-outlined {
  transform: scale(1.1) rotate(-5deg);
  transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.role-tag {
  @apply inline-flex items-center rounded-full border border-stone-200 bg-stone-50 px-2.5 py-0.5 text-xs font-medium text-stone-600;
}

.user-trigger {
  @apply flex items-center gap-2 cursor-pointer text-sm text-stone-900 px-2 py-1 rounded-lg border border-transparent bg-transparent transition-colors;
}

.user-trigger:hover {
  @apply bg-stone-50 border-stone-200;
}

.switcher-caret {
  @apply text-stone-400 transition-transform inline-block;
}

.switcher-caret.open {
  @apply rotate-180;
}

.role-dropdown {
  @apply absolute right-0 top-[calc(100%+6px)] min-w-[180px] rounded-2xl border border-gray-200 bg-white shadow-md z-50 py-1.5;
}

.dropdown-item {
  @apply flex items-center justify-between w-full px-3 py-1.5 text-xs text-stone-600 bg-transparent border-none text-left cursor-pointer gap-2;
}

.dropdown-item:hover {
  @apply bg-stone-50 text-stone-900;
}

.logout-item {
  @apply text-red-600;
}

.logout-item:hover {
  @apply bg-red-50 text-red-700;
}

/* 铺满滚动区宽度（不再居中 max-width）；保留少量边距避免贴边 */
.content {
  @apply flex-1 w-full min-w-0 px-4 py-4;
}

/* Light admin shell skin. Style-only: keep drawer behavior and DOM unchanged. */
.app-shell {
  --sidebar-rail: 4.25rem;
  --sidebar-drawer: 16rem;
  background: #f5f6f8 !important;
  color: #111827;
}

.sidebar {
  position: relative;
  z-index: 40;
  width: var(--sidebar-rail) !important;
  flex: 0 0 var(--sidebar-rail) !important;
  margin: 0.5rem 0 0.5rem 0.5rem;
  height: calc(100% - 1rem) !important;
  overflow: hidden !important;
  border: 0 !important;
  border-radius: 0.95rem;
  background: transparent !important;
  box-shadow: none !important;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
  background-clip: padding-box;
  clip-path: inset(0 round 0.95rem);
  contain: paint;
  transition:
    width 0.3s cubic-bezier(0.2, 0.8, 0.2, 1),
    flex-basis 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.sidebar:hover {
  width: var(--sidebar-drawer) !important;
  flex-basis: var(--sidebar-drawer) !important;
}

.app-main {
  min-width: 0;
}

.sidebar::before {
  content: '';
  position: absolute;
  inset: 0;
  width: 100%;
  pointer-events: none;
  border-radius: inherit;
  border: 1px solid #e5e7eb;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
  background-clip: padding-box;
  z-index: 0;
}

.sidebar::after {
  display: none;
}

.sidebar > div:first-child {
  position: relative;
  z-index: 1;
  width: calc(var(--sidebar-rail) - 0.8rem);
  margin: 0.35rem 0.4rem 0.65rem;
  height: 3rem !important;
  padding-inline: 0.45rem !important;
  border-radius: 0.85rem;
  background: transparent !important;
  transition: width 0.24s ease;
}

.sidebar:hover > div:first-child {
  width: calc(var(--sidebar-drawer) - 0.8rem);
}

.sidebar > nav {
  position: relative;
  z-index: 1;
  width: 100%;
  box-sizing: border-box;
  overflow-x: hidden;
  padding-inline: 0.75rem !important;
  transition:
    width 0.24s ease,
    padding-inline 0.24s ease;
}

.sidebar:hover > nav {
  padding-inline: 1rem !important;
}

.sidebar > div:first-child > div:first-child {
  width: 2rem !important;
  height: 2rem !important;
  border-radius: 0.75rem !important;
  background: #eff6ff !important;
  box-shadow: none;
}

.sidebar h2 {
  color: #111827 !important;
  font-size: 0.82rem !important;
  font-weight: 750 !important;
}

.sidebar .opacity-0 {
  opacity: 0 !important;
}

.sidebar:hover .opacity-0 {
  opacity: 1 !important;
}

.sidebar :deep(.material-symbols-outlined) {
  color: #6b7280;
}

.nav-section-label {
  color: #9ca3af;
}

.nav-item {
  color: #6b7280 !important;
  overflow: hidden;
  border-radius: 0.85rem !important;
  background-clip: padding-box;
  border: 1px solid transparent;
}

.nav-item:hover {
  background: #f3f4f6 !important;
  color: #111827 !important;
}

.nav-item.active {
  border: 1px solid #bfdbfe !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
  text-shadow: none;
  box-shadow: none;
}

.nav-item.active :deep(.material-symbols-outlined) {
  color: #2563eb;
}

.nav-badge-p2 {
  border-color: #e5e7eb;
  background: #f3f4f6;
  color: #6b7280;
}

main > header {
  position: relative;
  z-index: 120 !important;
  margin: 0.75rem 0.75rem 0;
  min-height: 3.85rem;
  padding-block: 0.65rem !important;
  border: 1px solid #e5e7eb !important;
  border-radius: 1rem;
  background: #ffffff !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06);
  overflow: visible;
  background-clip: padding-box;
  clip-path: none;
  contain: none;
}

main > header span {
  color: #111827 !important;
}

main > header > div:first-child > span {
  color: #111827 !important;
  font-size: 1rem !important;
  font-weight: 800 !important;
  letter-spacing: 0 !important;
  text-shadow: none !important;
}

main > header input {
  background: #f9fafb !important;
  border-color: #e5e7eb !important;
  color: #111827 !important;
}

main > header input::placeholder {
  color: #9ca3af !important;
}

.content {
  padding: 0.75rem;
  background: #f5f6f8;
  max-width: 100%;
  overflow-x: hidden;
}

.mobile-menu-button,
.mobile-close-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.35rem;
  height: 2.35rem;
  flex: 0 0 auto;
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  background: #ffffff;
  color: #374151;
}

.mobile-menu-button:hover,
.mobile-close-button:hover {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
}

.mobile-sidebar-backdrop {
  position: fixed;
  inset: 0;
  z-index: 7500;
  display: flex;
  background: rgba(15, 23, 42, 0.2);
  padding: 0.75rem;
}

.mobile-sidebar-panel {
  display: flex;
  width: min(20rem, calc(100vw - 1.5rem));
  max-width: 100%;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #e5e7eb;
  border-radius: 1rem;
  background: #ffffff;
  box-shadow: 0 24px 48px -28px rgba(15, 23, 42, 0.55);
}

.mobile-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-bottom: 1px solid #e5e7eb;
  padding: 0.85rem;
}

.mobile-brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.55rem;
  color: #111827;
}

.mobile-brand .material-symbols-outlined {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  background: #eff6ff;
  color: #2563eb !important;
}

.mobile-brand strong {
  min-width: 0;
  overflow: hidden;
  color: #111827;
  font-size: 0.95rem;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-sidebar-nav {
  display: grid;
  min-height: 0;
  gap: 0.35rem;
  overflow-y: auto;
  padding: 0.8rem;
}

.mobile-nav-section-label {
  padding: 0.7rem 0.35rem 0.2rem;
  color: #9ca3af;
  font-size: 0.68rem;
  font-weight: 800;
}

.mobile-nav-item {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  border: 1px solid transparent;
  border-radius: 0.8rem;
  padding: 0.55rem 0.65rem;
  color: #4b5563;
}

.mobile-nav-item:hover {
  background: #f3f4f6;
  color: #111827;
  text-decoration: none;
}

.mobile-nav-item.active {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
}

.mobile-nav-item .material-symbols-outlined {
  color: currentColor !important;
}

.mobile-nav-item span:nth-child(2) {
  min-width: 0;
  overflow: hidden;
  font-size: 0.9rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-nav-badge {
  border-radius: 999px;
  background: #f3f4f6;
  color: #6b7280;
  padding: 0.05rem 0.45rem;
  font-size: 0.68rem;
  font-weight: 800;
}

.mobile-sidebar-enter-active,
.mobile-sidebar-leave-active {
  transition: opacity 0.16s ease;
}

.mobile-sidebar-enter-active .mobile-sidebar-panel,
.mobile-sidebar-leave-active .mobile-sidebar-panel {
  transition: transform 0.18s ease;
}

.mobile-sidebar-enter-from,
.mobile-sidebar-leave-to {
  opacity: 0;
}

.mobile-sidebar-enter-from .mobile-sidebar-panel,
.mobile-sidebar-leave-to .mobile-sidebar-panel {
  transform: translateX(-0.75rem);
}

@media (max-width: 767px) {
  .app-shell {
    display: block;
  }

  .app-main {
    width: 100%;
    height: 100%;
  }

  main > header {
    margin: 0.5rem 0.5rem 0;
    min-height: 3.25rem;
    padding: 0.5rem 0.65rem !important;
    border-radius: 0.85rem;
  }

  main > header > div:first-child > span {
    font-size: 0.92rem !important;
  }

  .content {
    padding: 0.55rem;
  }
}

@media (max-width: 420px) {
  main > header {
    gap: 0.4rem;
  }

  .mobile-menu-button {
    width: 2.2rem;
    height: 2.2rem;
  }
}
</style>
