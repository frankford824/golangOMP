<template>
  <div class="risk-card">
    <h3 class="risk-title">风险提示</h3>
    <div v-if="loading" class="risk-loading">
      <StatusSkeleton :loading="true" :lines="2" />
    </div>
    <div v-else-if="error" class="risk-error">
      <p>{{ error }}</p>
    </div>
    <div v-else-if="!items.length" class="risk-empty">
      <p>暂无风险项</p>
    </div>
    <template v-else>
      <ul class="risk-list">
        <li
          v-for="item in displayItems"
          :key="item.id"
          class="risk-item"
          :class="[item.level, { navigable: isNavigable(item) }]"
          @click="onSelect(item)"
        >
          <span class="risk-dot" />
          <span class="risk-message">{{ item.message }}</span>
          <span
            v-if="item.refNo"
            class="risk-ref"
            :class="{ 'risk-ref--link': isNavigable(item) }"
          >{{ item.refNo }}</span>
        </li>
      </ul>
      <div v-if="showToggle" class="risk-footer">
        <BaseButton
          type="button"
          class="risk-toggle"
          variant="ghost"
          size="sm"
          @click.stop="toggleExpanded"
        >
          {{ expanded ? '收起' : '查看更多' }}
        </BaseButton>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import type { RiskItem } from '@/types/dashboard'
import StatusSkeleton from '@/components/common/StatusSkeleton.vue'
import { useRouter } from 'vue-router'

const props = defineProps<{
  items: RiskItem[]
  loading?: boolean
  error?: string
}>()

const RISK_LIST_LIMIT = 6

const emit = defineEmits<{ select: [item: RiskItem] }>()
const router = useRouter()

const expanded = ref(false)

const showToggle = computed(() => props.items.length > RISK_LIST_LIMIT)

const displayItems = computed(() => {
  if (expanded.value || props.items.length <= RISK_LIST_LIMIT) {
    return props.items
  }
  return props.items.slice(0, RISK_LIST_LIMIT)
})

watch(
  () => props.items,
  () => {
    expanded.value = false
  },
)

function isNavigable(item: RiskItem): boolean {
  if (item.route) return true
  if (item.refId) return true
  return false
}

function toggleExpanded() {
  expanded.value = !expanded.value
}

function onSelect(item: RiskItem) {
  emit('select', item)
  if (!isNavigable(item)) return
  if (item.route) {
    void router.push(item.route)
  } else if (item.refId) {
    void router.push(`/tasks/${item.refId}`)
  }
}
</script>

<style scoped>
.risk-card {
  display: flex;
  flex-direction: column;
  background: #fff;
  border: 1px solid rgb(226 232 240 / 0.95);
  border-radius: 0.75rem;
  padding: 1.25rem;
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.05),
    0 0 0 1px rgba(15, 23, 42, 0.04);
}
.risk-title {
  margin: 0 0 0.75rem;
  font-size: 0.9375rem;
  font-weight: 600;
  color: #0f172a;
}
.risk-loading,
.risk-error,
.risk-empty {
  padding: 0.5rem 0;
  font-size: 0.875rem;
  color: #64748b;
}
.risk-list {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: 1;
  min-width: 0;
}
.risk-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0;
  font-size: 0.8125rem;
}
.risk-item.navigable {
  cursor: pointer;
}
.risk-item.navigable:hover {
  background: #f8fafc;
}
.risk-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.risk-item.high .risk-dot {
  background: #dc2626;
}
.risk-item.medium .risk-dot {
  background: #f59e0b;
}
.risk-item.low .risk-dot {
  background: #10b981;
}
.risk-message {
  flex: 1;
  color: #0f172a;
}
.risk-ref {
  font-size: 0.75rem;
  color: #64748b;
}
.risk-ref--link {
  color: rgb(16 185 129);
}
.risk-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.5rem;
  padding-top: 0.25rem;
}
.risk-toggle {
  margin: 0;
}
</style>
