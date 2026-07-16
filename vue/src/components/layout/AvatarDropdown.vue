<template>
  <div ref="rootEl" class="relative">
    <button
      type="button"
      class="avatar-trigger"
      :aria-expanded="open"
      aria-haspopup="menu"
      :aria-controls="open ? userMenuId : undefined"
      @click="open = !open"
    >
      <span class="avatar-trigger__photo">
        <img v-if="userAvatar" :src="userAvatar" alt="头像" />
        <span v-else>{{ userInitial }}</span>
      </span>
      <span class="avatar-trigger__name">{{ userName }}</span>
      <ChevronDown :size="15" aria-hidden="true" />
    </button>

    <div v-show="open" :id="userMenuId" role="menu" class="avatar-dropdown-menu">
      <div class="avatar-summary" aria-hidden="true">
        <span class="avatar-summary__photo">
          <img v-if="userAvatar" :src="userAvatar" alt="" />
          <span v-else>{{ userInitial }}</span>
        </span>
        <div>
          <strong>{{ userName }}</strong>
          <small>{{ accountLabel }}</small>
        </div>
      </div>

      <section class="avatar-menu-group" aria-label="账户">
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
          <component :is="item.icon" :size="15" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </router-link>
      </section>

      <section class="avatar-task-group" aria-label="我的任务">
        <div class="avatar-task-group__title">
          <ClipboardList :size="15" aria-hidden="true" />
          <span>我的任务</span>
        </div>
        <div class="avatar-task-tabs">
          <router-link
            v-for="item in taskStatusItems"
            :key="item.key"
            role="menuitem"
            :to="item.to"
            :class="{ 'avatar-task-tabs__item--current': taskItemIsCurrent(item) }"
            active-class=""
            exact-active-class=""
            @click="closeMenu"
          >
            {{ item.label }}
          </router-link>
        </div>
      </section>

      <button type="button" role="menuitem" class="avatar-menu-logout" @click="logout">
        <LogOut :size="15" aria-hidden="true" />
        <span>退出登录</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Bell,
  Building2,
  ChevronDown,
  ClipboardList,
  LockKeyhole,
  LogOut,
  UserRound,
} from 'lucide-vue-next'
import { usePermissionsStore } from '@/stores/permissions'

interface TaskStatusItem {
  key: string
  label: string
  to: string
  status?: string
  draft?: boolean
}

const route = useRoute()
const router = useRouter()
const permissionsStore = usePermissionsStore()
const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)
const userMenuId = 'avatar-user-menu'

const userName = computed(() => permissionsStore.currentUser?.name ?? '未登录')
const userAvatar = computed(() => permissionsStore.currentUser?.avatarUrl || permissionsStore.currentUser?.avatar || '')
const userInitial = computed(() => userName.value.trim().slice(0, 1).toUpperCase() || '我')
const accountLabel = computed(
  () => permissionsStore.currentUser?.account || permissionsStore.currentUser?.username || permissionsStore.currentUser?.id || '账号',
)

const accountItems = [
  { label: '个人中心', to: '/me', icon: UserRound },
  { label: '安全设置', to: '/me/security', icon: LockKeyhole },
  { label: '我的组织', to: '/me/org', icon: Building2 },
  { label: '通知中心', to: '/me/notifications', icon: Bell },
]

const taskStatusItems: TaskStatusItem[] = [
  { key: 'all', label: '全部', to: '/tasks?tab=mine' },
  { key: 'progress', label: '进行中', to: '/tasks?tab=mine&status=InProgress', status: 'InProgress' },
  {
    key: 'audit',
    label: '待审核',
    to: '/tasks?tab=mine&status=PendingAudit',
    status: 'PendingAudit',
  },
  { key: 'completed', label: '已完成', to: '/tasks?tab=mine&status=Completed', status: 'Completed' },
  { key: 'cancelled', label: '已终止', to: '/tasks?tab=mine&status=Cancelled', status: 'Cancelled' },
  { key: 'drafts', label: '草稿', to: '/me/task-drafts', draft: true },
]

function linkIsCurrent(to: string): boolean {
  try {
    return router.resolve(to).fullPath === route.fullPath
  } catch {
    return false
  }
}

function normalizeStatus(value: unknown): string {
  if (Array.isArray(value)) return value.map((item) => String(item)).join(',')
  return String(value ?? '')
}

