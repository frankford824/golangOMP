<template>
  <NConfigProvider :theme-overrides="themeOverrides">
    <NMessageProvider>
      <router-view />
    </NMessageProvider>
  </NConfigProvider>
</template>

<script setup lang="ts">
import { NConfigProvider, NMessageProvider, type GlobalThemeOverrides } from 'naive-ui'

// Lock naive-ui's accent to the design-system brand blue so any naive surface
// (messages, future data tables) matches the rest of the app instead of
// naive's default green. Components that set their own theme-overrides
// (e.g. WorkflowProgress NSteps) keep their explicit colors.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: 'rgb(var(--yb-brand))',
    primaryColorHover: 'rgb(var(--yb-brand-strong))',
    primaryColorPressed: 'rgb(var(--yb-brand-deep))',
    primaryColorSuppl: 'rgb(var(--yb-brand))',
  },
}
</script>
