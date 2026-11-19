<template>
  <div :class="{ 'oa-client-shell': oaEnabled }" class="app-shell font-body flex overflow-hidden h-[100dvh]">
    <!-- Light Slim Sidebar -->
    <aside
      :class="{ 'oa-sidebar': oaEnabled, 'oa-sidebar-open': sidebarPinned }"
      @pointerleave="tip = null"
      class="sidebar hidden md:flex flex-col h-full w-20 bg-[rgb(var(--yb-surface))] border-r border-[rgb(var(--yb-border))] transition-all duration-300 overflow-hidden group hover:w-64"
      @transitionend="onSidebarTransitionEnd"
    >
      <div class="flex items-center gap-3 px-6 h-16 mb-4">
        <div class="w-8 h-8 rounded-lg bg-[rgb(var(--yb-brand-soft))] border border-[rgb(var(--yb-brand-border))] flex items-center justify-center flex-shrink-0">
          <img v-if="oaEnabled" class="oa-brand-mark" :src="oaBrandMark" alt="永箔" width="32" height="32" />
          <span v-else class="material-symbols-outlined text-[rgb(var(--yb-brand))] text-lg">architecture</span>
        </div>
        <div class="opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">
          <h2 class="font-headline font-extrabold text-[rgb(var(--yb-text))] text-sm tracking-tight">永箔运营</h2>
        </div>
      </div>
      <nav aria-label="主导航" class="flex-1 px-4 space-y-4 overflow-y-auto custom-scrollbar">
        <template v-if="workbenchMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">工作台</div>
          <div v-for="menu in workbenchMenus" :key="menu.key" class="nav-entry">
          <router-link
            :aria-label="menuLabel(menu)"
            @pointerenter="showTip(menu, $event)"
            @focus="showTip(menu, $event)"
            @blur="tip = null"
            @pointerleave="tip = null"
            :to="resolveMenuTo(menu)"
            :class="{ active: oaEnabled && menu.key === 'oa_operations' && $route.path.startsWith('/operations/') }"
            :active-class="menu.exact ? '' : 'active'"
            :exact-active-class="menu.exact ? 'active' : ''"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-[rgb(var(--yb-text-muted))] hover:text-[rgb(var(--yb-text))] hover:bg-[rgb(var(--yb-surface-muted))] sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menuLabel(menu) }}</span>
            <span v-if="oaEnabled && !sidebarPinned" class="oa-rail-caption" aria-hidden="true">{{ shortLabel[menu.key] || menu.label }}</span>
            <span v-if="menu.badge && menu.badge() > 0" class="nav-badge-p2">{{ menu.badge() }}</span>
          </router-link>
          <RouterLink v-if="oaEnabled && navSignal(menu)" class="oa-nav-signal" :to="navSignal(menu)!.to" @click="repeatShortcut(menu)" :aria-label="navSignal(menu)!.label" :title="navSignal(menu)!.label">{{ navSignal(menu)!.count }}</RouterLink>
          </div>
        </template>
        <template v-if="businessMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">业务处理</div>
          <div v-for="menu in businessMenus" :key="menu.key" class="nav-entry">
          <router-link
            :aria-label="menuLabel(menu)"
            @pointerenter="showTip(menu, $event)"
            @focus="showTip(menu, $event)"
            @blur="tip = null"
            @pointerleave="tip = null"
            :to="menu.to"
            :class="{ active: menu.key === 'oa_expenses' && $route.path.startsWith('/fees/') }"
            active-class="active"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-[rgb(var(--yb-text-muted))] hover:text-[rgb(var(--yb-text))] hover:bg-[rgb(var(--yb-surface-muted))] sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menuLabel(menu) }}</span>
            <span v-if="oaEnabled && !sidebarPinned" class="oa-rail-caption" aria-hidden="true">{{ shortLabel[menu.key] || menu.label }}</span>
          </router-link>
          <RouterLink v-if="oaEnabled && navSignal(menu)" class="oa-nav-signal" :to="navSignal(menu)!.to" @click="repeatShortcut(menu)" :aria-label="navSignal(menu)!.label" :title="navSignal(menu)!.label">{{ navSignal(menu)!.count }}</RouterLink>
          </div>
        </template>
        <template v-if="dataMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">数据与配置</div>
          <div v-for="menu in dataMenus" :key="menu.key" class="nav-entry">
          <router-link
            :aria-label="menuLabel(menu)"
            @pointerenter="showTip(menu, $event)"
            @focus="showTip(menu, $event)"
            @blur="tip = null"
            @pointerleave="tip = null"
            :to="menu.to"
            :class="{ active: isPersonnelSubpage(menu) }"
            active-class="active"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-[rgb(var(--yb-text-muted))] hover:text-[rgb(var(--yb-text))] hover:bg-[rgb(var(--yb-surface-muted))] sidebar-item-hover transition-all"
          >
            <div class="w-8 h-8 flex items-center justify-center flex-shrink-0">
              <span class="material-symbols-outlined">{{ menu.icon }}</span>
            </div>
            <span class="text-sm font-medium opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">{{ menuLabel(menu) }}</span>
            <span v-if="oaEnabled && !sidebarPinned" class="oa-rail-caption" aria-hidden="true">{{ shortLabel[menu.key] || menu.label }}</span>
            <span v-if="menu.badge" class="nav-badge-p2">P2</span>
          </router-link>
          <RouterLink v-if="oaEnabled && navSignal(menu)" class="oa-nav-signal" :to="navSignal(menu)!.to" @click="repeatShortcut(menu)" :aria-label="navSignal(menu)!.label" :title="navSignal(menu)!.label">{{ navSignal(menu)!.count }}</RouterLink>
          </div>
        </template>
      </nav>
      <button v-if="oaEnabled" type="button" class="oa-sidebar-pin" :aria-pressed="sidebarPinned" :aria-label="sidebarPinned ? '收起导航' : '展开导航'" :title="sidebarPinned ? '收起导航' : '展开导航'" @click="toggleSidebar">
        <PanelLeftClose v-if="sidebarPinned" :size="18"/><PanelLeftOpen v-else :size="18"/><span v-if="sidebarPinned">收起导航</span>
      </button>
      <div v-else class="mt-auto p-4" />
    </aside>

    <Teleport to="body"><span v-if="tip" class="oa-sidebar-tip" role="tooltip" :style="{ top: tip.top + 'px', left: tip.left + 'px' }">{{ tip.label }}</span></Teleport>
    <main class="app-main flex-1 flex flex-col h-full overflow-hidden relative min-w-0">
      <header class="app-topbar">
        <div class="app-topbar__brand">
          <button
            type="button"
            class="mobile-menu-button md:hidden"
            aria-label="打开导航"
            @click="mobileSidebarOpen = true"
          >
            <Menu :size="18" aria-hidden="true" />
          </button>
          <div v-if="oaEnabled" ref="desktopBack" class="oa-bar-return" />
          <div v-if="oaEnabled" ref="desktopTitle" class="oa-context-title" />
          <h1 v-if="oaEnabled && desktop" class="oa-bar-title">{{ $route.name === 'TaskDetail' ? '任务中心 / 任务详情' : moduleName || pageTitle }}</h1>
          <span v-else-if="oaEnabled" class="oa-breadcrumb"><span v-if="moduleName && moduleName !== pageTitle">{{ moduleName }} / </span>{{ pageTitle }}</span>
          <span v-else class="app-topbar__title" title="永箔运营管理系统">
            <span class="app-topbar__full-name">永箔运营管理系统</span>
            <span class="app-topbar__short-name">永箔运营</span>
          </span>

        </div>
        <div v-if="oaEnabled" ref="desktopTabs" class="oa-bar-tabs" />
        <div class="app-topbar__actions">
          <div v-if="oaEnabled" ref="desktopActions" class="oa-bar-page-actions" />
          <div v-if="oaEnabled" ref="desktopStatus" class="oa-bar-status" />
          <button type="button" class="topbar-icon topbar-search" aria-label="全局搜索" aria-keyshortcuts="Control+K Meta+K" title="全局搜索" @click="openSearch">
            <Search :size="18" aria-hidden="true" />
            <span class="topbar-search__label">{{ oaEnabled ? '查找页面、任务、SKU' : '搜索任务、SKU、产品' }}</span><kbd v-if="oaEnabled" class="oa-search-key">Ctrl K</kbd>
          </button>
          <MediaPreparationPanel />
          <NotificationBadge @open="notificationOpen = true" />
          <AvatarDropdown />
        </div>
      </header>

      <div v-if="oaEnabled" ref="compactBack" class="oa-bar-return oa-compact-return" />
      <div ref="contentScroller" :class="{ 'oa-dashboard-viewport': oaEnabled && ($route.path === '/operations' || $route.path.startsWith('/operations/') || $route.path === '/me/performance' || $route.name === 'Dashboard') }" class="app-page-scroll flex-1 overflow-auto custom-scrollbar isolate" :data-workspace="desktopWorkspace">
        <main class="content">
          <PersonalNav v-if="oaEnabled && ($route.path === '/me' || $route.path.startsWith('/me/')) && $route.path !== '/me/performance'" />
          <router-view v-slot="{ Component, route }">
            <KeepAlive :max="8">
              <CachedWorkspace v-if="oaEnabled && Component && route.meta.keepAlive" :key="String(route.name || route.path)">
                <component :is="Component" />
              </CachedWorkspace>
              <component v-else-if="Component && route.meta.keepAlive" :is="Component" :key="String(route.name || route.path)" />
            </KeepAlive>
            <component v-if="Component && !route.meta.keepAlive" :is="Component" :key="route.path" />
          </router-view>
        </main>
      </div>
    </main>
    <GlobalSearchOverlay v-model:open="searchOpen" :pages="oaEnabled ? quickPages : undefined" />
    <NotificationCenter v-if="!oaEnabled" v-model:open="notificationOpen" />
    <transition name="route-progress">
      <div v-if="routeLoading" class="route-progress" role="status" aria-live="polite">
        <span class="route-progress__bar" />
        <span class="sr-only">正在切换业务页面，当前页面仍可使用。</span>
      </div>
    </transition>
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
                <img v-if="oaEnabled" class="oa-brand-mark" :src="oaBrandMark" alt="" width="32" height="32" />
                <span v-else class="material-symbols-outlined">architecture</span>
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
                  :class="{ active: isPersonnelSubpage(menu) }"
                  :active-class="menu.exact ? '' : 'active'"
                  :exact-active-class="menu.exact ? 'active' : ''"
                  class="mobile-nav-item"
                  @click="closeMobileSidebar"
                >
                  <span class="material-symbols-outlined">{{ menu.icon }}</span>
                  <span>{{ menuLabel(menu) }}</span>
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
import { ref, computed, provide, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Menu, Search, PanelLeftOpen, PanelLeftClose } from 'lucide-vue-next'
import { usePermissionsStore } from '@/stores/permissions'
import { oaEnabled, useOAAccess } from '@/oa/access'
import GlobalSearchOverlay from '@/components/global-search/GlobalSearchOverlay.vue'
import NotificationBadge from '@/components/notification/NotificationBadge.vue'
import MediaPreparationPanel from '@/components/media/MediaPreparationPanel.vue'
import NotificationCenter from '@/components/notification/NotificationCenter.vue'
import PersonalNav from '@/oa/components/PersonalNav.vue'
import CachedWorkspace from '@/oa/components/CachedWorkspace.vue'
import { desktopChromeKey } from '@/oa/desktopChrome'
import { useExpenseActivity } from '@/oa/expenseActivity'
import { useSidebarCounts } from '@/oa/sidebarCounts'
import { oaPageTitle } from '@/oa/pageTitle'
import oaBrandMark from '@/oa/assets/yongbo-mark.svg'
import AvatarDropdown from '@/components/layout/AvatarDropdown.vue'