function taskItemIsCurrent(item: TaskStatusItem): boolean {
  if (item.draft) return route.path === '/me/task-drafts'
  if (route.path !== '/tasks' || route.query.tab !== 'mine') return false
  const currentStatus = normalizeStatus(route.query.status)
  if (!item.status) return currentStatus === ''
  return currentStatus === item.status
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
.avatar-trigger {
  display: inline-flex;
  max-width: 220px;
  height: 38px;
  align-items: center;
  gap: 8px;
  border: 1px solid rgb(var(--yb-border-subtle));
  border-radius: 999px;
  background: rgb(var(--yb-surface));
  padding: 3px 10px 3px 4px;
  color: rgb(var(--yb-text-navy));
  font-size: 13px;
  font-weight: 800;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.08);
  cursor: pointer;
  transition:
    background-color 0.14s ease,
    border-color 0.14s ease,
    box-shadow 0.14s ease;
}

.avatar-trigger:hover,
.avatar-trigger:focus-visible {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-surface-subtle));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.1);
  outline: none;
}

.avatar-trigger__photo,
.avatar-summary__photo {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  background: linear-gradient(135deg, rgb(var(--yb-brand)), rgb(var(--yb-teal-accent)));
  color: rgb(var(--yb-text-inverse));
  font-weight: 900;
}

.avatar-trigger__photo {
  width: 30px;
  height: 30px;
  font-size: 12px;
}

.avatar-summary__photo {
  width: 42px;
  height: 42px;
  font-size: 16px;
}

.avatar-trigger__photo img,
.avatar-summary__photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-trigger__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.avatar-dropdown-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 10px);
  z-index: 7300;
  display: flex;
  width: 21rem;
  max-width: min(92vw, 21rem);
  flex-direction: column;
  gap: 10px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 16px;
  background: rgb(var(--yb-surface));
  padding: 12px;
  box-shadow: 0 22px 44px -20px rgb(var(--yb-shadow) / 0.24);
}

.avatar-summary {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
  border-radius: 12px;
  background: rgb(var(--yb-surface-subtle));
  padding: 10px;
}

.avatar-summary div {
  min-width: 0;
}

.avatar-summary strong,
.avatar-summary small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.avatar-summary strong {
  color: rgb(var(--yb-text));
  font-size: 14px;
  font-weight: 900;
}

.avatar-summary small {
  margin-top: 3px;
  color: rgb(var(--yb-text-secondary));
  font-size: 12px;
}

.avatar-menu-group {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.avatar-menu-row,
.avatar-menu-logout,
.avatar-task-tabs a,
.avatar-menu-row:hover,
.avatar-menu-row:focus-visible,
.avatar-menu-logout:hover,
.avatar-menu-logout:focus-visible,
.avatar-task-tabs a:hover,
.avatar-task-tabs a:focus-visible {
  text-decoration: none;
}

.avatar-menu-row {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 10px;
  background: rgb(var(--yb-surface));
  padding: 9px 10px;
  color: rgb(var(--yb-text-slate));
  font-size: 13px;
  font-weight: 800;
}

.avatar-menu-row span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.avatar-menu-row:hover,
.avatar-menu-row:focus-visible,
.avatar-menu-row--current {
  border-color: rgb(var(--yb-brand-border));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  outline: none;
}

.avatar-task-group {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 12px;
  padding: 10px;
}

.avatar-task-group__title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: rgb(var(--yb-text-navy));
  font-size: 13px;
  font-weight: 900;
}

.avatar-task-tabs {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 7px;
  margin-top: 9px;
}

.avatar-task-tabs a {
  display: inline-flex;
  min-height: 30px;
  min-width: 0;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 999px;
  background: rgb(var(--yb-surface));
  padding: 0 8px;
  color: rgb(var(--yb-text-soft));
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.avatar-task-tabs a:hover,
.avatar-task-tabs a:focus-visible,
.avatar-task-tabs__item--current {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand-strong));
  outline: none;
}

.avatar-menu-logout {
  display: inline-flex;
  width: 100%;
  min-height: 36px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border: 1px solid rgb(var(--yb-danger-border));
  border-radius: 10px;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-danger));
  font-size: 13px;
  font-weight: 900;
  cursor: pointer;
}

.avatar-menu-logout:hover,
.avatar-menu-logout:focus-visible {
  border-color: rgb(var(--yb-danger-border-hover));
  background: rgb(var(--yb-danger-soft));
  outline: none;
}

@media (max-width: 640px) {
  .avatar-trigger {
    max-width: 172px;
  }

  .avatar-dropdown-menu {
    right: -8px;
  }

  .avatar-menu-group,
  .avatar-task-tabs {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
