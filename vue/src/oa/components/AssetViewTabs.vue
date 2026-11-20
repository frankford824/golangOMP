<script setup lang="ts">
import DesktopSlot from './DesktopSlot.vue'

defineProps<{ ledger: boolean }>()
defineEmits<{ select: [ledger: boolean] }>()
</script>
<template>
  <DesktopSlot target="tabs">
    <nav class="asset-view-modes" aria-label="资产视图">
      <button type="button" :aria-pressed="!ledger" @click="$emit('select', false)">图库网格</button>
      <button type="button" :aria-pressed="ledger" @click="$emit('select', true)">规格台账</button>
    </nav>
  </DesktopSlot>
</template>
<style scoped>
.asset-view-modes { display:flex; align-self:stretch; align-items:stretch; }
.asset-view-modes button { min-height:32px; padding:0 8px; white-space:nowrap; border:0; border-bottom:2px solid transparent; background:transparent; }
.asset-view-modes button[aria-pressed=true] { color:var(--oa-accent); border-bottom-color:var(--oa-accent); font-weight:600; }
</style>