const permissionsStore = usePermissionsStore()
const { can } = useOAAccess()
const router = useRouter()

const deskQuery = window.matchMedia?.('(min-width: 1280px)')
const wideSidebar = window.matchMedia?.('(min-width: 1440px)')
const desktop = ref(oaEnabled && !!deskQuery?.matches)
const large = ref(!!wideSidebar?.matches)
const desktopBack = ref<HTMLElement | null>(null)
const compactBack = ref<HTMLElement | null>(null)
const backHost = computed(() => desktop.value ? desktopBack.value : compactBack.value)
const desktopTitle = ref<HTMLElement | null>(null)
const desktopTabs = ref<HTMLElement | null>(null)
const desktopStatus = ref<HTMLElement | null>(null)
const desktopActions = ref<HTMLElement | null>(null)
if (oaEnabled) provide(desktopChromeKey, { desktop, back: backHost, title: desktopTitle, tabs: desktopTabs, status: desktopStatus, actions: desktopActions })
function syncDesktop() {
  desktop.value = oaEnabled && !!deskQuery?.matches
  large.value = !!wideSidebar?.matches
}
const sidebarPreference = ref(oaEnabled ? localStorage.getItem('oa.navigation.expanded') : null)
const sidebarPinned = computed(() => oaEnabled && (sidebarPreference.value === null
  ? (!desktop.value || large.value)
  : sidebarPreference.value === '1'))
