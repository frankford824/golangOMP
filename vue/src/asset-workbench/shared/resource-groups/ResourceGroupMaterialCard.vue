<template>
  <section class="aw-resource-card" :class="{ 'is-published': published?.enabled }">
    <button class="aw-resource-card__main" type="button" @click="emit('select')" @dblclick="emit('preview')">
      <span class="aw-resource-card__cover">
        <img v-if="previewUrl" :src="previewUrl" :alt="title" loading="lazy" @error="emit('preview-failed', previewUrl)" />
        <span v-else aria-hidden="true">{{ asset.resource_mode === 'set' ? 'SET' : 'IMG' }}</span>
      </span>
      <span class="aw-resource-card__body">
        <strong>{{ title }}</strong>
        <small>{{ asset.task_no || '任务资源' }} · {{ sku }}</small>
        <span class="aw-resource-card__facts">
          <em>{{ modeLabel }}</em>
          <em>{{ itemCount }} 张最终成品</em>
          <em v-if="published?.finalized_revision_id">客户端固定版本 {{ published.finalized_revision_id }}</em>
        </span>
      </span>
      <span class="aw-resource-card__status">{{ published ? (published.enabled ? '客户端已上架' : '客户端已停用') : '未上架' }}</span>
    </button>
    <div class="aw-resource-card__actions">
      <button type="button" @click="emit('preview')">预览</button>
      <button type="button" @click="emit('download')">整组下载</button>
      <button ref="publishTrigger" v-if="canPublish" type="button" :disabled="publishing || loadingRevision" @click="openPublisher">
        {{ published ? '重新发布当前版本' : '发布到客户端' }}
      </button>
    </div>
    <p v-if="pinOutdated" class="aw-resource-card__pin-note">资源组已有新版本；客户端仍固定在版本 {{ published?.finalized_revision_id }}，不会自动切换。</p>

    <div v-if="publisherOpen" class="aw-resource-publisher-mask" role="presentation" @click.self="closePublisher">
      <section ref="publisherDialog" class="aw-resource-publisher" role="dialog" aria-modal="true" aria-labelledby="aw-resource-publisher-title" tabindex="-1" @keydown.esc.prevent="closePublisher" @keydown.tab="trapPublisherFocus">
        <header>
          <div><p>客户发布</p><h3 id="aw-resource-publisher-title">{{ published ? '重新发布资源组' : '发布资源组' }}</h3></div>
          <button ref="publisherClose" type="button" aria-label="关闭发布窗口" :disabled="publishing" @click="closePublisher">关闭</button>
        </header>
        <p>本次将固定资源版本 {{ revision?.id }}。资源组以后更新不会影响客户端，只有再次重新发布才会切换。</p>
        <div v-if="loadError" class="aw-resource-publisher__error" role="alert">{{ loadError }}</div>
        <fieldset v-else-if="revision">
          <legend>{{ revision.mode === 'set' ? '请选择一张客户封面' : '最终成品' }}</legend>
          <label v-for="item in orderedItems" :key="item.id" class="aw-resource-cover-option">
            <input v-model.number="selectedCoverID" type="radio" name="resource-cover" :value="item.id" />
            <span>{{ item.sort_order }}. {{ item.file?.file_name || item.item_name || `成品 ${item.id}` }}</span>
          </label>
        </fieldset>
        <p v-else>正在读取当前最终成品…</p>
        <footer>
          <button type="button" :disabled="publishing" @click="closePublisher">取消</button>
          <button type="button" :disabled="publishing || !revision || !selectedCoverID" @click="confirmPublish">
            {{ publishing ? '发布中…' : published ? '确认重新发布' : '确认发布' }}
          </button>
        </footer>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { assetWorkbenchApi, type ClientMaterialRow, type SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'
import type { WorkbenchResourceRevision } from './types'

const props = defineProps<{
  asset: SystemAssetRow
  published?: ClientMaterialRow | null
  previewUrl?: string
  canPublish?: boolean
  publishing?: boolean
}>()

const emit = defineEmits<{
  select: []
  preview: []
  download: []
  'preview-failed': [url: string]
  publish: [selection: { finalizedRevisionId: number; coverRevisionItemId: number }]
}>()

const publisherOpen = ref(false)
const loadingRevision = ref(false)
const revision = ref<WorkbenchResourceRevision | null>(null)
const selectedCoverID = ref(0)
const loadError = ref('')
const publishTrigger = ref<HTMLButtonElement | null>(null)
const publisherDialog = ref<HTMLElement | null>(null)
const publisherClose = ref<HTMLButtonElement | null>(null)
let previousFocus: HTMLElement | null = null
const title = computed(() => props.asset.product_name || props.asset.task_no || `资源组 ${props.asset.resource_group_id}`)
const sku = computed(() => props.asset.scope_sku_code || props.asset.sku_code || props.asset.primary_sku_code || '任务级资源')
const itemCount = computed(() => Number(props.asset.resource_item_count || revision.value?.items.length || 1))
const modeLabel = computed(() => (props.asset.resource_mode || revision.value?.mode) === 'set' ? '套装' : '单图')
const orderedItems = computed(() => [...(revision.value?.items || [])].sort((left, right) => left.sort_order - right.sort_order))
const pinOutdated = computed(() => Boolean(
  props.published?.finalized_revision_id
  && props.asset.finalized_revision_id
  && props.published.finalized_revision_id !== props.asset.finalized_revision_id,
))

async function openPublisher() {
  if (!props.asset.resource_group_id || loadingRevision.value) return
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : publishTrigger.value
  publisherOpen.value = true
  loadingRevision.value = true
  loadError.value = ''
  revision.value = null
  selectedCoverID.value = 0
  try {
    const group = await assetWorkbenchApi.getResourceGroup(props.asset.resource_group_id)
    const finalized = group.finalized_revision
    if (!finalized || !finalized.items.length) throw new Error('该资源组尚无可发布的最终成品。')
    revision.value = finalized
    if (finalized.mode === 'single') selectedCoverID.value = finalized.items[0]?.id || 0
  } catch (cause) {
    loadError.value = cause instanceof Error ? cause.message : '最终成品读取失败。'
  } finally {
    loadingRevision.value = false
    await nextTick()
    publisherClose.value?.focus()
  }
}

function closePublisher() {
  if (props.publishing) return
  publisherOpen.value = false
  void nextTick(() => (previousFocus || publishTrigger.value)?.focus())
}

function trapPublisherFocus(event: KeyboardEvent) {
  const controls = [...(publisherDialog.value?.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])') || [])]
  if (!controls.length) return
  const first = controls[0]
  const last = controls[controls.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}

function confirmPublish() {
  if (props.publishing || !revision.value || !selectedCoverID.value) return
  emit('publish', { finalizedRevisionId: revision.value.id, coverRevisionItemId: selectedCoverID.value })
}
</script>
