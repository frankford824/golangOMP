<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { Bell, Command, LogOut, Settings, UserRound } from 'lucide-vue-next'

import {
  appendSearchCommand,
  buildCommandItems,
  filterCommandItems,
  visibleDailyNavItems,
  type WorkbenchCommandItem,
} from '@aw/app/navigation'
import { firstAccessibleSettingsPath, hasSettingsAccess, isSettlementHubPath } from '@aw/app/access'
import { refreshUnreadCountKey } from '@aw/app/useWorkbenchUnread'
import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'
import { useWorkbenchSession } from '../app/useWorkbenchSession'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import { notificationsApi } from '@/services/api/notificationsApi'
import { currentBusinessMonth } from '../shared/format/businessMonth'
import IconfontActionIcon from '../shared/icons/IconfontActionIcon.vue'
import MotionReveal from '../shared/ui/MotionReveal.vue'

const SETTLEMENT_HUB_TAB_KEY = 'aw-settlement-hub-tab'

const route = useRoute()
const router = useRouter()
const commandOpen = ref(false)
const commandQuery = ref('')
const commandIndex = ref(0)
const commandInputRef = ref<HTMLInputElement | null>(null)
const commandNotice = ref('')
const unreadCount = ref(0)
const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()
const { logout } = useWorkbenchSession()

const dailyNavItems = computed(() => visibleDailyNavItems(bootstrap.value))
const showSettingsGear = computed(() => hasSettingsAccess(bootstrap.value))
const settingsTarget = computed(() => firstAccessibleSettingsPath(bootstrap.value) ?? '/settings')
const activeLabel = computed(() => String(route.meta.label || '资产工作台'))
const settingsActive = computed(() => route.path.startsWith('/settings'))
const bellLabel = computed(() => (unreadCount.value > 0 ? `消息，未读 ${unreadCount.value} 条` : '消息'))
const bellBadge = computed(() => {
  if (unreadCount.value <= 0) return ''
  if (unreadCount.value >= 100) return '99+'
  return String(unreadCount.value)
})

const commandHandlers = {
  navigate: async (to: string) => {
    await router.push(to)
    closeCommand()
  },
  markAllRead: async () => {
    await assetWorkbenchApi.markAllNotificationsRead()
    unreadCount.value = 0
    closeCommand()
  },
  searchOverview: async (query: string) => {
    await router.push({ path: '/drive', query: { q: query, scope: 'all' } })
    closeCommand()
  },
  generateSettlement: async () => {
    commandNotice.value = '请在本页确认业务月后再生成结算批次'
    await router.push('/settlement')
    closeCommand()
  },
  exportSettlement: async () => {
    commandNotice.value = '请在本页选择批次后导出工资'
    await router.push('/settlement')
    closeCommand()
  },
  exportReport: async () => {
    commandNotice.value = '请在本页确认业务月后导出计件统计'
    await router.push('/reports')
    closeCommand()
  },
}

const baseCommandItems = computed(() => buildCommandItems(bootstrap.value, commandHandlers))

const filteredCommandItems = computed(() => {
  const filtered = filterCommandItems(baseCommandItems.value, commandQuery.value)
  return appendSearchCommand(filtered, commandQuery.value, bootstrap.value, commandHandlers.searchOverview)
})

const groupedCommandItems = computed(() => {
  const groups = new Map<string, WorkbenchCommandItem[]>()
  for (const item of filteredCommandItems.value) {
    const bucket = groups.get(item.group) ?? []
    bucket.push(item)
    groups.set(item.group, bucket)
  }
  return [...groups.entries()].map(([label, items]) => ({ label, items }))
})

const flatCommandItems = computed(() => groupedCommandItems.value.flatMap((group) => group.items))

const profileStatusLabel = computed(() => {
  const status = bootstrap.value?.profile?.status
  if (!status) return '资料待完善，请补全资料'
  const labels: Record<string, string> = {
    pending: '资料审核中',
    active: '资料已完成',
    disabled: '账号已停用',
  }
  return labels[status] ?? '资料状态待确认'
})

const permissionSummary = computed(() => {
  const count = bootstrap.value?.capabilities.length ?? 0
  return count > 0 ? '可用功能已就绪' : '暂无可用功能，请联系管理员'
})

