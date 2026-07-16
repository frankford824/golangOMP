<template>
  <section class="resource-matrix" aria-labelledby="resource-matrix-title">
    <header class="matrix-head">
      <div>
        <h2 id="resource-matrix-title">SKU 资源</h2>
        <p>参考图、当前有效源文件与当前最终成品图。</p>
      </div>
      <span class="revision">工作流修订 {{ bundle.workflow_revision }}</span>
    </header>

    <div v-if="!bundle.groups.length" class="empty">尚未建立资源组。</div>
    <article v-for="group in bundle.groups" :key="group.id" class="group-card">
      <div class="group-title">
        <div>
          <strong>{{ group.sku_code || scopeLabel(group) }}</strong>
          <span>{{ revision(group)?.mode === 'set' ? '套装' : '单图' }}</span>
        </div>
        <span v-if="group.migration_incomplete" class="migration-warning">资源待人工确认</span>
      </div>

      <div class="resource-row">
        <div class="row-label">参考图</div>
        <div class="file-list">
          <span v-for="(reference, index) in revision(group)?.references || []" :key="index" class="file-chip">
            {{ referenceName(reference, index) }}
          </span>
          <span v-if="!(revision(group)?.references?.length)" class="muted">无</span>
        </div>
      </div>
      <div class="resource-row">
        <div class="row-label">有效源文件</div>
        <div class="file-list">
          <a v-if="revision(group)?.source_file" :href="revision(group)?.source_file?.download_url" class="file-chip link">
            {{ revision(group)?.source_file?.file_name }}
          </a>
          <span v-else class="muted">源文件缺失</span>
        </div>
      </div>
      <div class="resource-row">
        <div class="row-label">最终成品图</div>
        <ol class="final-list">
          <li v-for="item in orderedItems(group)" :key="item.id">
            <span class="order">{{ item.sort_order + 1 }}</span>
            <a v-if="item.file?.download_url" :href="item.file.download_url">{{ item.file.file_name }}</a>
            <span v-else>{{ item.file?.file_name || item.item_name || `文件 ${item.task_asset_id}` }}</span>
          </li>
        </ol>
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import type { ResourceBundle, ResourceGroup, ResourceRevision } from '@/services/api/resourceGroupsApi'

defineProps<{ bundle: ResourceBundle }>()

const revision = (group: ResourceGroup): ResourceRevision | null | undefined => group.finalized_revision || group.working_revision
const orderedItems = (group: ResourceGroup) => [...(revision(group)?.items || [])].sort((a, b) => a.sort_order - b.sort_order)
const scopeLabel = (group: ResourceGroup) => group.scope_kind === 'retouch_requirement' ? `修图需求 ${group.retouch_requirement_id}` : '任务资源'
const referenceName = (reference: Record<string, unknown>, index: number) => String(reference.file_name_snapshot || reference.file_name || `参考图 ${index + 1}`)
</script>

<style scoped>
.resource-matrix { display: grid; gap: 16px; }
.matrix-head,.group-title { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.matrix-head h2 { margin: 0; font-size: 20px; }
.matrix-head p,.muted { color: rgb(var(--yb-text-muted)); }
.revision,.group-title span,.file-chip { font-size: 12px; }
.group-card { border: 1px solid rgb(var(--yb-border)); border-radius: 16px; background: rgb(var(--yb-surface)); overflow: hidden; }
.group-title { padding: 16px 18px; border-bottom: 1px solid rgb(var(--yb-border)); }
.group-title > div { display: flex; align-items: center; gap: 10px; }
.migration-warning { color: rgb(var(--yb-warning-text)); }
.resource-row { display: grid; grid-template-columns: 130px 1fr; gap: 16px; padding: 14px 18px; border-bottom: 1px solid rgb(var(--yb-border)); }
.resource-row:last-child { border-bottom: 0; }
.row-label { font-weight: 700; }
.file-list { display: flex; flex-wrap: wrap; gap: 8px; }
.file-chip { padding: 5px 9px; border-radius: 999px; background: rgb(var(--yb-surface-muted)); color: rgb(var(--yb-text)); }
.link,.final-list a { color: rgb(var(--yb-brand)); text-decoration: none; }
.final-list { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
.final-list li { display: flex; align-items: center; gap: 10px; }
.order { display: inline-flex; width: 24px; height: 24px; align-items: center; justify-content: center; border-radius: 8px; background: rgb(var(--yb-brand-soft)); }
.empty { padding: 36px; text-align: center; border: 1px dashed rgb(var(--yb-border)); border-radius: 16px; color: rgb(var(--yb-text-muted)); }
@media (max-width: 720px) { .resource-row { grid-template-columns: 1fr; gap: 7px; } .matrix-head { align-items: flex-start; } }
</style>
