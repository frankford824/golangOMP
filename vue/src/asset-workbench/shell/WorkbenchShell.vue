<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import {
  Boxes,
  Calculator,
  Command,
  FileUp,
  LayoutDashboard,
  Library,
  LogOut,
  ReceiptText,
  ScrollText,
  Search,
  Send,
  UserRound,
  UsersRound,
} from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'
import { useWorkbenchSession } from '../app/useWorkbenchSession'

const route = useRoute()
const router = useRouter()
const commandOpen = ref(false)
const commandQuery = ref('')
const commandIndex = ref(0)
const commandInputRef = ref<HTMLInputElement | null>(null)
const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()
const { logout } = useWorkbenchSession()

const navItems = [
  { group: '工作台', to: '/', label: '总览', icon: LayoutDashboard, requires: ['asset.workbench.bootstrap'] },
  { group: '工作台', to: '/upload', label: '交作品', icon: FileUp, requires: ['asset.workbench.submit'] },
  { group: '工作台', to: '/submissions', label: '维护区', icon: Boxes, requires: ['asset.workbench.submit', 'asset.workbench.manage', 'asset.workbench.settlement'] },
  { group: '工作台', to: '/materials', label: '素材库', icon: Library, requires: ['asset.workbench.system_search'] },
  { group: '管理', to: '/template-assignments', label: '作品下发', icon: Send, requires: ['asset.workbench.template.assign'] },
  { group: '管理', to: '/cost-center', label: '成本中心', icon: Calculator, requires: ['asset.workbench.cost_center.manage'] },
  { group: '管理', to: '/settlement', label: '结算工资', icon: ReceiptText, requires: ['asset.workbench.settlement'] },
  { group: '管理', to: '/people', label: '人员档案', icon: UsersRound, requires: ['asset.workbench.profile', 'asset.workbench.profile.manage'] },
  { group: '管理', to: '/members', label: '成员权限', icon: UserRound, requires: ['asset.workbench.member.identity'] },
  { group: '管理', to: '/events', label: '操作日志', icon: ScrollText, requires: ['asset.workbench.manage', 'asset.workbench.cost_center.manage', 'asset.workbench.settlement'] },
]

const activeLabel = computed(() => String(route.meta.label || '资产工作台'))
const visibleNavItems = computed(() => navItems.filter((item) => hasAnyCapability(item.requires)))
const visibleNavGroups = computed(() => {
  const groups: Array<{ label: string; items: typeof navItems }> = []
  for (const label of ['工作台', '管理']) {
    const items = visibleNavItems.value.filter((item) => item.group === label)
    if (items.length) groups.push({ label, items })
  }
  return groups
})
const filteredCommandItems = computed(() => {
  const query = commandQuery.value.trim().toLowerCase()
  const source = visibleNavItems.value
  if (!query) return source
  return source.filter((item) => {
    const haystack = `${item.label} ${item.to}`.toLowerCase()
    return haystack.includes(query)
  })
})
const profileStatusLabel = computed(() => {
  const status = bootstrap.value?.profile?.status
  if (!status) return '档案待完善'
  const labels: Record<string, string> = {
    pending: '档案待审核',
    active: '档案已生效',
    disabled: '档案已停用',
  }
  return labels[status] ?? status
})
const permissionSummary = computed(() => {
  const count = bootstrap.value?.capabilities.length ?? 0
  return count > 0 ? `${count} 项权限已加载` : '暂无可用权限'
})
const profileState = computed(() => {
  const status = bootstrap.value?.profile?.status
  if (status === 'active') return 'active'
  if (status === 'pending') return 'pending'
  return 'idle'
})
const businessMonth = computed(() =>
  new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit' }).format(new Date()),
)
const displayName = computed(() => bootstrap.value?.profile?.real_name || bootstrap.value?.user?.name || bootstrap.value?.user?.username || '我的账号')

function hasAnyCapability(required: string[]) {
  const capabilities = new Set(bootstrap.value?.capabilities ?? [])
  return required.some((item) => capabilities.has(item))
}

async function openCommand() {
  commandOpen.value = true
  commandIndex.value = 0
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

async function jump(to: string) {
  await router.push(to)
  closeCommand()
}

function moveCommandSelection(delta: number) {
  const total = filteredCommandItems.value.length
  if (!total) return
  commandIndex.value = (commandIndex.value + delta + total) % total
}

function runSelectedCommand() {
  const item = filteredCommandItems.value[commandIndex.value]
  if (item) void jump(item.to)
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

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  void refresh()
})
onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))

watch(commandQuery, () => {
  commandIndex.value = 0
})
</script>

<template>
  <div class="aw-root">
    <aside class="aw-shell__rail" aria-label="资产工作台导航">
      <div class="aw-shell__brand">
        <span class="aw-shell__mark">AW</span>
        <span class="aw-shell__brand-text">资产工作台</span>
      </div>

      <nav class="aw-shell__nav">
        <section v-for="group in visibleNavGroups" :key="group.label" class="aw-nav-group">
          <p>{{ group.label }}</p>
          <RouterLink
            v-for="item in group.items"
            :key="item.to"
            :to="item.to"
            class="aw-nav-item"
            active-class="aw-nav-item--active"
          >
            <component :is="item.icon" :size="18" stroke-width="2" aria-hidden="true" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </section>
      </nav>
    </aside>

    <section class="aw-shell__workspace">
      <header class="aw-topbar">
        <div>
          <p class="aw-eyebrow">资产交付与计件结算</p>
          <h1>{{ activeLabel }}</h1>
        </div>
        <div class="aw-page-bar__actions">
          <button class="aw-command-button" type="button" @click="toggleCommand">
            <Search :size="16" aria-hidden="true" />
            <span>搜索或执行动作</span>
            <kbd>⌘K</kbd>
          </button>
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
        <div v-else-if="loading" class="aw-inline-alert">正在加载工作台信息</div>
        <div v-else-if="bootstrap" class="aw-statusline">
          <span class="aw-statusline__dot" :data-state="profileState"></span>
          <span class="aw-statusline__item">业务月 <b>{{ businessMonth }}</b>（北京时间）</span>
          <span class="aw-statusline__sep">·</span>
          <span class="aw-statusline__item">{{ profileStatusLabel }}</span>
          <span class="aw-statusline__sep">·</span>
          <span class="aw-statusline__item">{{ permissionSummary }}</span>
        </div>
        <RouterView />
      </main>
    </section>

    <div v-if="commandOpen" class="aw-command" role="dialog" aria-modal="true" aria-label="命令面板" @keydown="handleCommandKeydown">
      <div class="aw-command__panel">
        <div class="aw-command__input">
          <Command :size="18" aria-hidden="true" />
          <input ref="commandInputRef" v-model="commandQuery" type="search" placeholder="跳转页面或执行常用动作" />
        </div>
        <div class="aw-command__list">
          <button
            v-for="(item, index) in filteredCommandItems"
            :key="item.to"
            class="aw-command__item"
            :class="{ 'aw-command__item--active': index === commandIndex }"
            type="button"
            @mouseenter="commandIndex = index"
            @click="jump(item.to)"
          >
            <component :is="item.icon" :size="16" aria-hidden="true" />
            <span>{{ item.label }}</span>
          </button>
          <p v-if="filteredCommandItems.length === 0" class="aw-command__empty">没有匹配动作</p>
        </div>
      </div>
      <button class="aw-command__scrim" type="button" aria-label="关闭命令面板" @click="closeCommand" />
    </div>
  </div>
</template>