const desktopWorkspace = computed(() => {
  const path = router.currentRoute.value.path
  if (['/operations','/','/design-dashboard'].includes(path)) return 'overview'
  if (['/tasks','/operations/products','/operations/pricing','/operations/rankings','/users','/cost-rules','/fees/approval','/fees/mine'].includes(path) || path.startsWith('/operations/orders/')) return 'list'
  return 'page'
})
const expenseActivity = useExpenseActivity()
const workCounts = useSidebarCounts(computed(() => ({
  account: desktop.value ? permissionsStore.currentUser?.id : undefined,
  tasks: desktop.value && permissionsStore.hasMenu('task_list') && permissionsStore.hasAction('task.view'),
  costs: desktop.value && permissionsStore.hasMenu('cost_rules') && permissionsStore.hasAction('catalog.view'),
})))
const tip = ref<{ label: string; top: number; left: number } | null>(null)
function toggleSidebar() {
  sidebarPreference.value = sidebarPinned.value ? '0' : '1'
  tip.value = null
  localStorage.setItem('oa.navigation.expanded', sidebarPinned.value ? '1' : '0')
}
const shortLabel: Record<string, string> = {
  oa_operations: '经营', dashboard: '看板', task_list: '任务', oa_expenses: '费用',
  resource_management: '资产', cost_rules: '成本', user_admin: '人员',
}
function navSignal(menu: MenuConfig) {
  if (menu.key === 'oa_expenses' && expenseActivity.todoCount) return {
    count: expenseActivity.todoCount, label: expenseActivity.todoCount + ' 项费用待办', to: '/me/notifications?section=expense',
  }
  if (menu.key === 'task_list' && workCounts.audits.value) return {
    count: workCounts.audits.value, label: workCounts.audits.value + ' 项待审任务', to: '/tasks?operational_bucket=pending_audit',
  }
  if (menu.key === 'cost_rules' && workCounts.conflicts.value) return {
    count: workCounts.conflicts.value, label: '待确认成本冲突 ' + workCounts.conflicts.value, to: '/cost-rules?focus=cost-conflicts',
  }
  return null
}
function repeatShortcut(menu: MenuConfig) {
  if (menu.key === 'cost_rules' && router.currentRoute.value.query.focus === 'cost-conflicts') {
    window.dispatchEvent(new Event('oa-refocus-cost-conflicts'))
  }
}
function menuLabel(menu: MenuConfig) {
  return menu.key === 'dashboard' && can('orders.view') ? '任务看板' : menu.label
}
function showTip(menu: MenuConfig, event: Event) {
  if (!oaEnabled || sidebarPinned.value) return
  const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
  tip.value = { label: menuLabel(menu), top: box.top + box.height / 2, left: box.right + 12 }
}
const pageTitle = computed(() => oaPageTitle(router.currentRoute.value.path))
const moduleName = computed(() => {
  const path = router.currentRoute.value.path
  if (path.startsWith('/operations') || path === '/me/performance') return '经营中心'
  if (path.startsWith('/fees')) return '费用中心'
  if (path === '/cost-rules') return '成本规则'
  if (path === '/') return '任务看板'
  if (path.startsWith('/me')) return '个人中心'
  if (path.startsWith('/tasks/')) return '任务中心'
  return ''
})
watch(pageTitle, title => {
  tip.value = null
  if (oaEnabled) document.title = title + ' · 永箔运营'
}, { immediate: true })

