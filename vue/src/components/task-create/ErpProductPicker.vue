<template>
  <div class="space-y-2">
    <input
      v-model="code"
      class="w-full rounded border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] px-2 py-1 text-sm"
      placeholder="输入 ERP 编码"
    />
    <button
      type="button"
      class="rounded bg-[var(--v1-bg-primary)] px-2 py-1 text-xs text-white"
      @click="handleSearch"
    >
      查询
    </button>
    <p v-if="searched && items.length === 0" class="text-xs text-[var(--v1-danger)]">未命中 ERP，禁止提交</p>
    <p v-for="item in items" :key="String(item.product_id)" class="text-xs text-[var(--v1-text-secondary)]">{{ item.product_name }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useErpProduct } from '@/composables/useErpProduct'

const emit = defineEmits<{ verified: [boolean] }>()
const code = ref('')
const searched = ref(false)
const { items, searchByCode } = useErpProduct()

async function handleSearch(): Promise<void> {
  searched.value = true
  await searchByCode(code.value)
  emit('verified', items.value.length > 0)
}
</script>
