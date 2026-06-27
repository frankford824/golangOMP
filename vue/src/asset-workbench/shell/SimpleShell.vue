<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { HandCoins, ImagePlus, UserRound } from 'lucide-vue-next'

import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

const props = defineProps<{
  bootstrap: AssetWorkbenchBootstrap | null
}>()

const route = useRoute()

const displayName = computed(() => props.bootstrap?.profile?.real_name || props.bootstrap?.user?.name || props.bootstrap?.user?.username || '我的账号')
const profileHint = computed(() => {
  if (!props.bootstrap?.profile) return '资料待填写'
  if (!props.bootstrap.profile.pii_completed) return '资料待补全'
  return '资料已完成'
})

const navItems = [
  { to: '/upload', label: '交作品', icon: ImagePlus },
  { to: '/my-settlement', label: '看收入', icon: HandCoins },
]
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
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="aw-simple-nav__item"
          :class="{ 'aw-simple-nav__item--active': route.path === item.to }"
        >
          <component :is="item.icon" :size="18" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="aw-simple-user" aria-label="我的资料">
        <UserRound :size="18" aria-hidden="true" />
        <span>
          <b>{{ displayName }}</b>
          <small>{{ profileHint }}</small>
        </span>
      </div>
    </header>

    <main class="aw-simple">
      <RouterView />
    </main>
  </div>
</template>