const profileState = computed(() => {
  const status = bootstrap.value?.profile?.status
  if (status === 'active') return 'active'
  if (status === 'pending') return 'pending'
  return 'idle'
})

const businessMonth = computed(() => currentBusinessMonth())

const displayName = computed(
  () => bootstrap.value?.profile?.real_name || bootstrap.value?.user?.name || bootstrap.value?.user?.username || '我的账号',
)

function settlementHubTarget() {
  if (typeof window === 'undefined') return '/settlement'
  const remembered = window.localStorage.getItem(SETTLEMENT_HUB_TAB_KEY)
  if (remembered === '/settlement' || remembered === '/reports') return remembered
  return '/settlement'
}

function navActive(item: { to: string; hub?: 'settlement' }) {
  if (item.hub === 'settlement') return isSettlementHubPath(route.path)
  return route.path === item.to
}

function navAriaLabel(item: { label: string; subtitle?: string }) {
  return item.subtitle ? `${item.label}，${item.subtitle}` : item.label
}

async function refreshUnreadCount() {
  try {
    const res = await notificationsApi.unreadCount()
    const root = res.data && typeof res.data === 'object' ? (res.data as Record<string, unknown>) : {}
    const body = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
    const count = body.unread_count ?? body.count ?? 0
    unreadCount.value = Number.isFinite(Number(count)) ? Number(count) : 0
  } catch {
    unreadCount.value = 0
  }
}

async function openCommand() {
  commandOpen.value = true
  commandIndex.value = 0
  commandNotice.value = ''
  await nextTick()
  commandInputRef.value?.focus()
}

function toggleCommand() {
  if (commandOpen.value) {
    closeCommand()
    return
  }
  void openCommand()
}

function closeCommand() {
  commandOpen.value = false
  commandQuery.value = ''
  commandIndex.value = 0
}

async function runCommand(item: WorkbenchCommandItem) {
  if (item.run) await item.run()
}

function moveCommandSelection(delta: number) {
  const total = flatCommandItems.value.length
  if (!total) return
  commandIndex.value = (commandIndex.value + delta + total) % total
}

function runSelectedCommand() {
  const item = flatCommandItems.value[commandIndex.value]
  if (item) void runCommand(item)
}

function flatIndex(item: WorkbenchCommandItem) {
  return flatCommandItems.value.findIndex((entry) => entry.id === item.id)
}

function handleKeydown(event: KeyboardEvent) {
  const isCommand = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k'
  if (isCommand) {
    event.preventDefault()
    toggleCommand()
    return
  }
  if (event.key === 'Escape') {
    closeCommand()
  }
}

function handleCommandKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveCommandSelection(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveCommandSelection(-1)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    runSelectedCommand()
  }
}

provide(refreshUnreadCountKey, refreshUnreadCount)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  void refresh()
  void refreshUnreadCount()
})

onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))

watch(
  () => route.path,
  (path, previousPath) => {
    if (path === '/notifications' || previousPath === '/notifications') void refreshUnreadCount()
  },
)

watch(commandQuery, () => {
  commandIndex.value = 0
})
</script>

