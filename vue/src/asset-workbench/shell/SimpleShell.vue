<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { HandCoins, HardDrive, ImagePlus, LayoutDashboard, Table2 } from 'lucide-vue-next'

import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'
import { canAccessPath } from '@aw/app/access'
import { workbenchProfileStatusMeta } from '@aw/shared/format/status'
import GlobalDownloadCenter from '../shared/download/GlobalDownloadCenter.vue'
import GlobalUploadCenter from '../shared/drive/GlobalUploadCenter.vue'
import MotionReveal from '../shared/ui/MotionReveal.vue'
import WorkbenchHeaderActions from './WorkbenchHeaderActions.vue'

const props = defineProps<{
  bootstrap: AssetWorkbenchBootstrap | null
}>()

const route = useRoute()

const displayName = computed(() => props.bootstrap?.profile?.real_name || props.bootstrap?.user?.name || props.bootstrap?.user?.username || '我的账号')
const profileHint = computed(() => workbenchProfileStatusMeta(props.bootstrap?.profile).label)

const navItems = [
  { to: '/', label: '今日待办', icon: LayoutDashboard },
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
      <nav class="aw-simple-nav" aria-label="工作模块">
        <RouterLink
          v-for="item in visibleNavItems"
          :key="item.to"
          :to="item.to"
          class="aw-simple-nav__item"
          :aria-current="route.path === item.to ? 'page' : undefined"
        >
          <component :is="item.icon" :size="18" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <WorkbenchHeaderActions class="aw-simple-account-actions" :display-name="displayName" :profile-hint="profileHint" />
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