const searchOpen = ref(false)
const notificationOpen = ref(false)
const mobileSidebarOpen = ref(false)
const routeLoading = ref(false)
const contentScroller = ref<HTMLElement | null>(null)
const routeScrollPositions = new Map<string, number>()
let routeLoadingTimer: ReturnType<typeof setTimeout> | null = null
let removeRouteBeforeGuard: (() => void) | null = null
let removeRouteAfterHook: (() => void) | null = null
let removeRouteErrorHook: (() => void) | null = null

interface MenuConfig {
  key: string
  label: string
  to: string
  exact?: boolean
  section: 'workbench' | 'business' | 'data'
  icon: string
  badge?: () => number
  oaActions?: string[]
}

// 侧边栏完全以后端 `frontend_access.menus` 为 SoT；
// 原菜单使用正式 frontend_access.menus；OA 增补菜单使用同会话返回的业务动作。
const MENU_CONFIG: MenuConfig[] = [
  { key: 'oa_operations', label: '经营总览', to: '/operations', exact: true, section: 'workbench', icon: 'monitoring', oaActions: ['orders.view'] },
  { key: 'oa_expenses', label: '费用中心', to: '/fees', section: 'business', icon: 'receipt_long', oaActions: ['finance.apply', 'finance.approve', 'finance.pay', 'finance.templates', 'finance.admin', 'finance.report'] },
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
    section: 'workbench',
    icon: 'reorder',
  },
  {
    key: 'resource_management',
    label: '资产管理',
    to: '/asset-center',
    section: 'data',
    icon: 'perm_media',
  },
  {
    key: 'cost_rules',
    label: '成本规则',
    to: '/cost-rules',
    section: 'data',
    icon: 'calculate',
  },
  {
    key: 'report_center',
    label: '数据中心',
    to: '/data-center',
    section: 'data',
    icon: 'analytics',
  },
  {
    key: 'user_admin',
    label: oaEnabled ? '人员管理' : '用户与角色',
    to: '/users',
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
  return MENU_CONFIG.filter((menu) => {
    if (oaEnabled && menu.key === 'report_center') return false
    if (menu.key === 'user_admin' && can('people.manage')) return true
    return menu.oaActions ? menu.oaActions.some(can) : userMenus.includes(menu.key)
  })
})

function isPersonnelSubpage(menu: MenuConfig) {
  return menu.key === 'user_admin' && router.currentRoute.value.path === '/users/roster'
}

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

const lastTaskQuery = ref<Record<string, string>>({})
watch(() => permissionsStore.currentUser?.id, () => { lastTaskQuery.value = {} }, { flush: 'sync' })
function resolveMenuTo(menu: MenuConfig) {
  if (oaEnabled && menu.key === 'task_list') return { path: '/tasks', query: lastTaskQuery.value }

  if (menu.key === 'dashboard' && can('orders.view')) return { path: '/', query: { workspace: 'tasks' } }
  return menu.to
}

