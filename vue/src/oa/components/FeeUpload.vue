<script setup lang="ts">
import FeeEvidence from './FeeEvidence.vue'
import OaFileInput from './OaFileInput.vue'

type Evidence = { id: string; name: string }
defineProps<{ files: Evidence[]; error?: string; busy: boolean; progress: number; label: string; single?: boolean; disabled?: boolean }>()
const emit = defineEmits<{ upload: [files: File[]]; remove: [id: string] }>()
</script>

<template>
  <fieldset class="oa-fieldset fee-upload">
    <legend>{{ label }} *</legend>
    <OaFileInput
      accept=".pdf,.png,.jpg,.jpeg"
      :disabled="disabled || busy"
      :multiple="!single"
      paste
      :label="single ? '拖入或粘贴回单，或点击选择文件' : '拖入文件，粘贴截图，或点击选择文件'"
      :help="single ? '支持 PDF、PNG、JPEG，文件不得超过 10 MB。' : '支持 PDF、PNG、JPEG，每份不超过 10 MB，最多 15 份。'"
      @select="emit('upload', $event)"
    />
    <progress v-if="busy" :value="progress" max="100" :aria-label="label + '上传进度'" />
    <small v-if="error" class="fee-error" role="alert">{{ error }}</small>
    <FeeEvidence :ids="files.map(file => file.id)" :names="files.map(file => file.name)" :label="single ? '回单' : '凭证'">
      <template #actions="{ id }"><button type="button" :disabled="disabled || busy" @click="emit('remove', id)">移除</button></template>
    </FeeEvidence>
  </fieldset>
</template>

<style scoped>
.fee-upload { display: grid; gap: 8px; min-width: 0; }
progress { width: 100%; }
</style>
