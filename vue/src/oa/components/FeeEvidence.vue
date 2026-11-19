<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import http from '@/services/http'
import { download, errorText } from '../api'

const props = defineProps<{ ids: string[]; label: string; names?: string[] }>()
const index = ref(-1)
const src = ref('')
const mime = ref('')
const filename = ref('')
const busy = ref(false)
const error = ref('')
const downloadError = ref('')
const scale = ref(1)
const angle = ref(0)
let controller: AbortController | undefined
function reset() {
  controller?.abort()
  if (src.value) URL.revokeObjectURL(src.value)
  src.value = ''; filename.value = ''; mime.value = ''; error.value = ''; busy.value = false
  scale.value = 1; angle.value = 0
}
function close() { index.value = -1; reset() }
async function preview(i: number) {
  if (!props.ids[i]) return
  reset()
  index.value = i
  const request = new AbortController()
  controller = request
  busy.value = true
  try {
    const response = await http.get<Blob>('/oa-api/finance/files/' + encodeURIComponent(props.ids[i]) + '?inline=1', { responseType: 'blob', signal: request.signal })
    if (request.signal.aborted) return
    const type = response.data.type.split(';')[0]
    if (!['application/pdf','image/png','image/jpeg'].includes(type)) throw new Error('该格式暂不支持预览')
    mime.value = type
    const disposition = String(response.headers['content-disposition'] || '')
    const encoded = /filename\*=UTF-8''([^;]+)/i.exec(disposition)?.[1]
    filename.value = encoded ? decodeURIComponent(encoded) : /filename="?([^";]+)/i.exec(disposition)?.[1] || ''
    src.value = URL.createObjectURL(response.data)
  } catch (cause) { if (!request.signal.aborted) error.value = errorText(cause) }
  finally { if (!request.signal.aborted) busy.value = false }
}
async function original(i: number) {
  downloadError.value = ''
  try { await download('/oa-api/finance/files/' + encodeURIComponent(props.ids[i]), props.label + '-' + (i + 1)) }
  catch (cause) { downloadError.value = errorText(cause) }
}
watch(() => props.ids.join(','), () => { close(); downloadError.value = '' })
onBeforeUnmount(reset)
</script>
<template>
  <section v-if="ids.length" class="fee-evidence" :aria-label="label">
    <h3>{{ label }}（{{ ids.length }}）</h3>
    <div v-for="(id, i) in ids" :key="id" class="file-line">
      <span>{{ names?.[i] || label + " " + (i + 1) }}</span><button type="button" @click="preview(i)">预览{{ label }}</button>
      <button type="button" @click="original(i)">下载{{ label }}</button><slot name="actions" :id="id" />
    </div>
    <p v-if="downloadError" role="alert">{{ downloadError }}</p>
    <BaseModal :model-value="index >= 0" :title="filename || label + '预览'" panel-max-width="1024px" @cancel="close">
      <p v-if="busy" role="status">正在读取{{ label }}…</p>
      <p v-else-if="error" role="alert">{{ error }} <button type="button" @click="preview(index)">重试预览</button></p>
      <object v-else-if="src && mime === 'application/pdf'" :data="src" type="application/pdf" class="pdf-frame" :aria-label="label + ' PDF'">浏览器无法预览 PDF，可下载原件查看。</object>
      <div v-else-if="src" class="image-viewer"><img :src="src" :alt="filename || label" :style="{ width: scale * 100 + '%', transform: 'rotate(' + angle + 'deg)' }" /></div>
      <template #footer><footer class="viewer-tools">
        <button v-if="ids.length > 1" :disabled="index <= 0" @click="preview(index-1)">上一份</button>
        <button v-if="ids.length > 1" :disabled="index >= ids.length-1" @click="preview(index+1)">下一份</button>
        <template v-if="src && mime.startsWith('image/')"><button :disabled="scale <= 0.5" @click="scale-=0.25">缩小</button><button :disabled="scale >= 3" @click="scale+=0.25">放大</button><button @click="angle=(angle+90)%360">旋转 90°</button></template>
        <button @click="original(index)">下载原件</button><button @click="close">关闭预览</button>
      </footer></template>
    </BaseModal>
  </section>
</template>
<style scoped>
.fee-evidence { margin-block:12px; }
.file-line,.viewer-tools { display:flex; align-items:center; flex-wrap:wrap; gap:8px; padding:6px 0; }
.file-line span { margin-inline-end:auto; }
.pdf-frame,.image-viewer { width:100%; height:min(64dvh,720px); border:0; }
.image-viewer { overflow:auto; background:rgb(var(--yb-surface-muted)); }
.image-viewer img { display:block; height:auto; max-width:none; transform-origin:center; }
.viewer-tools { justify-content:flex-end; padding:12px 20px; }
.viewer-tools button,.file-line button { min-height:32px; padding:4px 8px; border:1px solid var(--yb-line-control); border-radius:6px; }
</style>