const quickPages = computed(() => {
  const links = visibleMenus.value.map(menu => ({ label: menuLabel(menu), path: router.resolve(resolveMenuTo(menu)).fullPath }))
  if (can('orders.view')) links.push({ label: '订单明细', path: '/operations/orders/detail' }, { label: '商品表现', path: '/operations/products' }, { label: '经营排行', path: '/operations/rankings' })
  if (can('pricing.view')) links.push({ label: 'SKU 核价', path: '/operations/pricing' })
  if (can('finance.apply')) links.push({ label: '发起费用申请', path: '/fees/apply' })
  if (can('finance.approve') || can('finance.pay')) links.push({ label: '审批与付款', path: '/fees/approval' })
  if (can('people.manage')) links.push({ label: '人员花名册', path: '/users/roster' })
  links.push({ label: '个人中心', path: '/me' })
  return links
})

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
  deskQuery?.addEventListener('change', syncDesktop)
  wideSidebar?.addEventListener('change', syncDesktop)
  window.addEventListener('keydown', onGlobalKeydown)
  removeRouteBeforeGuard = router.beforeEach((_to, from, next) => {
    if (oaEnabled && from.name === 'TaskList' && _to.name !== 'TaskList') {
      const filters: Record<string, string> = {}
      for (const key of ['q', 'tab', 'operational_bucket', 'status', 'owner_department', 'owner_org_team', 'task_category', 'task_type', 'creator_id', 'designer_id', 'priority', 'date_from', 'date_to', 'overdue', 'sort']) {
        const value = from.query[key]
        if (typeof value === 'string') filters[key] = value
      }
      lastTaskQuery.value = filters
    }
    if (contentScroller.value) routeScrollPositions.set(from.fullPath, contentScroller.value.scrollTop)
    startRouteLoading()
    next()
  })
  removeRouteAfterHook = router.afterEach((to) => {
    stopRouteLoading()
    void nextTick(() => {
      if (contentScroller.value) contentScroller.value.scrollTop = routeScrollPositions.get(to.fullPath) ?? 0
    })
  })
  removeRouteErrorHook = router.onError(() => {
    stopRouteLoading()
  })
})

onUnmounted(() => {
  deskQuery?.removeEventListener('change', syncDesktop)
  wideSidebar?.removeEventListener('change', syncDesktop)
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
  @apply px-4 pt-3 pb-1 text-[10px] font-headline font-bold uppercase tracking-[0.2em];
  color: rgb(var(--yb-text-muted));
}

.nav-item.active {
  @apply border;
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.nav-badge-p2 {
  @apply inline-flex items-center ml-1 px-1.5 text-[10px] font-semibold rounded-full border;
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-secondary));
}

.sidebar-item-hover:hover .material-symbols-outlined {
  transform: scale(1.1) rotate(-5deg);
  transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.role-tag {
  @apply inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium;
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-brand));
}

.user-trigger {
  @apply flex items-center gap-2 cursor-pointer text-sm px-2 py-1 rounded-lg border border-transparent bg-transparent transition-colors;
  color: rgb(var(--yb-text));
}

.user-trigger:hover {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.switcher-caret {
  @apply transition-transform inline-block;
  color: rgb(var(--yb-text-faint));
}

.switcher-caret.open {
  @apply rotate-180;
}

.role-dropdown {
  @apply absolute right-0 top-[calc(100%+6px)] min-w-[180px] rounded-2xl border border-[rgb(var(--yb-border))] bg-[rgb(var(--yb-surface))] shadow-md z-50 py-1.5;
}

.dropdown-item {
  @apply flex items-center justify-between w-full px-3 py-1.5 text-xs text-[rgb(var(--yb-brand))] bg-transparent border-none text-left cursor-pointer gap-2;
}

.dropdown-item:hover {
  background: rgb(var(--yb-surface-soft));
  color: rgb(var(--yb-text));
}

.logout-item {
  @apply text-[rgb(var(--yb-danger))];
}

.logout-item:hover {
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}

/* 铺满滚动区宽度（不再居中 max-width）；保留少量边距避免贴边 */
.content {
  @apply flex-1 w-full min-w-0 px-4 py-4;
}

/* Light admin shell skin. Style-only: keep drawer behavior and DOM unchanged. */
.app-shell {
  --sidebar-rail: 4.25rem;
  --sidebar-drawer: 16rem;
  background: rgb(var(--yb-bg-page));
  color: rgb(var(--yb-text));
}

.sidebar {
  position: relative;
  z-index: 40;
  width: var(--sidebar-rail);
  flex: 0 0 var(--sidebar-rail);
  margin: 0.5rem 0 0.5rem 0.5rem;
  height: calc(100% - 1rem);
  overflow: hidden;
  border: 0;
  border-radius: 0.95rem;
  background: transparent;
  box-shadow: none;
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
  width: var(--sidebar-drawer);
  flex-basis: var(--sidebar-drawer);
}

.app-main {
  min-width: 0;
}

.route-progress {
  position: fixed;
  inset: 0 0 auto;
  z-index: 7600;
  height: 3px;
  overflow: hidden;
  pointer-events: none;
  background: rgb(var(--yb-brand-soft));
}

.route-progress__bar {
  display: block;
  width: 42%;
  height: 100%;
  border-radius: 999px;
  background: rgb(var(--yb-brand));
  animation: route-progress-slide 1s ease-in-out infinite;
  will-change: transform;
}

.route-progress-enter-active,
.route-progress-leave-active {
  transition: opacity 0.12s ease;
}

.route-progress-enter-from,
.route-progress-leave-to {
  opacity: 0;
}

@keyframes route-progress-slide {
  from {
    transform: translateX(-110%);
  }
  to {
    transform: translateX(340%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .route-progress__bar {
    width: 100%;
    animation: none;
  }
}

.sidebar::before {
  content: '';
  position: absolute;
  inset: 0;
  width: 100%;
  pointer-events: none;
  border-radius: inherit;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
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
  height: 3rem;
  padding-inline: 0.45rem;
  border-radius: 0.85rem;
  background: transparent;
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
  padding-inline: 0.75rem;
  transition:
    width 0.24s ease,
    padding-inline 0.24s ease;
}

.sidebar:hover > nav {
  padding-inline: 1rem;
}

.sidebar > div:first-child > div:first-child {
  width: 2rem;
  height: 2rem;
  border-radius: 0.75rem;
  background: rgb(var(--yb-brand-soft));
  box-shadow: none;
}

.sidebar h2 {
  color: rgb(var(--yb-text));
  font-size: 0.82rem;
  font-weight: 750;
}

.sidebar .opacity-0 {
  opacity: 0;
}

.sidebar:hover .opacity-0 {
  opacity: 1;
}

.sidebar :deep(.material-symbols-outlined) {
  color: rgb(var(--yb-text-muted));
}

.nav-section-label {
  color: rgb(var(--yb-text-faint));
}

.nav-item {
  color: rgb(var(--yb-text-muted));
  overflow: hidden;
  border-radius: 0.85rem;
  background-clip: padding-box;
  border: 1px solid transparent;
}

.nav-item:hover {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text));
}

.nav-item.active {
  border: 1px solid rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  text-shadow: none;
  box-shadow: none;
}

.nav-item.active :deep(.material-symbols-outlined) {
  color: rgb(var(--yb-brand));
}

.nav-badge-p2 {
  border-color: rgb(var(--yb-border));
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-muted));
}

