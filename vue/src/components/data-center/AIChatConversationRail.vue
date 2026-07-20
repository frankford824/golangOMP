<template>
  <aside class="conversation-rail" :class="{ 'is-mobile': mobile }" aria-label="对话历史">
    <div class="rail-actions">
      <button type="button" class="new-chat-button" @click="$emit('create')">
        <MessageSquarePlus aria-hidden="true" />
        <span>新对话</span>
      </button>
      <button v-if="canReviewAll" type="button" class="review-button" @click="$emit('review')">
        <ShieldCheck aria-hidden="true" />
        <span>审阅全部对话</span>
      </button>
    </div>

    <div v-if="loading" class="rail-state">正在读取历史对话…</div>
    <div v-else-if="!groups.length" class="rail-state">还没有历史对话</div>
    <div v-else class="conversation-groups">
      <section v-for="group in groups" :key="group.label" class="conversation-group">
        <h2>{{ group.label }}</h2>
        <ul>
          <li v-for="item in group.items" :key="item.id">
            <button
              type="button"
              class="conversation-button"
              :class="{ active: item.id === activeId }"
              :aria-current="item.id === activeId ? 'page' : undefined"
              @click="$emit('select', item.id)"
            >
              <MessageCircle aria-hidden="true" />
              <span>{{ item.title || '未命名对话' }}</span>
              <time :datetime="item.updated_at">{{ displayDate(item.updated_at) }}</time>
            </button>
          </li>
        </ul>
      </section>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { MessageCircle, MessageSquarePlus, ShieldCheck } from 'lucide-vue-next'
import type { AIConversation } from '@/services/api/aiChatApi'

const props = withDefaults(defineProps<{
  items: AIConversation[]
  activeId?: string
  loading?: boolean
  mobile?: boolean
  canReviewAll?: boolean
}>(), { activeId: '', loading: false, mobile: false, canReviewAll: false })

defineEmits<{
  (event: 'create'): void
  (event: 'select', id: string): void
  (event: 'review'): void
}>()

const groups = computed(() => {
  const today: AIConversation[] = []
  const week: AIConversation[] = []
  const earlier: AIConversation[] = []
  const now = Date.now()
  for (const item of props.items) {
    const age = now - new Date(item.updated_at).valueOf()
    if (age <= 86400000) today.push(item)
    else if (age <= 7 * 86400000) week.push(item)
    else earlier.push(item)
  }
  return [
    { label: '今天', items: today },
    { label: '近 7 天', items: week },
    { label: '更早', items: earlier },
  ].filter((group) => group.items.length)
})

function displayDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.valueOf())) return ''
  if (Date.now() - date.valueOf() <= 86400000) return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return `${date.getMonth() + 1}-${String(date.getDate()).padStart(2, '0')}`
}
</script>

<style scoped>
.conversation-rail{display:flex;min-width:17rem;max-width:17rem;flex-direction:column;border-right:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));overflow:hidden}.rail-actions{display:grid;gap:.45rem;padding:1rem}.new-chat-button,.review-button{display:flex;width:100%;min-height:2.65rem;align-items:center;justify-content:center;gap:.5rem;border-radius:.72rem;font:inherit;font-size:.78rem;font-weight:760;cursor:pointer}.new-chat-button{border:1px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));box-shadow:0 7px 18px rgb(var(--yb-brand)/.18)}.review-button{border:1px solid rgb(var(--yb-border-blue-soft));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-secondary))}.new-chat-button svg,.review-button svg{width:1rem;height:1rem}.new-chat-button:focus-visible,.review-button:focus-visible,.conversation-button:focus-visible{outline:2px solid rgb(var(--yb-brand));outline-offset:2px}.conversation-groups{flex:1;overflow:auto;border-top:1px solid rgb(var(--yb-border-quiet));padding:.85rem .75rem 1.2rem}.conversation-group+.conversation-group{margin-top:1rem}.conversation-group h2{margin:0 0 .35rem;padding:0 .4rem;color:rgb(var(--yb-text-faint));font-size:.67rem;font-weight:700}.conversation-group ul{display:grid;gap:.15rem;margin:0;padding:0;list-style:none}.conversation-button{display:grid;width:100%;grid-template-columns:1rem minmax(0,1fr) auto;align-items:center;gap:.45rem;border:0;border-radius:.58rem;background:transparent;padding:.55rem .5rem;color:rgb(var(--yb-text-secondary));font:inherit;font-size:.74rem;text-align:left;cursor:pointer}.conversation-button:hover{background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text))}.conversation-button.active{background:rgb(var(--yb-surface-blue-focus));color:rgb(var(--yb-brand-strong));font-weight:730}.conversation-button svg{width:.9rem;height:.9rem}.conversation-button span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.conversation-button time{color:rgb(var(--yb-text-faint));font-size:.64rem;font-weight:500}.rail-state{padding:1.25rem;color:rgb(var(--yb-text-muted));font-size:.74rem}.conversation-rail.is-mobile{min-width:0;max-width:none;width:100%;height:100%;border-right:0}
</style>
