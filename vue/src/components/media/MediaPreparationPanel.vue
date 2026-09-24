<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import http from '@/services/http'
import { resolveMediaAccess, mediaMessage, type MediaAccess } from '@/services/mediaDelivery'

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
let session = 0
const clearSession = () => { session += 1; rows.value = []; notice.value = ''; handedOff.value.clear() }
window.addEventListener('asset-media-session-reset', clearSession)
onBeforeUnmount(() => window.removeEventListener('asset-media-session-reset', clearSession))

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
  <details class="media-preparation" @toggle="($event.target as HTMLDetailsElement).open && refresh()">
    <summary>下载准备</summary>
    <div class="media-preparation__panel" role="region" aria-label="下载准备记录">
      <h2>下载准备记录</h2>
      <p>最近24小时及未完成任务。关闭页面不取消后台任务。</p>
      <button type="button" :disabled="busy" @click="refresh">刷新状态</button>
      <p v-if="notice" role="status">{{ notice }}</p>
      <p v-if="busy" role="status">正在查询准备记录…</p>
      <p v-else-if="!rows.length && !notice">暂无准备记录</p>
      <ul>
        <li v-for="row in rows" :key="row.request_id">
          <strong>{{ row.job.filename || row.job.resource_id }}</strong>
          <p>{{ stateLabel(row) }}</p>
          <p v-if="row.job.state === 'processing' && row.job.total_bytes > 0">{{ row.job.phase }}：{{ row.job.processed_bytes }} / {{ row.job.total_bytes }} 字节</p>
          <button v-if="row.job.state === 'succeeded'" type="button" :disabled="busy" @click="download(row)">获取链接并下载</button>
          <button v-if="row.job.state === 'failed' && row.job.retryable" type="button" :disabled="busy" @click="change(row, 'retry')">重试准备</button>
          <button type="button" :disabled="busy" @click="change(row, 'cancel')">取消我的等待</button>
        </li>
      </ul>
    </div>
  </details>
</template>

<style scoped>
.media-preparation { position: relative; font-size: .75rem; }
.media-preparation summary { cursor: pointer; }
.media-preparation__panel { position: absolute; right: 0; top: 2rem; z-index: 80; width: min(28rem, 85vw); max-height: 70vh; overflow: auto; padding: 1rem; border: 1px solid rgb(var(--yb-border)); border-radius: .75rem; background: rgb(var(--yb-surface)); color: rgb(var(--yb-text)); }
.media-preparation__panel li { padding: .75rem 0; border-bottom: 1px solid rgb(var(--yb-border)); }
.media-preparation__panel button { margin: .25rem .5rem .25rem 0; text-decoration: underline; }
</style>
