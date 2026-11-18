<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Download } from 'lucide-vue-next'
import http from '@/services/http'
import BaseButton from '@/components/base/BaseButton.vue'
import { resolveMediaAccess, mediaMessage, lanMediaBusy, lanMediaEnabled, lanMediaNotice, toggleLANMedia, type MediaAccess } from '@/services/mediaDelivery'

interface Preparation {
  request_id: string
  cancelled: boolean
  access_reference?: MediaAccess
  job: { job_id: string; resource_id: string; filename?: string; state: string; phase: string; processed_bytes: number; total_bytes: number; retryable: boolean }
}
const rows = ref<Preparation[]>([])
const notice = ref('')
const busy = ref(false)
const handedOff = ref(new Set<string>())
const disclosure = ref<HTMLDetailsElement | null>(null)
const open = ref(false)
function handleToggle() {
  open.value = disclosure.value?.open === true
  if (open.value) void refresh()
}
const trigger = ref<HTMLElement | null>(null)
let session = 0
const clearSession = () => { session += 1; rows.value = []; notice.value = ''; handedOff.value.clear() }

function closePanel(restoreFocus = false) {
  if (!disclosure.value?.open) return
  disclosure.value.open = false
  if (restoreFocus) trigger.value?.focus()
}

function onOutsidePointer(event: PointerEvent) {
  if (event.target instanceof Node && !disclosure.value?.contains(event.target)) closePanel()
}

onMounted(() => {
  window.addEventListener('asset-media-session-reset', clearSession)
  document.addEventListener('pointerdown', onOutsidePointer)
})
onBeforeUnmount(() => {
  window.removeEventListener('asset-media-session-reset', clearSession)
  document.removeEventListener('pointerdown', onOutsidePointer)
})

async function refresh() {
  if (busy.value) return
  busy.value = true
  const epoch = session
  try {
    const response = await http.get<{ data: Preparation[] }>('/v1/assets/media/requests')
    if (epoch !== session) return
    rows.value = (response.data.data || []).filter((row) => !row.cancelled && row.access_reference?.purpose === 'download')
    notice.value = ''
  } catch { notice.value = '准备记录暂不可用，请稍后刷新' }
  finally { busy.value = false }
}
function stateLabel(row: Preparation) {
  if (handedOff.value.has(row.request_id)) return '已交给浏览器，请在浏览器中查看进度'
  return ({ queued: '排队中', retry_wait: '等待重试', processing: '准备中', succeeded: '已就绪，点击下载', failed: '准备失败', stale: '文件版本已变化，请重新选择' } as Record<string,string>)[row.job.state] || row.job.state
}
async function download(row: Preparation) {
  if (!row.access_reference || busy.value) return
  busy.value = true
  const epoch = session
  try {
    const info = await resolveMediaAccess(row.access_reference)
    if (epoch !== session) return
    if (info.state !== 'ready' || !info.download_url) { notice.value = mediaMessage(info); return }
    const url = new URL(info.download_url, window.location.origin)
    if (!['https:', 'http:'].includes(url.protocol)) throw new Error('无效的下载地址')
    const link = document.createElement('a')
    link.href = url.href; link.download = info.filename; link.rel = 'noopener noreferrer'
    document.body.appendChild(link); link.click(); link.remove()
    handedOff.value.add(row.request_id)
    notice.value = '已交给浏览器；是否保存成功请查看浏览器下载列表'
  } catch (error) { notice.value = error instanceof Error ? error.message : '获取下载链接失败' }
  finally { busy.value = false }
}
async function change(row: Preparation, action: 'cancel' | 'retry') {
  if (busy.value) return
  busy.value = true
  try {
    if (action === 'cancel') await http.delete(`/v1/assets/media/requests/${encodeURIComponent(row.request_id)}`)
    else await http.post(`/v1/assets/media/jobs/${encodeURIComponent(row.job.job_id)}/retry`)
    busy.value = false
    await refresh()
  } catch (error) { notice.value = error instanceof Error ? error.message : '操作失败' }
  finally { busy.value = false }
}
</script>

