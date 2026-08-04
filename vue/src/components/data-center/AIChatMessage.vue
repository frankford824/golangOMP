<template>
  <article class="assistant-message" :class="`is-${message.role}`" :data-message-role="message.role">
    <div v-if="message.role === 'assistant'" class="assistant-avatar" aria-hidden="true">数</div>
    <div class="message-content">
      <div class="message-bubble">
        <p v-for="(paragraph, index) in paragraphs" :key="index">{{ paragraph }}</p>
        <span v-if="message.status === 'streaming'" class="stream-caret" aria-label="正在生成" />
      </div>
      <div v-if="message.role === 'assistant' && sources.length" class="source-region">
        <p class="source-label">检索依据</p>
        <div class="source-list">
          <button
            v-for="source in sources"
            :key="source.source_id"
            type="button"
            class="source-link"
            :title="source.evidence_excerpt"
            @click="$emit('open-source', source)"
          >
            <component :is="sourceIcon(source.entity_type)" aria-hidden="true" />
            <span>{{ source.title }}</span>
            <ArrowUpRight aria-hidden="true" />
          </button>
        </div>
      </div>
      <p v-if="message.status === 'cancelled'" class="message-state">生成已停止，已保留当前内容。</p>
      <p v-else-if="message.status === 'failed'" class="message-state is-error">本次分析未完成，请重新提问。</p>
      <time v-if="message.role === 'user'" class="message-time" :datetime="message.created_at">{{ displayTime }}</time>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowUpRight, Boxes, FileText, MessageCircle, PackageSearch } from 'lucide-vue-next'
import type { AIMessage, AIMessageSource } from '@/services/api/aiChatApi'

const props = defineProps<{ message: AIMessage }>()
defineEmits<{ (event: 'open-source', source: AIMessageSource): void }>()

const sources = computed(() => props.message.sources ?? [])
const paragraphs = computed(() => {
  const value = props.message.content.trim()
  if (!value) return props.message.status === 'streaming' ? [''] : []
  return value.split(/\n{2,}/).map((item) => item.trim()).filter(Boolean)
})
const displayTime = computed(() => {
  const date = new Date(props.message.created_at)
  return Number.isNaN(date.valueOf()) ? '' : date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
})

function sourceIcon(type: string) {
  if (type === 'task') return FileText
  if (type === 'task_resource_group') return Boxes
  if (type === 'external_asset' || type === 'system_asset') return PackageSearch
  return MessageCircle
}
</script>

<style scoped>
.assistant-message{display:flex;align-items:flex-start;gap:.85rem;max-width:51rem}.assistant-message.is-user{align-self:flex-end;flex-direction:row-reverse;max-width:38rem}.assistant-avatar{display:grid;width:2.15rem;height:2.15rem;flex:0 0 auto;place-items:center;border-radius:999px;background:linear-gradient(145deg,rgb(var(--yb-brand-bright)),rgb(var(--yb-info-sky)));color:rgb(var(--yb-text-inverse));font-size:.78rem;font-weight:850;box-shadow:0 7px 18px rgb(var(--yb-brand)/.18)}.message-content{display:grid;min-width:0;gap:.65rem}.message-bubble{font-size:.9rem;line-height:1.78;color:rgb(var(--yb-text-body-strong))}.message-bubble p{margin:0;white-space:pre-wrap}.message-bubble p+p{margin-top:.8rem}.is-user .message-bubble{border:1px solid rgb(var(--yb-brand-border)/.65);border-radius:1rem 1rem .35rem 1rem;background:rgb(var(--yb-surface-blue-soft));padding:.8rem 1rem;color:rgb(var(--yb-text-navy));box-shadow:0 7px 20px rgb(var(--yb-brand)/.07)}.source-region{display:grid;gap:.45rem;border-top:1px solid rgb(var(--yb-border-blue-quiet));padding-top:.65rem}.source-label{margin:0;color:rgb(var(--yb-text-muted));font-size:.68rem;font-weight:720}.source-list{display:flex;flex-wrap:wrap;gap:.4rem}.source-link{display:inline-flex;min-height:2rem;align-items:center;gap:.35rem;border:1px solid rgb(var(--yb-border-blue-soft));border-radius:.55rem;background:rgb(var(--yb-surface));padding:.35rem .55rem;color:rgb(var(--yb-brand-link));font:inherit;font-size:.72rem;font-weight:700;cursor:pointer;transition:border-color .16s ease,background .16s ease,transform .16s ease}.source-link:hover{border-color:rgb(var(--yb-brand-border-strong));background:rgb(var(--yb-surface-blue-soft));transform:translateY(-1px)}.source-link:focus-visible{outline:2px solid rgb(var(--yb-brand));outline-offset:2px}.source-link svg{width:.82rem;height:.82rem;flex:0 0 auto}.message-time{justify-self:end;color:rgb(var(--yb-text-faint));font-size:.67rem}.message-state{margin:0;color:rgb(var(--yb-text-muted));font-size:.72rem}.message-state.is-error{color:rgb(var(--yb-danger-text))}.stream-caret{display:inline-block;width:.38rem;height:1rem;margin-left:.12rem;border-radius:.15rem;background:rgb(var(--yb-brand));vertical-align:-.17rem;animation:caret-pulse .85s ease-in-out infinite}@keyframes caret-pulse{50%{opacity:.16}}@media(max-width:720px){.assistant-message,.assistant-message.is-user{max-width:100%}.assistant-avatar{width:2rem;height:2rem}.message-bubble{font-size:.88rem;line-height:1.75}.is-user .message-bubble{padding:.72rem .85rem}.source-list{display:grid}.source-link{width:100%;justify-content:flex-start}.source-link svg:last-child{margin-left:auto}}@media(prefers-reduced-motion:reduce){.stream-caret{animation:none}.source-link{transition:none}}
</style>
