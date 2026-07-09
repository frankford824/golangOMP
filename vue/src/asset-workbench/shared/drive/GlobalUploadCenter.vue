<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAutoAnimate } from '@formkit/auto-animate/vue'
import { AlertCircle, CheckCircle2, ChevronDown, FileUp, LoaderCircle, UploadCloud, X } from 'lucide-vue-next'

import { formatFileSize, formatInt } from '@aw/shared/format/number'
import { useUploadCenterStore, type UploadCenterItem, type UploadCenterStatus } from './uploadCenter.store'

const router = useRouter()
const uploadCenter = useUploadCenterStore()
const [listRef] = useAutoAnimate({ duration: 180, easing: 'ease-out' })

const shellVisible = computed(() => uploadCenter.hasItems)
const ringStyle = computed(() => ({
  '--aw-upload-center-progress': `${Math.max(uploadCenter.overallProgress, uploadCenter.hasActive ? 8 : 100)}%`,
}))
const panelTitle = computed(() => {
  if (uploadCenter.hasActive) return '上传正在进行'
  if (uploadCenter.failedItems.length) return '上传需要处理'
  if (uploadCenter.pendingRecordItems.length) return '等待生成记录'
  return '上传已完成'
})

function statusLabel(status: UploadCenterStatus) {
  const labels: Record<UploadCenterStatus, string> = {
    queued: '等待上传',
    uploading: '上传中',
    uploaded: '待生成记录',
    submitting: '生成记录中',
    submitted: '已完成',
    failed: '上传失败',
  }
  return labels[status]
}

function statusIcon(status: UploadCenterStatus) {
  if (status === 'failed') return AlertCircle
  if (status === 'submitted' || status === 'uploaded') return CheckCircle2
  if (status === 'uploading' || status === 'submitting') return LoaderCircle
  return FileUp
}

function itemSubtitle(item: UploadCenterItem) {
  const parts = [
    item.uploadDirectoryName || '默认目录',
    item.difficultyClass ? `计价 ${item.difficultyClass}` : '',
    formatFileSize(item.fileSize),
  ].filter(Boolean)
  return parts.join(' · ')
}

function itemProgress(item: UploadCenterItem) {
  if (item.status === 'submitted') return 100
  if (item.status === 'uploaded' || item.status === 'submitting') return 96
  return Math.max(0, Math.min(100, item.progress))
}

async function goUploadPage() {
  uploadCenter.openPanel()
  await router.push('/upload')
}
</script>

<template>
  <Teleport to="body">
    <Transition name="aw-upload-center-shell">
      <section v-if="shellVisible" class="aw-token-scope aw-upload-center" :class="{ 'is-open': uploadCenter.panelOpen }" aria-label="全局上传中心">
        <button
          class="aw-upload-center__orb"
          type="button"
          :style="ringStyle"
          :aria-expanded="uploadCenter.panelOpen"
          aria-controls="aw-upload-center-panel"
          @click="uploadCenter.panelOpen ? uploadCenter.closePanel() : uploadCenter.openPanel()"
        >
          <span class="aw-upload-center__ring" aria-hidden="true">
            <UploadCloud :size="20" />
          </span>
          <span class="aw-upload-center__orb-copy">
            <strong>{{ uploadCenter.hasActive ? `${formatInt(uploadCenter.overallProgress)}%` : '上传' }}</strong>
            <small>{{ uploadCenter.summaryText }}</small>
          </span>
        </button>

        <Transition name="aw-upload-center-panel">
          <div v-if="uploadCenter.panelOpen" id="aw-upload-center-panel" class="aw-upload-center__panel">
            <header class="aw-upload-center__head">
              <div>
                <p class="aw-eyebrow">上传中心</p>
                <h3>{{ panelTitle }}</h3>
                <span>{{ uploadCenter.summaryText }}</span>
              </div>
              <button class="aw-icon-action" type="button" aria-label="收起上传中心" @click="uploadCenter.closePanel()">
                <ChevronDown :size="16" aria-hidden="true" />
              </button>
            </header>

            <div v-if="uploadCenter.failedItems.length" class="aw-upload-center__failed-callout">
              <strong>{{ formatInt(uploadCenter.failedItems.length) }} 个文件上传失败</strong>
              <span>失败文件已保留，但不会自动重复上传。恢复网络后请回到上传页手动重试。</span>
              <div>
                <button class="aw-secondary-button" type="button" @click="goUploadPage">去上传页处理</button>
                <button class="aw-secondary-button" type="button" @click="uploadCenter.clearFailed">清除失败记录</button>
              </div>
            </div>

            <div ref="listRef" class="aw-upload-center__list">
              <article
                v-for="item in uploadCenter.visibleItems"
                :key="item.id"
                class="aw-upload-center__item"
                :class="`is-${item.status}`"
              >
                <component :is="statusIcon(item.status)" :size="18" aria-hidden="true" />
                <div class="aw-upload-center__item-main">
                  <div class="aw-upload-center__item-title">
                    <strong :title="item.displayName">{{ item.displayName }}</strong>
                    <span>{{ statusLabel(item.status) }}</span>
                  </div>
                  <small>{{ itemSubtitle(item) }}</small>
                  <div class="aw-upload-center__bar" :aria-label="`${statusLabel(item.status)} ${itemProgress(item)}%`">
                    <span :style="{ width: `${itemProgress(item)}%` }" />
                  </div>
                  <p v-if="item.error" class="aw-upload-center__error">{{ item.error }}</p>
                </div>
                <button
                  v-if="item.status !== 'uploading' && item.status !== 'submitting'"
                  class="aw-upload-center__remove"
                  type="button"
                  aria-label="移除上传任务"
                  @click="uploadCenter.removeItem(item.id)"
                >
                  <X :size="14" aria-hidden="true" />
                </button>
              </article>
            </div>

            <footer class="aw-upload-center__foot">
              <button class="aw-secondary-button" type="button" @click="goUploadPage">查看上传页</button>
              <button class="aw-secondary-button" type="button" :disabled="!uploadCenter.finishedItems.length" @click="uploadCenter.clearFinished">
                清除完成
              </button>
            </footer>
          </div>
        </Transition>
      </section>
    </Transition>
  </Teleport>
</template>