<template>
  <details ref="disclosure" class="media-preparation" @toggle="handleToggle" @keydown.esc.stop.prevent="closePanel(true)">
    <summary ref="trigger" class="topbar-icon" aria-label="下载" title="下载">
      <Download :size="18" aria-hidden="true" />
    </summary>
    <div v-if="open" class="media-preparation__panel" role="region" aria-label="下载">
      <div class="media-preparation__header">
        <h2>下载</h2>
        <BaseButton size="sm" variant="secondary" :disabled="busy" @click="refresh">刷新状态</BaseButton>
      </div>
      <div class="media-preparation__mode">
        <span>下载方式 · {{ lanMediaEnabled ? '内网' : '云端' }}</span>
        <BaseButton size="sm" variant="secondary" :loading="lanMediaBusy" @click="toggleLANMedia">
          {{ lanMediaBusy ? '连接中…' : lanMediaEnabled ? '切换云端' : '切换内网' }}
        </BaseButton>
      </div>
      <p v-if="lanMediaNotice" class="media-preparation__note" role="status">{{ lanMediaNotice }}</p>
      <p v-if="notice" class="media-preparation__note" role="status">{{ notice }}</p>
      <p v-if="busy" class="media-preparation__note" role="status">正在查询下载记录…</p>
      <p v-else-if="!rows.length && !notice" class="media-preparation__note">暂无下载记录</p>
      <ul v-if="rows.length" class="media-preparation__jobs">
        <li v-for="row in rows" :key="row.request_id">
          <strong>{{ row.job.filename || row.job.resource_id }}</strong>
          <p>{{ stateLabel(row) }}</p>
          <p v-if="row.job.state === 'processing' && row.job.total_bytes > 0">{{ row.job.phase }}：{{ row.job.processed_bytes }} / {{ row.job.total_bytes }} 字节</p>
          <div class="media-preparation__actions">
            <BaseButton v-if="row.job.state === 'succeeded'" size="sm" :disabled="busy" @click="download(row)">获取链接并下载</BaseButton>
            <BaseButton v-if="row.job.state === 'failed' && row.job.retryable" size="sm" variant="secondary" :disabled="busy" @click="change(row, 'retry')">重试准备</BaseButton>
            <BaseButton size="sm" variant="ghost" :disabled="busy" @click="change(row, 'cancel')">取消等待</BaseButton>
          </div>
        </li>
      </ul>
    </div>
  </details>
</template>

<style scoped>
.media-preparation { position: relative; flex: 0 0 auto; }
.media-preparation summary { list-style: none; }
.media-preparation summary::-webkit-details-marker { display: none; }
.media-preparation__panel {
  position: absolute;
  z-index: 80;
  inset-block-start: calc(100% + 0.6rem);
  inset-inline-end: 0;
  inline-size: min(24rem, calc(100dvw - 1.5rem));
  max-block-size: min(70dvh, 36rem);
  overflow: auto;
  padding: 0.9rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.85rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 10px 28px rgb(var(--yb-shadow) / 0.14);
  color: rgb(var(--yb-text));
  font-size: 0.8125rem;
  line-height: 1.5;
}
.media-preparation__header,
.media-preparation__mode,
.media-preparation__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.media-preparation__header { margin-block-end: 0.6rem; }
.media-preparation__header h2 { font-size: 0.9375rem; font-weight: 700; }
.media-preparation__mode {
  padding: 0.45rem 0.55rem 0.45rem 0.75rem;
  border-radius: 0.6rem;
  background: rgb(var(--yb-surface-soft));
}
.media-preparation__note { margin: 0.6rem 0 0; color: rgb(var(--yb-text-muted)); }
.media-preparation__jobs { margin: 0.45rem 0 0; padding: 0; list-style: none; }
.media-preparation__jobs li { padding-block: 0.65rem; border-block-end: 1px solid rgb(var(--yb-border)); overflow-wrap: anywhere; }
.media-preparation__jobs li:last-child { border-block-end: 0; }
.media-preparation__actions { margin-block-start: 0.35rem; justify-content: flex-start; flex-wrap: wrap; }

@media (max-width: 640px) {
  .media-preparation__panel {
    position: fixed;
    inset-block-start: 4.4rem;
    inset-inline: 0.75rem;
    inline-size: auto;
  }
}
</style>
