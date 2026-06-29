<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { NConfigProvider, NDialogProvider, NMessageProvider, type GlobalThemeOverrides } from 'naive-ui'

import MotionReveal from './shared/ui/MotionReveal.vue'
import ShellGate from './shell/ShellGate.vue'

const route = useRoute()
const isPublicRoute = computed(() => route.meta.public === true)

const themeOverrides: GlobalThemeOverrides = {
  common: {
    fontFamily: 'var(--aw-font-sans)',
    fontFamilyMono: 'var(--aw-font-mono)',
    primaryColor: 'var(--aw-accent)',
    primaryColorHover: 'var(--aw-accent-hover)',
    primaryColorPressed: 'var(--aw-accent-pressed)',
    textColorBase: 'var(--aw-ink)',
    borderColor: 'var(--aw-hairline)',
    borderRadius: 'var(--aw-radius-sm)',
    heightMedium: 'var(--aw-row-sm)',
  },
  Button: {
    borderRadiusMedium: 'var(--aw-radius-sm)',
    fontWeight: '600',
  },
  Card: {
    borderRadius: 'var(--aw-radius-md)',
  },
  DataTable: {
    thColor: 'var(--aw-surface-muted)',
    tdColor: 'var(--aw-surface)',
    borderColor: 'var(--aw-hairline)',
  },
}
</script>

<template>
  <NConfigProvider :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NDialogProvider>
        <div v-if="isPublicRoute" class="aw-root aw-root--auth">
          <RouterView v-slot="{ Component, route: activeRoute }">
            <MotionReveal :key="activeRoute.fullPath" class="aw-route-view">
              <component :is="Component" />
            </MotionReveal>
          </RouterView>
        </div>
        <ShellGate v-else />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
