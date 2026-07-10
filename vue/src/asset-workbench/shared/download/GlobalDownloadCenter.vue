<script setup lang="ts">
import { computed, watch } from 'vue'
import { useAutoAnimate } from '@formkit/auto-animate/vue'
import {
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  CircleStop,
  Download,
  ExternalLink,
  LoaderCircle,
  RotateCcw,
  X,
} from 'lucide-vue-next'

import { formatFileSize, formatInt } from '@aw/shared/format/number'
import { useUploadCenterStore } from '@aw/shared/drive/uploadCenter.store'
import { useDownloadCenterStore, type DownloadCenterItem, type DownloadCenterStatus } from './downloadCenter.store'

const downloadCenter = useDownloadCenterStore()
const uploadCenter = useUploadCenterStore()
const [listRef] = useAutoAnimate({ duration: 180, easing: 'ease-out' })

const shellVisible = computed(() => downloadCenter.hasItems)
const ringStyle = computed(() => ({
  '--aw-upload-center-progress': Math.max(downloadCenter.overallProgress, downloadCenter.hasActive ? 8 : 100) + '%',
}))
const panelTitle = computed(() => {
  if (downloadCenter.hasActive) return '文件正在下载'
  if (downloadCenter.failedItems.length) return '有下载需要处理'
  if (downloadCenter.handedOffItems.length) return '请查看浏览器下载列表'
  return '下载已完成'
})

watch(
  () => downloadCenter.panelOpen,
  (open) => {
    if (open) uploadCenter.closePanel()
  },
)

function togglePanel() {
  if (downloadCenter.panelOpen) downloadCenter.closePanel()
  else downloadCenter.openPanel()
}

function statusLabel(status: DownloadCenterStatus) {
  const labels: Record<DownloadCenterStatus, string> = {
    queued: '等待下载',
    preparing: '正在准备',
    downloading: '下载中',
    completed: '已完成',
    handed_off: '浏览器下载',
    failed: '下载失败',
    cancelled: '已取消',
  }
  return labels[status]
}

function statusIcon(status: DownloadCenterStatus) {
  if (status === 'failed') return AlertCircle
  if (status === 'completed') return CheckCircle2
  if (status === 'handed_off') return ExternalLink
  if (status === 'preparing' || status === 'downloading') return LoaderCircle
  return Download
}

function transferText(item: DownloadCenterItem) {
  if (item.status === 'preparing') return '正在准备文件，可继续使用其他页面'
  if (item.status === 'queued') return '等待前面的下载任务'
  if (item.status === 'handed_off') return '已交给浏览器，请在浏览器下载列表查看进度'
  if (item.status === 'cancelled') return item.receivedBytes > 0 ? '已下载 ' + formatFileSize(item.receivedBytes) + ' 后取消' : '下载已取消'
  if (item.status === 'failed') return item.receivedBytes > 0 ? '已下载 ' + formatFileSize(item.receivedBytes) + ' 后中断' : '文件未下载，可直接重试'
  if (item.status === 'completed') return formatFileSize(item.totalBytes || item.receivedBytes) + ' · 已保存到本机'
  if (item.totalBytes > 0) return formatFileSize(item.receivedBytes) + ' / ' + formatFileSize(item.totalBytes)
  return '已下载 ' + formatFileSize(item.receivedBytes)
}

function speedText(item: DownloadCenterItem) {
  if (item.status !== 'downloading' || item.speedBytesPerSecond <= 0) return ''
  return formatFileSize(item.speedBytesPerSecond) + '/秒'
}

function etaText(item: DownloadCenterItem) {
  if (item.status !== 'downloading' || item.totalBytes <= item.receivedBytes || item.speedBytesPerSecond <= 0) return ''
  const seconds = Math.ceil((item.totalBytes - item.receivedBytes) / item.speedBytesPerSecond)
  if (seconds < 60) return '约 ' + seconds + ' 秒'
  if (seconds < 3600) return '约 ' + Math.ceil(seconds / 60) + ' 分钟'
  return '约 ' + Math.ceil(seconds / 3600) + ' 小时'
}

function canCancel(item: DownloadCenterItem) {
  return item.status === 'queued' || item.status === 'preparing' || item.status === 'downloading'
}

function canRetry(item: DownloadCenterItem) {
  return item.status === 'failed' || item.status === 'cancelled'
}
</script>

