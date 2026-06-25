<template>
  <div class="space-y-2">
    <input
      v-model="keyword"
      class="w-full rounded border border-[var(--v1-border)] bg-[var(--v1-bg-surface-soft)] px-2 py-1 text-sm"
      placeholder="输入设计源关键字"
    />
    <button
      type="button"
      class="rounded bg-[var(--v1-bg-primary)] px-2 py-1 text-xs text-[rgb(var(--yb-text-inverse))]"
      @click="handleSearch"
    >
      查询
    </button>
    <p v-if="searched && items.length === 0" class="text-xs text-[var(--v1-danger)]">未命中设计源文件，禁止提交</p>
    <p v-for="item in items" :key="String(item.id)" class="text-xs text-[var(--v1-text-secondary)]">{{ item.file_name }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useDesignSourceSearch } from '@/composables/useDesignSourceSearch'

const emit = defineEmits<{ verified: [boolean] }>()
const keyword = ref('')
const searched = ref(false)
const { items, search } = useDesignSourceSearch()

async function handleSearch(): Promise<void> {
  searched.value = true
  await search(keyword.value)
  emit('verified', items.value.length > 0)
}
</script>