.app-topbar {
  --topbar-control-size: 2.5rem;
  position: relative;
  z-index: 120;
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0.75rem 0.75rem 0;
  padding: 0.6rem 1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 1px 3px rgb(var(--yb-shadow) / 0.06);
  overflow: visible;
  background-clip: padding-box;
  clip-path: none;
  contain: none;
}

.app-topbar__brand,
.app-topbar__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.app-topbar__brand { min-width: 0; }
.app-topbar__actions { flex: 0 0 auto; }

.app-topbar__title {
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-size: clamp(0.875rem, 0.82rem + 0.2vw, 1rem);
  font-weight: 800;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-topbar :deep(.topbar-icon) {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  inline-size: var(--topbar-control-size);
  block-size: var(--topbar-control-size);
  flex: 0 0 auto;
  padding: 0;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
  cursor: pointer;
}

.app-topbar :deep(.topbar-icon:hover) {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.app-topbar :deep(.topbar-icon:focus-visible),
.app-topbar .mobile-menu-button:focus-visible {
  outline: 2px solid rgb(var(--yb-brand));
  outline-offset: 2px;
}

.topbar-search__label { display: none; }
.app-topbar__short-name { display: none; }

@media (min-width: 1024px) {
  .app-topbar .topbar-search {
    inline-size: auto;
    gap: 0.5rem;
    padding-inline: 0.7rem 0.9rem;
  }

  .topbar-search__label {
    display: inline;
    font-size: 0.8125rem;
    white-space: nowrap;
  }
}

@media (min-width: 768px) {
  .app-topbar .mobile-menu-button { display: none; }
}

.content {
  padding: 0.75rem;
  background: rgb(var(--yb-bg-page));
  max-width: 100%;
  overflow-x: hidden;
}

@media (min-width: 120rem) {
  .content {
    inline-size: min(100%, 120rem);
    margin-inline: auto;
  }
}

.mobile-menu-button,
.mobile-close-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.35rem;
  height: 2.35rem;
  flex: 0 0 auto;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-body));
}

.mobile-menu-button:hover,
.mobile-close-button:hover {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.mobile-sidebar-backdrop {
  position: fixed;
  inset: 0;
  z-index: 7500;
  display: flex;
  background: rgb(var(--yb-shadow) / 0.2);
  padding: 0.75rem;
}

.mobile-sidebar-panel {
  display: flex;
  width: min(20rem, calc(100vw - 1.5rem));
  max-width: 100%;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 24px 48px -28px rgb(var(--yb-shadow) / 0.55);
}

.mobile-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-bottom: 1px solid rgb(var(--yb-border));
  padding: 0.85rem;
}

.mobile-brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.55rem;
  color: rgb(var(--yb-text));
}

.mobile-brand .material-symbols-outlined {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand));
}

.mobile-brand strong {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text));
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
  color: rgb(var(--yb-text-faint));
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
  color: rgb(var(--yb-text-secondary));
}

.mobile-nav-item:hover {
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text));
  text-decoration: none;
}

.mobile-nav-item.active {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
}

.mobile-nav-item .material-symbols-outlined {
  color: currentColor;
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
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-muted));
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

  .app-topbar {
    --topbar-control-size: 2.75rem;
    margin: 0.5rem 0.5rem 0;
    padding: 0.4rem 0.5rem;
    border-radius: 0.85rem;
  }

  .app-topbar .mobile-menu-button {
    inline-size: var(--topbar-control-size);
    block-size: var(--topbar-control-size);
  }

  .content {
    padding: 0.55rem;
  }
}

@media (max-width: 420px) {
  .app-topbar,
  .app-topbar__brand,
  .app-topbar__actions {
    gap: 0.35rem;
  }

  .app-topbar__full-name { display: none; }
  .app-topbar__short-name { display: inline; }
}

