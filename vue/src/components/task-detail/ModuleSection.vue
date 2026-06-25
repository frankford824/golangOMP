<template>
  <section v-if="visible" class="rounded-xl border border-[var(--v1-border)] bg-[rgb(var(--yb-surface))] p-4">
    <div class="flex items-start justify-between gap-3">
      <div>
        <p class="text-xs text-[var(--v1-text-secondary)]">{{ eyebrow }}</p>
        <h2 class="text-sm font-semibold text-[var(--v1-text-primary)]">{{ title }}</h2>
      </div>
      <span class="rounded-full bg-[var(--v1-bg-surface-soft)] px-2 py-0.5 text-xs text-[var(--v1-text-secondary)]">
        {{ stateLabel }}
      </span>
    </div>
    <slot :readonly="readonly" :actions="actions" />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ModuleSummary } from '@/services/apiTypes'
import { getModuleAllowedActions } from '@/domain/module-actions'

const props = defineProps<{
  module?: ModuleSummary
  title: string
  eyebrow?: string
}>()

const visible = computed(() => props.module?.scope?.deny_code !== 'module_not_instantiated')
const readonly = computed(() => props.module?.scope?.in_scope === false)
const actions = computed(() => getModuleAllowedActions(props.module))
const stateLabel = computed(() => props.module?.state ?? 'not_started')
</script>
