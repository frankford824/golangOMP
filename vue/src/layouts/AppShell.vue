<template>
  <div class="app-shell bg-white font-body text-stone-800 flex overflow-hidden h-screen">
    <!-- Dark Slim Sidebar -->
    <aside
      class="sidebar hidden md:flex flex-col h-full w-20 bg-sidebar border-r border-stone-800/40 transition-all duration-300 overflow-hidden group hover:w-64"
      @transitionend="onSidebarTransitionEnd"
    >
      <div class="flex items-center gap-3 px-6 h-16 mb-4">
        <div class="w-8 h-8 rounded-lg bg-stone-600/30 flex items-center justify-center flex-shrink-0">
          <span class="material-symbols-outlined text-stone-300 text-lg">architecture</span>
        </div>
        <div class="opacity-0 group-hover:opacity-100 transition-opacity duration-300 whitespace-nowrap">
          <h2 class="font-headline font-extrabold text-stone-100 text-sm tracking-tight">永箔运营</h2>
        </div>
      </div>
      <nav class="flex-1 px-4 space-y-4 overflow-y-auto custom-scrollbar">
        <template v-if="workbenchMenus.length > 0">
          <div class="nav-section-label opacity-0 group-hover:opacity-100 transition-opacity">工作台</div>
          <router-link
            v-for="menu in workbenchMenus"
            :key="menu.key"
            :to="menu.to"
            :active-class="menu.exact ? '' : 'active'"
            :exact-active-class="menu.exact ? 'active' : ''"
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-stone-400 hover:text-stone-100 hover:bg-stone-700/40 sidebar-item-hover transition-all"
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
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-stone-400 hover:text-stone-100 hover:bg-stone-700/40 sidebar-item-hover transition-all"
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
            class="nav-item flex items-center gap-4 p-2 rounded-xl text-stone-400 hover:text-stone-100 hover:bg-stone-700/40 sidebar-item-hover transition-all"
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

    <main class="flex-1 flex flex-col h-full overflow-hidden relative min-w-0">
      <!-- TopNavBar: Minimal Glass -->
      <header class="flex justify-between items-center px-8 py-4 bg-white/60 backdrop-blur-xl border-b border-stone-200/60 z-10">
        <div class="flex items-center gap-12">
          <span class="text-lg font-headline font-extrabold tracking-tighter text-stone-900 uppercase">永箔运营管理系统</span>
        </div>
        <div class="flex items-center gap-6">
          <div class="relative hidden sm:block">
            <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 !text-lg pointer-events-none">search</span>
            <input
              type="text"
              readonly
              placeholder="Ctrl+K 搜索任务、SKU、产品"
              class="bg-stone-100/80 border border-stone-200/50 rounded-full pl-10 pr-4 py-1.5 text-xs w-64 focus:ring-1 focus:ring-stone-400 focus:border-stone-300 transition-all placeholder:text-stone-400"
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'
import GlobalSearchOverlay from '@/components/global-search/GlobalSearchOverlay.vue'
import NotificationBadge from '@/components/notification/NotificationBadge.vue'
import NotificationCenter from '@/components/notification/NotificationCenter.vue'
import AvatarDropdown from '@/components/layout/AvatarDropdown.vue'

const permissionsStore = usePermissionsStore()

const searchOpen = ref(false)
const notificationOpen = ref(false)

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
    to: '/assets',
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
    label: '报表中心',
    to: '/reports',
    aliases: ['kpi', 'finance'],
    section: 'data',
    icon: 'analytics',
  },
  {
    key: 'export_center',
    label: '导出中心',
    to: '/export-center',
    aliases: ['export_jobs'],
    section: 'data',
    icon: 'download',
  },
  {
    key: 'audit_log',
    label: '审计日志',
    to: '/audit-log',
    section: 'data',
    icon: 'history',
  },
  {
    key: 'logs_center',
    label: '日志管理',
    to: '/logs',
    aliases: ['logs_manage'],
    section: 'data',
    icon: 'description',
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
    key: 'kpi',
    label: '设计师 KPI',
    to: '/kpi',
    section: 'data',
    icon: 'analytics',
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

function openSearch() {
  searchOpen.value = true
}

function onGlobalKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    searchOpen.value = true
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKeydown))

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
  @apply bg-stone-600/50 text-stone-100;
}

.nav-badge-p2 {
  @apply inline-flex items-center ml-1 px-1.5 text-[10px] font-semibold rounded-full border border-stone-500/30 bg-stone-600/40 text-stone-200;
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
  @apply absolute right-0 top-[calc(100%+6px)] min-w-[180px] rounded-2xl border border-stone-200 bg-white/95 backdrop-blur-md shadow-float z-50 py-1.5;
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
</style>