.oa-sidebar.sidebar { --sidebar-rail: 76px; --sidebar-drawer: 176px; }
.oa-sidebar.sidebar:not(.oa-sidebar-open):hover { width: var(--sidebar-rail); flex-basis: var(--sidebar-rail); }
.oa-sidebar.sidebar.oa-sidebar-open { width: var(--sidebar-drawer); flex-basis: var(--sidebar-drawer); }
.oa-sidebar.sidebar > nav { min-height: 0; padding-inline: 8px; }
.oa-sidebar.sidebar > nav > * { margin-block: 4px; }
.oa-sidebar.sidebar .nav-item { min-height: 40px; padding: 4px 8px; gap: 8px; border-radius: 6px; }
.oa-sidebar:not(.oa-sidebar-open) .opacity-0 { display: none; }
.oa-sidebar.sidebar:not(.oa-sidebar-open) .nav-section-label { display: block; height: 1px; padding: 0; margin: 8px; overflow: hidden; opacity: 1; font-size: 0; background: rgb(var(--yb-border)); }
.oa-sidebar.sidebar.oa-sidebar-open .opacity-0 { opacity: 1; }
.oa-sidebar.sidebar.oa-sidebar-open .nav-section-label { padding: 4px 8px; font-size: 12px; letter-spacing: 0; white-space: nowrap; }
.oa-sidebar.sidebar > div:first-child { width: auto; }
.oa-sidebar-pin { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 40px; margin: 8px; border-radius: 6px; color: var(--oa-secondary); }
.oa-sidebar-pin:hover { background: rgb(var(--yb-surface-muted)); }
.oa-sidebar-tip { position: fixed; z-index: 130; transform: translateY(-50%); padding: 4px 8px; border: 1px solid var(--oa-line); border-radius: 6px; background: var(--oa-surface); color: var(--oa-ink); pointer-events: none; font-size: 14px; }
.oa-breadcrumb { font-size: 14px; font-weight: 600; white-space: nowrap; text-overflow: ellipsis; overflow: hidden; }
.oa-breadcrumb > span { color: var(--oa-secondary); font-weight: 400; }
.oa-search-key { display:none; }
.oa-search-key { margin-left:auto; color:var(--oa-secondary); font:13px/normal var(--yb-font-mono); }
@media (max-width: 600px) { .oa-breadcrumb > span { display:none; } }

.oa-sidebar:not(.oa-sidebar-open) .nav-item { flex-direction: column; justify-content: center; gap: 0; padding: 4px; }
.oa-sidebar:not(.oa-sidebar-open) .nav-item > div { width: 24px; height: 24px; }
.oa-rail-caption { font: 500 12px/1.45 var(--oa-font); white-space: nowrap; }
.oa-sidebar .nav-item.active .oa-rail-caption { color: var(--oa-accent); }
.oa-sidebar.sidebar .nav-item:focus-visible { outline: 2px solid var(--oa-accent); outline-offset: -2px; }
.oa-sidebar.sidebar h2 { font: 600 16px/1.4 var(--oa-font); letter-spacing: 0; }
@media (min-width: 1024px) {
  .oa-dashboard-viewport { min-height: 0; overflow: hidden; }
  .oa-dashboard-viewport > .content { height: 100%; min-height: 0; overflow: hidden; box-sizing: border-box; }
}

.oa-brand-mark { flex: none; display: block; width: 32px; height: 32px; }
.oa-sidebar.sidebar > div:first-child { align-items: center; padding-inline: 12px; }
.oa-sidebar.sidebar:not(.oa-sidebar-open) > div:first-child { justify-content: center; }
.oa-sidebar.sidebar > div:first-child > div:first-child { border: 0; background: transparent; }

