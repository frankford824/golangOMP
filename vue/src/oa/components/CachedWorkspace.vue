<script setup lang="ts">
import { onActivated, onDeactivated, provide, ref } from 'vue'
import { workspaceActiveKey } from '../desktopChrome'

// Children created while this view is cached must inherit its inactive state.
const active = ref(true)
provide(workspaceActiveKey, active)
onActivated(() => { active.value = true })
onDeactivated(() => { active.value = false })
</script>

<template><slot /></template>
