<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { Bell, HandCoins, HardDrive, ImagePlus, Table2, UserRound } from 'lucide-vue-next'

import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'
import { canAccessPath } from '@aw/app/access'
import { workbenchProfileStatusMeta } from '@aw/shared/format/status'
import GlobalDownloadCenter from '../shared/download/GlobalDownloadCenter.vue'
import GlobalUploadCenter from '../shared/drive/GlobalUploadCenter.vue'
import MotionReveal from '../shared/ui/MotionReveal.vue'

const props = defineProps<{
  bootstrap: AssetWorkbenchBootstrap | null
}>()

const route = useRoute()

const displayName = computed(() => props.bootstrap?.profile?.real_name || props.bootstrap?.user?.name || props.bootstrap?.user?.username || '我的账号')
const profileHint = computed(() => workbenchProfileStatusMeta(props.bootstrap?.profile).label)

const navItems = [
  {
    to: '/drive',
    label: '我的网盘',
    icon: HardDrive,
    capabilities: ['asset.workbench.submit', 'asset.workbench.material.download', 'asset.workbench.system_search'],
  },
  { to: '/upload', label: '上传成品', icon: ImagePlus, capabilities: ['asset.workbench.submit'] },
  { to: '/upload-overview', label: '上传台账', icon: Table2, capabilities: ['asset.workbench.submit'] },
  { to: '/my-settlement', label: '看收入', icon: HandCoins },
]
const visibleNavItems = computed(() => {
  const capabilities = new Set(props.bootstrap?.capabilities ?? [])
  return navItems.filter((item) => canAccessPath(props.bootstrap, item.to) && (!item.capabilities || item.capabilities.some((capability) => capabilities.has(capability))))
})
</script>

<template>
  <div class="aw-root aw-root--simple">
    <header class="aw-simple-topbar">
      <RouterLink class="aw-simple-brand" to="/">
        <span class="aw-shell__mark">AW</span>
        <span>作品工作台</span>
      </RouterLink>
      <nav class="aw-simple-nav" aria-label="常用入口">
        <RouterLink
          v-for="item in visibleNavItems"
          :key="item.to"
          :to="item.to"
          class="aw-simple-nav__item"
          :class="{ 'aw-simple-nav__item--active': route.path === item.to }"
        >
          <component :is="item.icon" :size="18" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="aw-simple-account-actions">
        <RouterLink
          class="aw-secondary-button aw-simple-messages"
          to="/notifications"
          aria-label="消息"
          :aria-current="route.path === '/notifications' ? 'page' : undefined"
          title="消息"
        >
          <Bell :size="18" aria-hidden="true" />
          <span>消息</span>
        </RouterLink>
        <RouterLink class="aw-simple-user" to="/account" aria-label="我的资料">
          <UserRound :size="18" aria-hidden="true" />
          <span>
            <b>{{ displayName }}</b>
            <small>{{ profileHint }}</small>
          </span>
        </RouterLink>
      </div>
    </header>

    <main class="aw-simple">
      <RouterView v-slot="{ Component, route: activeRoute }">
        <KeepAlive>
          <component v-if="activeRoute.meta.keepAlive" :is="Component" class="aw-route-view" />
        </KeepAlive>
        <MotionReveal v-if="!activeRoute.meta.keepAlive" :key="activeRoute.path" class="aw-route-view">
          <component :is="Component" />
        </MotionReveal>
      </RouterView>
    </main>

    <GlobalUploadCenter />
    <GlobalDownloadCenter />
  </div>
</template>
