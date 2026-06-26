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
  ReceiptText,
  ScrollText,
  Search,
  UsersRound,
} from 'lucide-vue-next'

import { useAssetWorkbenchBootstrap } from '../app/useAssetWorkbenchBootstrap'

const route = useRoute()
const router = useRouter()
const commandOpen = ref(false)
const commandQuery = ref('')
const commandIndex = ref(0)
const commandInputRef = ref<HTMLInputElement | null>(null)
const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()

const navItems = [
  { to: '/', label: '总览', icon: LayoutDashboard, requires: ['asset.workbench.bootstrap'] },
  { to: '/upload', label: '上传', icon: FileUp, requires: ['asset.workbench.submit'] },
  { to: '/submissions', label: '维护', icon: Boxes, requires: ['asset.workbench.submit', 'asset.workbench.manage', 'asset.workbench.settlement'] },
  { to: '/materials', label: '素材', icon: Library, requires: ['asset.workbench.system_search'] },
  { to: '/cost-center', label: '成本', icon: Calculator, requires: ['asset.workbench.cost_center.manage'] },
  { to: '/settlement', label: '结算', icon: ReceiptText, requires: ['asset.workbench.settlement'] },
  { to: '/people', label: '人员', icon: UsersRound, requires: ['asset.workbench.profile', 'asset.workbench.profile.manage'] },
  { to: '/events', label: '日志', icon: ScrollText, requires: ['asset.workbench.manage', 'asset.workbench.cost_center.manage', 'asset.workbench.settlement'] },
]

const activeLabel = computed(() => String(route.meta.label || '资产工作台'))
const visibleNavItems = computed(() => navItems.filter((item) => hasAnyCapability(item.requires)))
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
  return count > 0 ? '权限已加载' : '暂无可用权限'
})

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
        <RouterLink
          v-for="item in visibleNavItems"
          :key="item.to"
          :to="item.to"
          class="aw-nav-item"
          active-class="aw-nav-item--active"
        >
          <component :is="item.icon" :size="18" stroke-width="2" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
    </aside>

    <section class="aw-shell__workspace">
      <header class="aw-topbar">
        <div>
          <p class="aw-eyebrow">资产交付与计件结算</p>
          <h1>{{ activeLabel }}</h1>
        </div>
        <button class="aw-command-button" type="button" @click="toggleCommand">
          <Search :size="16" aria-hidden="true" />
          <span>搜索或执行动作</span>
          <kbd>⌘K</kbd>
        </button>
      </header>

      <main class="aw-shell__content">
        <div v-if="error" class="aw-inline-alert">{{ error }}</div>
        <div v-else-if="loading" class="aw-inline-alert">正在加载工作台信息</div>
        <div v-else-if="bootstrap" class="aw-context-strip">
          <span>业务月份按北京时间计算</span>
          <span>{{ profileStatusLabel }}</span>
          <span>{{ permissionSummary }}</span>
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