.nav-entry { display: block; }
.oa-sidebar .nav-entry { display: block; position: relative; }
.oa-sidebar .nav-item:not(.active) { border-color:var(--oa-surface); }
.oa-nav-signal { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); padding: 0 4px; min-width: 20px; text-align: center; border-radius: 6px; font: 500 12px/18px var(--yb-font-mono); color: rgb(var(--yb-warning-text)); background: rgb(var(--yb-warning-soft)); }
.oa-sidebar:not(.oa-sidebar-open) .oa-nav-signal { right: 0; top: 0; transform: none; }
.oa-bar-title { font: 600 15px/1.4 var(--oa-font); white-space: nowrap; }
.oa-bar-tabs,.oa-bar-status,.oa-bar-page-actions { display:none; }
@media (min-width:1280px) {
  .oa-client-shell .oa-sidebar.sidebar { --sidebar-rail:56px; --sidebar-drawer:172px; margin:0; height:100%; border:0; border-right:1px solid var(--oa-chrome-divider); border-radius:0; clip-path:none; background:var(--oa-surface); box-shadow:none; }
  .oa-client-shell .oa-sidebar::before,.oa-client-shell .oa-sidebar::after { display:none; }
  .oa-client-shell .oa-sidebar.sidebar > div:first-child { margin:0 0 8px; min-height:44px; height:44px; padding-inline:12px; }
  .oa-client-shell .oa-sidebar.sidebar .nav-entry { margin-block:2px; }
  .oa-client-shell .oa-sidebar.sidebar .nav-item { flex-direction:row; height:34px; min-height:34px; padding-block:0; }
  .oa-client-shell .oa-sidebar .oa-rail-caption { display:none; }
  .oa-client-shell .oa-sidebar:not(.oa-sidebar-open) > nav { padding-inline:4px; }
  .oa-client-shell .oa-sidebar.oa-sidebar-open .nav-section-label { font-size:var(--oa-nav-label-size); line-height:14px; padding-block:4px; }
  .oa-client-shell .app-topbar { margin:0; height:44px; min-height:44px; padding:0 16px; gap:12px; border:0; border-bottom:1px solid var(--oa-chrome-divider); border-radius:0; justify-content:initial; box-shadow:none; box-sizing:border-box; --topbar-control-size:32px; }
  .oa-client-shell .app-topbar__brand, .oa-client-shell .app-topbar__actions { flex:none; gap:6px; }
  .oa-client-shell .app-topbar__actions { margin-left:auto; }
  .oa-client-shell .app-topbar :deep(.topbar-icon) { border-radius:6px; }
  .oa-client-shell .app-topbar button.topbar-search { display:inline-flex; align-items:center; justify-content:flex-start; width:auto; min-width:220px; height:32px; flex:0 0 auto; gap:8px; padding:0 10px; border:1px solid var(--yb-line-control); border-radius:6px; }
  .oa-client-shell .app-topbar .topbar-search > svg { flex-shrink:0; display:block; }
  .oa-client-shell .app-topbar .topbar-search__label { white-space:nowrap; }

  .oa-bar-page-actions,.oa-bar-status { display:flex; align-items:center; gap:4px; min-width:0; }
  .oa-bar-page-actions:not(:has(button,a,input,select)),.oa-bar-status:not(:has(time,[role=status])) { display:none; }
  .oa-bar-tabs { flex:1 1 auto; min-width:0; display:flex; align-items:stretch; height:100%; overflow-x:auto; overflow-y:hidden; scrollbar-width:thin; }
  .oa-client-shell .content { width:100%; inline-size:100%; max-width:none; margin:0; padding:12px 16px; box-sizing:border-box; }
  .oa-client-shell .app-page-scroll[data-workspace] { min-height:0; overflow:auto; }
  .oa-client-shell .app-page-scroll[data-workspace] > .content { height:auto; min-height:100%; overflow:visible; }
  .oa-client-shell .app-page-scroll[data-workspace=list] { overflow:hidden; }
  .oa-client-shell .app-page-scroll[data-workspace=list] > .content { height:100%; min-height:0; }
}
@media (min-width:1440px) {
  .oa-client-shell .app-topbar .topbar-search__label,.oa-client-shell .app-topbar .oa-search-key { display:inline; line-height:16px; font-size:13px; }
}
@media (max-width:1439px) {
  .oa-client-shell .app-topbar button.topbar-search { width:32px; min-width:32px; height:32px; flex:0 0 32px; padding:0; gap:0; justify-content:center; border:1px solid var(--yb-line-control); border-radius:6px; }
  #app .oa-client-shell .app-topbar button.topbar-search > :is(.topbar-search__label,.oa-search-key) { display:none; }
}
/* Give compact menu names and their queue shortcuts separate pointer targets. */
@media (min-width:1280px) {
  .oa-sidebar.sidebar:not(.oa-sidebar-open) .nav-entry { height:44px; display:flex; align-items:flex-end; }
  .oa-sidebar.sidebar:not(.oa-sidebar-open) .nav-item { width:100%; height:28px; min-height:28px; }
  .oa-sidebar.sidebar:not(.oa-sidebar-open) .nav-entry .oa-nav-signal {
    position:absolute; top:2px; right:2px; transform:none; min-width:0;
    font-size:var(--oa-rail-badge-size); line-height:14px; padding:0 4px;
  }
  .oa-sidebar.sidebar.oa-sidebar-open .nav-entry:has(.oa-nav-signal) { display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:center; }
  .oa-sidebar.sidebar.oa-sidebar-open .nav-entry .oa-nav-signal { position:static; transform:none; }
}

.oa-search-key { white-space:nowrap; flex-shrink:0; }
@media (min-width:1280px) {
  .oa-client-shell .app-topbar__brand:has(.oa-bar-title) + .oa-bar-tabs:has(.operation-nav)::before {
    content:""; align-self:center; flex:0 0 1px; height:14px; background:var(--oa-chrome-divider); margin-inline-end:6px;
  }
}

.oa-compact-return {
  flex: none; padding: 4px 12px; background: var(--oa-surface); border-bottom: 1px solid var(--oa-chrome-divider);
}
.oa-bar-return:empty { display: none; }
.oa-context-title:empty,.oa-context-title:has(+ .oa-breadcrumb),.oa-context-title:not(:empty) + .oa-bar-title { display:none; }
.oa-context-title { white-space:nowrap; }

@media (min-width:1440px) {
  #app .oa-client-shell .app-topbar button.topbar-search { width:auto; min-width:220px; }
  #app .oa-client-shell .app-topbar button.topbar-search > :is(.topbar-search__label,.oa-search-key) { font-size:13px; line-height:normal; }
  #app .oa-client-shell .app-topbar button.topbar-search > svg { width:16px; height:16px; }
}
</style>
