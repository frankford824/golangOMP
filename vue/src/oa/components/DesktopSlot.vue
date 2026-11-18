<script setup lang="ts">
import { computed, inject, onActivated, onDeactivated, ref } from 'vue'
import { desktopChromeKey } from '../desktopChrome'

const props = defineProps<{ target: 'tabs' | 'status' | 'actions' }>()
const chrome = inject(desktopChromeKey, null)
const active = ref(true)
onActivated(() => { active.value = true })
onDeactivated(() => { active.value = false })
const host = computed(() => chrome?.[props.target].value)
const inBar = computed(() => Boolean(active.value && chrome?.desktop.value && host.value))
</script>

<template>
  <Teleport :to="host || 'body'" :disabled="!inBar"><slot /></Teleport>
</template>
