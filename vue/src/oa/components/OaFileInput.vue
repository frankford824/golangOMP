<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, useId } from 'vue'

const props = defineProps<{ accept: string; label: string; disabled?: boolean; multiple?: boolean; paste?: boolean; help?: string }>()
const emit = defineEmits<{ select: [files: File[]] }>()
const picker = ref<HTMLInputElement>()
const helpID = useId()

function emitFiles(files: File[]) {
  if (props.disabled || !files.length) return
  emit('select', props.multiple ? files : files.slice(0, 1))
}

function pick() {
  emitFiles(Array.from(picker.value?.files ?? []))
  if (picker.value) picker.value.value = ''
}

function drop(event: DragEvent) {
  emitFiles(Array.from(event.dataTransfer?.files ?? []))
}

function paste(event: ClipboardEvent) {
  if (!props.paste || props.disabled || event.defaultPrevented) return
  const files = Array.from(event.clipboardData?.files ?? [])
  if (!files.length) return
  event.preventDefault()
  emitFiles(files)
}

onMounted(() => window.addEventListener('paste', paste))
onBeforeUnmount(() => window.removeEventListener('paste', paste))
</script>

<template>
  <div class="oa-file-control">
    <input ref="picker" type="file" hidden :accept="accept" :multiple="multiple" :disabled="disabled" @change="pick" />
    <button
      type="button"
      class="oa-file-drop"
      :disabled="disabled"
      :aria-describedby="help ? helpID : undefined"
      @click="picker?.click()"
      @dragover.prevent
      @drop.prevent="drop"
    >
      {{ label }}
    </button>
    <small v-if="help" :id="helpID" class="oa-muted">{{ help }}</small>
  </div>
</template>

<style scoped>
.oa-file-control { display: grid; gap: 6px; min-width: 0; }
.oa-file-control .oa-file-drop { width: 100%; min-height: 64px; border-style: dashed; }
</style>
