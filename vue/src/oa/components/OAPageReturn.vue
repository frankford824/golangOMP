<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'
import type { RouteLocationRaw } from 'vue-router'
import DesktopSlot from './DesktopSlot.vue'
import OAReturnLink from './OAReturnLink.vue'

defineProps<{ to?: RouteLocationRaw; source?: string; disabled?: boolean; inline?: boolean }>()
defineEmits<{ activate: [] }>()
</script>

<template>
  <DesktopSlot target="back" :inline="inline">
    <OAReturnLink v-if="to" class="oa-page-return" data-oa-page-return :to="to" :source="source">
      <ArrowLeft :size="16" aria-hidden="true" />
      <slot />
    </OAReturnLink>
    <button v-else class="oa-page-return" data-oa-page-return type="button" :disabled="disabled" @click="$emit('activate')">
      <ArrowLeft :size="16" aria-hidden="true" />
      <slot />
    </button>
  </DesktopSlot>
</template>

<style>
#app .oa-page-return[data-oa-page-return] {
  box-sizing: border-box;
  display: inline-flex;
  width: max-content;
  max-width: 100%;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--yb-line-control);
  border-radius: 6px;
  background: rgb(var(--yb-surface));
  color: var(--oa-ink);
  font: 500 13px/1 var(--oa-font);
  cursor: pointer;
  text-decoration: none;
  white-space: nowrap;
}
#app .oa-page-return[data-oa-page-return]:disabled { cursor: not-allowed; }
#app .oa-page-return[data-oa-page-return]:not(:disabled):hover { color: var(--oa-accent); border-color: var(--oa-accent); }
#app .oa-page-return[data-oa-page-return]:focus-visible { outline: 2px solid var(--oa-accent); outline-offset: 2px; }
</style>