<template>
  <Teleport to="body">
    <Transition name="aw-upload-center-shell">
      <section
        v-if="shellVisible"
        class="aw-token-scope aw-upload-center aw-download-center"
        :class="{ 'has-upload-center': uploadCenter.hasItems, 'is-open': downloadCenter.panelOpen }"
        aria-label="全局下载中心"
      >
        <button
          class="aw-upload-center__orb"
          type="button"
          :style="ringStyle"
          :aria-expanded="downloadCenter.panelOpen"
          aria-controls="aw-download-center-panel"
          @click="togglePanel"
        >
          <span class="aw-upload-center__ring" aria-hidden="true">
            <Download :size="20" />
          </span>
          <span class="aw-upload-center__orb-copy">
            <strong>{{ downloadCenter.hasActive ? formatInt(downloadCenter.overallProgress) + '%' : '下载' }}</strong>
            <small>{{ downloadCenter.summaryText }}</small>
          </span>
        </button>

        <Transition name="aw-upload-center-panel">
          <div v-if="downloadCenter.panelOpen" id="aw-download-center-panel" class="aw-upload-center__panel">
            <header class="aw-upload-center__head">
              <div>
                <p class="aw-eyebrow">下载中心</p>
                <h3>{{ panelTitle }}</h3>
                <span>{{ downloadCenter.summaryText }}，切换页面不会中断</span>
              </div>
              <button class="aw-icon-action" type="button" aria-label="收起下载中心" title="收起" @click="downloadCenter.closePanel()">
                <ChevronDown :size="16" aria-hidden="true" />
              </button>
            </header>

            <div ref="listRef" class="aw-upload-center__list">
              <article
                v-for="item in downloadCenter.visibleItems"
                :key="item.id"
                class="aw-upload-center__item aw-download-center__item"
                :class="'is-' + item.status"
              >
                <component :is="statusIcon(item.status)" :size="18" aria-hidden="true" />
                <div class="aw-upload-center__item-main">
                  <div class="aw-upload-center__item-title">
                    <strong :title="item.displayName">{{ item.displayName }}</strong>
                    <span>{{ statusLabel(item.status) }}</span>
                  </div>
                  <small>{{ item.sourceLabel }} · {{ transferText(item) }}</small>
                  <div v-if="item.status === 'downloading' || item.status === 'preparing' || item.status === 'queued'" class="aw-download-center__metrics">
                    <span>{{ item.status === 'downloading' ? formatInt(item.progress) + '%' : statusLabel(item.status) }}</span>
                    <span v-if="speedText(item)">{{ speedText(item) }}</span>
                    <span v-if="etaText(item)">{{ etaText(item) }}</span>
                  </div>
                  <div
                    v-if="item.status !== 'handed_off' && item.status !== 'cancelled'"
                    class="aw-upload-center__bar"
                    :class="{ 'is-indeterminate': item.status === 'preparing' }"
                    :aria-label="statusLabel(item.status) + ' ' + item.progress + '%'"
                  >
                    <span :style="{ width: item.status === 'preparing' ? undefined : item.progress + '%' }" />
                  </div>
                  <p v-if="item.error" class="aw-upload-center__error">{{ item.error }}</p>
                </div>
                <div class="aw-download-center__actions">
                  <button
                    v-if="canCancel(item)"
                    class="aw-upload-center__remove"
                    type="button"
                    aria-label="取消下载"
                    title="取消下载"
                    @click="downloadCenter.cancel(item.id)"
                  >
                    <CircleStop :size="15" aria-hidden="true" />
                  </button>
                  <button
                    v-else-if="canRetry(item)"
                    class="aw-upload-center__remove"
                    type="button"
                    aria-label="重新下载"
                    title="重新下载"
                    @click="downloadCenter.retry(item.id)"
                  >
                    <RotateCcw :size="15" aria-hidden="true" />
                  </button>
                  <button
                    v-else
                    class="aw-upload-center__remove"
                    type="button"
                    aria-label="移除下载记录"
                    title="移除记录"
                    @click="downloadCenter.removeItem(item.id)"
                  >
                    <X :size="15" aria-hidden="true" />
                  </button>
                </div>
              </article>
            </div>

            <footer class="aw-upload-center__foot">
              <span class="aw-download-center__foot-note">失败任务不会自动重复下载</span>
              <button class="aw-secondary-button" type="button" @click="downloadCenter.clearFinished">清除已结束</button>
            </footer>
          </div>
        </Transition>
      </section>
    </Transition>
  </Teleport>
</template>