<template>
  <div class="aw-root">
    <aside class="aw-shell__rail" aria-label="资产工作台导航">
      <div class="aw-shell__rail-inner">
        <div class="aw-shell__brand">
          <span class="aw-shell__mark">AW</span>
          <span class="aw-shell__brand-text">资产工作台</span>
        </div>

        <nav class="aw-shell__nav">
          <RouterLink
            v-for="item in dailyNavItems"
            :key="item.to"
            :to="item.hub === 'settlement' ? settlementHubTarget() : item.to"
            class="aw-nav-item"
            :class="{ 'aw-nav-item--active': navActive(item) }"
            :aria-label="navAriaLabel(item)"
            :aria-current="navActive(item) ? 'page' : undefined"
          >
            <component :is="item.icon" :size="18" stroke-width="2" aria-hidden="true" />
            <span class="aw-nav-item__stack">
              <span>{{ item.label }}</span>
              <small v-if="navActive(item) && item.subtitle">{{ item.subtitle }}</small>
            </span>
          </RouterLink>
        </nav>
      </div>
    </aside>

    <section class="aw-shell__workspace">
      <header class="aw-topbar">
        <div>
          <p class="aw-eyebrow">{{ settingsActive ? '设置' : '资产交付与计件结算' }}</p>
          <h1>{{ activeLabel }}</h1>
        </div>
        <div class="aw-page-bar__actions">
          <button class="aw-command-button" type="button" @click="toggleCommand">
            <IconfontActionIcon name="search" :size="16" />
            <span>搜索或执行动作</span>
            <kbd>⌘K</kbd>
          </button>
          <RouterLink class="aw-icon-action aw-icon-action--bell" to="/notifications" :aria-label="bellLabel" @click="refreshUnreadCount">
            <Bell :size="18" aria-hidden="true" />
            <span v-if="bellBadge" class="aw-icon-action__badge" aria-hidden="true">{{ bellBadge }}</span>
          </RouterLink>
          <RouterLink
            v-if="showSettingsGear"
            class="aw-icon-action"
            :class="{ 'aw-icon-action--active': settingsActive }"
            :to="settingsTarget"
            aria-label="设置"
          >
            <Settings :size="18" aria-hidden="true" />
          </RouterLink>
          <RouterLink class="aw-secondary-button" to="/account">
            <UserRound :size="16" aria-hidden="true" />
            {{ displayName }}
          </RouterLink>
          <button class="aw-secondary-button" type="button" @click="logout">
            <LogOut :size="16" aria-hidden="true" />
            退出
          </button>
        </div>
      </header>

      <main class="aw-shell__content">
        <div v-if="error" class="aw-inline-alert">{{ error }}</div>
        <div v-else-if="loading" class="aw-inline-alert">正在进入工作台</div>
        <div v-else-if="bootstrap" class="aw-statusline">
          <span class="aw-statusline__dot" :data-state="profileState"></span>
          <span class="aw-statusline__item">本月结算 <b>{{ businessMonth }}</b>（北京时间）</span>
          <span class="aw-statusline__sep">·</span>
          <span class="aw-statusline__item">{{ profileStatusLabel }}</span>
          <span class="aw-statusline__sep">·</span>
          <span class="aw-statusline__item">{{ permissionSummary }}</span>
        </div>
        <RouterView v-slot="{ Component, route: activeRoute }">
          <MotionReveal :key="activeRoute.path" class="aw-route-view">
            <component :is="Component" />
          </MotionReveal>
        </RouterView>
      </main>
    </section>

    <div v-if="commandOpen" class="aw-command" role="dialog" aria-modal="true" aria-label="命令面板" @keydown="handleCommandKeydown">
      <div class="aw-command__panel">
        <div class="aw-command__input">
          <Command :size="18" aria-hidden="true" />
          <input
            ref="commandInputRef"
            v-model="commandQuery"
            type="search"
            placeholder="跳转页面、搜索旧菜单名或执行动作"
            aria-controls="aw-command-list"
          />
        </div>
        <div id="aw-command-list" class="aw-command__list">
          <template v-for="group in groupedCommandItems" :key="group.label">
            <p class="aw-command__group">{{ group.label }}</p>
            <button
              v-for="item in group.items"
              :key="item.id"
              class="aw-command__item"
              :class="{ 'aw-command__item--active': flatIndex(item) === commandIndex }"
              type="button"
              :aria-selected="flatIndex(item) === commandIndex"
              @mouseenter="commandIndex = flatIndex(item)"
              @click="runCommand(item)"
            >
              <component :is="item.icon" v-if="item.icon" :size="16" aria-hidden="true" />
              <span class="aw-command__item-copy">
                <strong>{{ item.label }}</strong>
                <small v-if="item.subtitle">{{ item.subtitle }}</small>
              </span>
            </button>
          </template>
          <p v-if="flatCommandItems.length === 0" class="aw-command__empty">
            没有匹配的页面或动作，试试：总盘、计价设置、交作品
          </p>
          <p v-if="commandNotice" class="aw-command__notice">{{ commandNotice }}</p>
        </div>
      </div>
      <button class="aw-command__scrim" type="button" aria-label="关闭命令面板" @click="closeCommand" />
    </div>
  </div>
</template>
