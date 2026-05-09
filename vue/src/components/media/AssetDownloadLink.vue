<template>
  <a
    :href="href || '#'"
    class="asset-dl-link"
    :class="{ 'asset-dl-link--button': variant === 'button' }"
    rel="noopener noreferrer"
    @click="onClick"
  >
    <template v-if="variant === 'button'">
      <Download class="asset-dl-icon" :size="15" :stroke-width="2" aria-hidden="true" />
      <span class="asset-dl-text"><slot>下载文件</slot></span>
    </template>
    <template v-else>
      <slot>下载</slot>
    </template>
  </a>
</template>

<script setup lang="ts">
import { Download } from 'lucide-vue-next'
import { fetchAssetDownloadUrl } from '@/domain/asset-access'

const props = withDefaults(
  defineProps<{
    assetId?: string | null
    /** 无 assetId 或接口失败时的直链 */
    href?: string | null
    /** `button`：实心按钮样式（任务详情 / 工作台主预览区） */
    variant?: 'link' | 'button'
  }>(),
  { assetId: null, href: null, variant: 'link' },
)

async function onClick(ev: MouseEvent) {
  const id = props.assetId?.trim()
  if (!id) return
  ev.preventDefault()
  const result = await fetchAssetDownloadUrl(id)
  if (result.status === 'ok' && result.downloadUrl) {
    window.open(result.downloadUrl, '_blank', 'noopener,noreferrer')
    return
  }
  if (props.href?.trim()) {
    window.open(props.href.trim(), '_blank', 'noopener,noreferrer')
  }
}
</script>

<style scoped>
.asset-dl-link {
  font-size: 0.75rem;
  color: rgb(37 99 235);
  cursor: pointer;
  text-decoration: underline;
}
.asset-dl-link--button {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.9rem;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #fff;
  text-decoration: none;
  background: rgb(37 99 235);
  border: 1px solid rgb(29 78 216);
  border-radius: 0.5rem;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.08);
  transition: background 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}
.asset-dl-link--button:hover {
  background: rgb(29 78 216);
  border-color: rgb(30 64 175);
  box-shadow: 0 2px 6px rgb(37 99 235 / 0.25);
}
.asset-dl-link--button:focus-visible {
  outline: 2px solid rgb(147 197 253);
  outline-offset: 2px;
}
.asset-dl-icon {
  flex-shrink: 0;
  opacity: 0.95;
}
.asset-dl-text {
  line-height: 1.2;
}
</style>
