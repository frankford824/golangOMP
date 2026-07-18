<template>
  <NConfigProvider :theme-overrides="themeOverrides">
    <NMessageProvider>
      <router-view />
    </NMessageProvider>
  </NConfigProvider>
</template>

<script setup lang="ts">
import { NConfigProvider, NMessageProvider, type GlobalThemeOverrides } from 'naive-ui'
import { resolveCssRgbToken } from '@/utils/color-tokens'

// Lock naive-ui's accent to the design-system brand blue so any naive surface
// (messages, future data tables) matches the rest of the app instead of
// naive's default green. Components that set their own theme-overrides
// (e.g. WorkflowProgress NSteps) keep their explicit colors.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: resolveCssRgbToken('--yb-brand', '37 99 235'),
    primaryColorHover: resolveCssRgbToken('--yb-brand-strong', '29 78 216'),
    primaryColorPressed: resolveCssRgbToken('--yb-brand-deep', '30 64 175'),
    primaryColorSuppl: resolveCssRgbToken('--yb-brand', '37 99 235'),
  },
}
</script>
