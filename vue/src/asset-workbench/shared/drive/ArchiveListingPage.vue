<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ArchiveVirtualFile, ArchiveVirtualFolder } from '@aw/shared/api/assetWorkbenchApi'

const props = defineProps<{ folders: ArchiveVirtualFolder[]; files: ArchiveVirtualFile[] }>()
const pageSize = 60
const page = ref(1)
const pages = computed(() => Math.max(1, Math.ceil((props.folders.length + props.files.length) / pageSize)))
const start = computed(() => (Math.min(page.value, pages.value) - 1) * pageSize)
const visibleFolders = computed(() => props.folders.slice(start.value, start.value + pageSize))
const visibleFiles = computed(() => {
  const offset = Math.max(0, start.value - props.folders.length)
  return props.files.slice(offset, offset + pageSize - visibleFolders.value.length)
})
</script>

<template>
  <slot :folders="visibleFolders" :files="visibleFiles" />
  <nav v-if="pages > 1" class="aw-inline-actions" aria-label="压缩包目录分页">
    <button class="aw-grid-button" type="button" :disabled="page <= 1" @click="page--">上一页</button>
    <span>{{ page }} / {{ pages }}</span>
    <button class="aw-grid-button" type="button" :disabled="page >= pages" @click="page++">下一页</button>
  </nav>
</template>
