<script setup lang="ts">
import { computed, inject, onActivated, onDeactivated, ref } from 'vue'
import { desktopChromeKey, workspaceActiveKey } from '../desktopChrome'

const props = defineProps<{ target: 'back' | 'tabs' | 'status' | 'actions'; inline?: boolean }>()
const chrome = inject(desktopChromeKey, null)
const workspaceActive = inject(workspaceActiveKey, null)
const active = ref(true)
onActivated(() => { active.value = true })
onDeactivated(() => { active.value = false })
const host = computed(() => chrome?.[props.target].value)
const inBar = computed(() => Boolean(!props.inline && active.value && (workspaceActive?.value ?? true) && host.value && (props.target === 'back' || chrome?.desktop.value)))
</script>

<template>
  <Teleport :to="host || 'body'" :disabled="!inBar"><slot /></Teleport>
</template>
