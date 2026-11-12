<template>
  <div v-if="selectedGroup" class="default-style-workspace">
    <label v-if="groups.length > 1" class="group-select">
      <span>{{ selectedGroup.scope_kind === 'retouch_requirement' ? '选择修图需求' : '选择 SKU' }}</span>
      <select v-model.number="selectedGroupId" :disabled="busy || dirty">
        <option v-for="(group, index) in groups" :key="group.id" :value="group.id">
          {{ groupLabel(group, index) }}{{ group.pending_default_style ? ' · 待生效' : group.default_style?.file ? ' · 已上传' : '' }}
        </option>
      </select>
      <small v-if="dirty && !busy">保存或取消后可切换。</small>
    </label>
    <p v-else-if="selectedGroup.sku_code" class="single-group">{{ selectedGroup.sku_code }}</p>
    <DefaultStyleSlot :key="selectedGroup.id" :group="selectedGroup"
      :editable="editable && !selectedGroup.migration_incomplete"
      :history-enabled="historyEnabled"
      @updated="$emit('updated', $event)" @busy="setBusy" @dirty-change="setDirty" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import DefaultStyleSlot from '@/components/task/DefaultStyleSlot.vue'
import type { ResourceGroup } from '@/services/api/resourceGroupsApi'

const props = defineProps<{ groups: ResourceGroup[]; editable?: boolean; historyEnabled?: boolean }>()
const emit = defineEmits<{ updated: [group: ResourceGroup]; busy: [active: boolean]; dirtyChange: [dirty: boolean] }>()
const selectedGroupId = ref(props.groups[0]?.id ?? 0)
const selectedGroup = computed(() => props.groups.find(group => group.id === selectedGroupId.value) ?? props.groups[0])
const busy = ref(false)
const dirty = ref(false)
watch(selectedGroup, group => { selectedGroupId.value = group?.id ?? 0 })

function setBusy(active: boolean) {
  busy.value = active
  emit('busy', active)
}

function setDirty(active: boolean) {
  dirty.value = active
  emit('dirtyChange', active)
}

function groupLabel(group: ResourceGroup, index: number) {
  if (group.scope_kind === 'retouch_requirement') return group.sku_code ? `${group.sku_code} · 修图需求 ${index + 1}` : `修图需求 ${index + 1}`
  return group.sku_code || `资源组 ${index + 1}`
}
</script>

<style scoped>
.default-style-workspace { display: grid; gap: 10px; min-width: 0; }
.group-select { display: grid; gap: 4px; color: rgb(var(--yb-text)); font-size: 13px; font-weight: 650; }
.group-select select {
  min-width: 0;
  min-height: 36px;
  padding: 6px 9px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 8px;
  background: rgb(var(--yb-surface));
  color: inherit;
}
.group-select small, .single-group { margin: 0; color: rgb(var(--yb-text-muted)); font-size: 12px; }
</style>
