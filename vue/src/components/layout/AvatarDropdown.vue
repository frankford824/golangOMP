<template>
  <div ref="rootEl" class="relative">
    <button
      type="button"
      class="inline-flex items-center rounded-full border border-neutral-300 bg-white px-3 py-1 text-xs font-bold text-neutral-900 shadow-sm transition-colors hover:bg-neutral-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-stone-400/35 focus-visible:ring-offset-2"
      :aria-expanded="open"
      aria-haspopup="menu"
      :aria-controls="open ? userMenuId : undefined"
      @click="open = !open"
    >
      {{ userName }}
    </button>
    <div
      v-show="open"
      :id="userMenuId"
      role="menu"
      class="avatar-dropdown-menu absolute right-0 top-[calc(100%+10px)] z-[200] flex w-[17rem] flex-col gap-1.5 rounded-[1.25rem] border border-neutral-200/90 bg-white px-2 pb-2 pt-2 shadow-[var(--v1-xhs-card-shadow)]"
    >
      <router-link
        v-for="item in accountItems"
        :key="item.to"
        role="menuitem"
        :to="item.to"
        :class="['avatar-menu-row', { 'avatar-menu-row--current': linkIsCurrent(item.to) }]"
        active-class=""
        exact-active-class=""
        @click="closeMenu"
      >
        {{ item.label }}
      </router-link>
      <div class="avatar-menu-sep avatar-menu-sep--subtle" aria-hidden="true" />
      <router-link
        v-for="item in taskItems"
        :key="item.to"
        role="menuitem"
        :to="item.to"
        :class="['avatar-menu-row', { 'avatar-menu-row--current': linkIsCurrent(item.to) }]"
        active-class=""
        exact-active-class=""
        @click="closeMenu"
      >
        {{ item.label }}
      </router-link>
      <div class="avatar-menu-sep" aria-hidden="true" />
      <button type="button" role="menuitem" class="avatar-menu-logout" @click="logout">退出</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePermissionsStore } from '@/stores/permissions'

const route = useRoute()
const router = useRouter()
const permissionsStore = usePermissionsStore()
const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)
const userMenuId = 'avatar-user-menu'

const userName = computed(() => permissionsStore.currentUser?.name ?? '未登录')

/** 与 pencil 稿一致：账户与任务分区，避免整块视觉混在一团 */
const accountItems = [
  { label: '个人中心', to: '/me' },
  { label: '安全设置', to: '/me/security' },
  { label: '我的组织', to: '/me/org' },
  { label: '通知中心', to: '/me/notifications' },
]

const taskItems = [
  { label: '我的任务 · 进行中', to: '/tasks?tab=mine&status=InProgress' },
  { label: '我的任务 · 已完成', to: '/tasks?tab=mine&status=Completed' },
  { label: '我的任务 · 已终止', to: '/tasks?tab=mine&status=Cancelled' },
  { label: '我的任务 · 草稿', to: '/me/task-drafts' },
]

/** 仅当前地址与链接解析结果 fullPath 完全一致时高亮，避免出现多个「我的任务」同时带左条 */
function linkIsCurrent(to: string): boolean {
  try {
    return router.resolve(to).fullPath === route.fullPath
  } catch {
    return false
  }
}

function closeMenu(): void {
  open.value = false
}

function onDocumentPointerDown(event: Event): void {
  if (!open.value) return
  const el = rootEl.value
  const target = event.target
  if (!el || !(target instanceof Node) || el.contains(target)) return
  closeMenu()
}

function onDocumentKeydown(event: KeyboardEvent): void {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeMenu()
  }
}

watch(
  () => route.fullPath,
  () => {
    closeMenu()
  },
)

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown, true)
  document.addEventListener('keydown', onDocumentKeydown, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown, true)
  document.removeEventListener('keydown', onDocumentKeydown, true)
})

function logout(): void {
  closeMenu()
  permissionsStore.logout()
  void router.push('/login')
}
</script>

<style scoped>
/* designs/xiaohongshu-menu.pen：顶栏胶囊石灰中性；品牌色用于下拉当前行与面板内主操作 */
.avatar-dropdown-menu :deep(a.avatar-menu-row) {
  display: block;
  width: 100%;
  box-sizing: border-box;
  position: relative;
  text-decoration: none !important;
  border-radius: 0.875rem;
  border: 1px solid #ebebee;
  background: #fff;
  padding: 0.5rem 0.7rem;
  padding-left: 0.75rem;
  font-size: 0.8125rem;
  font-weight: 600;
  line-height: 1.35;
  text-align: left;
  color: #1c1c1e !important;
  transition:
    background-color 0.12s ease,
    border-color 0.12s ease;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row:hover),
.avatar-dropdown-menu :deep(a.avatar-menu-row:focus-visible) {
  outline: none;
  text-decoration: none !important;
  background-color: var(--v1-xhs-brand-soft);
  border-color: var(--v1-xhs-brand-border);
  color: #1c1c1e !important;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row--current) {
  background-color: var(--v1-xhs-brand-soft);
  border-color: var(--v1-xhs-brand-border-strong);
  font-weight: 800;
  color: #1c1c1e !important;
  padding-left: 0.85rem;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row--current::before) {
  content: '';
  position: absolute;
  left: 0.4rem;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 0.9375rem;
  border-radius: 1px;
  background: var(--v1-xhs-brand);
}

.avatar-menu-sep {
  margin-top: 0.05rem;
  margin-bottom: 0;
  border-top: 1px solid #e8e8ec;
}

.avatar-menu-sep--subtle {
  opacity: 1;
}

.avatar-menu-logout {
  display: block;
  width: 100%;
  box-sizing: border-box;
  cursor: pointer;
  border-radius: 0.875rem;
  border: 1px solid #fecaca;
  background: #fff;
  padding: 0.5rem 0.7rem;
  font-size: 0.8125rem;
  font-weight: 700;
  line-height: 1.35;
  color: var(--v1-danger);
  text-align: left;
  transition:
    background-color 0.12s ease,
    border-color 0.12s ease;
}

.avatar-menu-logout:hover,
.avatar-menu-logout:focus-visible {
  outline: none;
  border-color: #f87171;
  background: #fef2f2;
}

button[aria-haspopup='menu'] {
  border-color: #d4d4d8 !important;
  background: #ffffff !important;
  color: #111827 !important;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
}

button[aria-haspopup='menu']:hover,
button[aria-haspopup='menu']:focus-visible {
  background: #f9fafb !important;
  border-color: #cbd5e1 !important;
}

.avatar-dropdown-menu {
  z-index: 7300 !important;
  pointer-events: auto;
  overflow: hidden;
  gap: 0.4rem !important;
  padding: 0.55rem !important;
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  box-shadow: 0 20px 40px -18px rgba(15, 23, 42, 0.18) !important;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row) {
  border-radius: 0.75rem;
  border-color: #e5e7eb;
  background: #ffffff;
  color: #111827 !important;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row:hover),
.avatar-dropdown-menu :deep(a.avatar-menu-row:focus-visible),
.avatar-dropdown-menu :deep(a.avatar-menu-row--current) {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8 !important;
}

.avatar-dropdown-menu :deep(a.avatar-menu-row--current::before) {
  background: #2563eb;
}

.avatar-menu-sep {
  border-color: #e5e7eb;
}

.avatar-menu-logout {
  border-color: #fecaca;
  background: #ffffff;
  color: #dc2626;
}

.avatar-menu-logout:hover,
.avatar-menu-logout:focus-visible {
  border-color: #fca5a5;
  background: #fef2f2;
}
</style>
