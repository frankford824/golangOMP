<template>
  <a
    v-if="url"
    class="file-fallback"
    :href="url"
    target="_blank"
    rel="noopener noreferrer"
    :title="name"
  >
    <span class="file-ext">{{ extLabel }}</span>
    <span class="file-name">{{ name }}</span>
    <span v-if="sizeText" class="file-size">{{ sizeText }}</span>
  </a>
  <div v-else class="file-fallback" :title="name">
    <span class="file-ext">{{ extLabel }}</span>
    <span class="file-name">{{ name }}</span>
    <span v-if="sizeText" class="file-size">{{ sizeText }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getUploadFileExtension } from '@/domain/constants/upload-types'

const props = withDefaults(
  defineProps<{
    name: string
    url?: string | null
    sizeText?: string
  }>(),
  {
    url: null,
    sizeText: '',
  },
)

const extLabel = computed(() => {
  const ext = getUploadFileExtension(props.name)
  return ext ? ext.toUpperCase() : 'FILE'
})
</script>

<style scoped>
.file-fallback {
  width: 100%;
  height: 100%;
  border-radius: 4px;
  border: 1px solid rgb(var(--yb-brand-border));
  background: rgb(var(--yb-surface));
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.125rem;
  text-align: center;
  padding: 0.25rem;
  text-decoration: none;
}

.file-ext {
  font-size: 0.625rem;
  font-weight: 700;
  color: rgb(var(--yb-brand-strong));
}

.file-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.5rem;
  line-height: 1.1;
  color: rgb(var(--yb-text-soft));
}

.file-size {
  font-size: 0.5rem;
  line-height: 1.1;
  color: rgb(var(--yb-text-muted-strong));
  font-variant-numeric: tabular-nums;
}
</style>
