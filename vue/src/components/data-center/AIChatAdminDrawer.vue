<template>
  <Teleport to="body">
    <div v-if="open" class="review-backdrop" @click.self="close">
      <aside ref="panel" class="review-drawer" role="dialog" aria-modal="true" aria-labelledby="review-title" @keydown="handleKeydown">
        <header>
          <div>
            <h2 id="review-title">对话审阅</h2>
            <p>仅超级管理员可查看，跨用户阅读会记录审计。</p>
          </div>
          <button ref="closeButton" type="button" aria-label="关闭对话审阅" @click="close"><X aria-hidden="true" /></button>
        </header>
        <form class="review-filter" @submit.prevent="load">
          <label>人员编号<input v-model.trim="ownerID" inputmode="numeric" placeholder="全部人员" /></label>
          <label>开始日期<input v-model="from" type="date" /></label>
          <label>结束日期<input v-model="to" type="date" /></label>
          <button type="submit">筛选</button>
        </form>
        <div class="review-body">
          <div class="review-list" aria-label="全部对话">
            <p v-if="loading" class="review-state">正在读取对话…</p>
            <p v-else-if="error" class="review-state is-error">{{ error }}</p>
            <p v-else-if="!items.length" class="review-state">没有符合条件的对话</p>
            <button v-for="item in items" :key="item.id" type="button" :class="{ active: selected?.id === item.id }" @click="select(item.id)">
              <strong>{{ item.title || '未命名对话' }}</strong>
              <span>{{ item.owner_name || `人员 ${item.owner_user_id}` }}</span>
              <time :datetime="item.updated_at">{{ formatDate(item.updated_at) }}</time>
            </button>
          </div>
          <section class="review-thread" aria-live="polite">
            <p v-if="detailLoading" class="review-state">正在读取对话内容…</p>
            <template v-else-if="selected">
              <header class="thread-header"><strong>{{ selected.title || '未命名对话' }}</strong><span>{{ selected.owner_name }}</span></header>
              <div class="thread-message" v-for="message in selected.messages ?? []" :key="message.id" :class="`is-${message.role}`">
                <span>{{ message.role === 'user' ? '提问' : '回答' }}</span>
                <p>{{ message.content || (message.status === 'cancelled' ? '生成已停止' : '暂无内容') }}</p>
              </div>
            </template>
            <p v-else class="review-state">选择一条对话查看审阅内容</p>
          </section>
        </div>
      </aside>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { X } from 'lucide-vue-next'
import { aiChatApi, type AIConversation } from '@/services/api/aiChatApi'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()

const panel = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const items = ref<AIConversation[]>([])
const selected = ref<AIConversation | null>(null)
const ownerID = ref('')
const from = ref('')
const to = ref('')
const loading = ref(false)
const detailLoading = ref(false)
const error = ref('')
let previousFocus: HTMLElement | null = null
let controller: AbortController | null = null

watch(() => props.open, (value) => {
  if (!value) return
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  void nextTick(() => closeButton.value?.focus())
  void load()
})

onBeforeUnmount(() => controller?.abort())

function close() {
  controller?.abort()
  emit('close')
  void nextTick(() => previousFocus?.focus())
}

async function load() {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const owner = Number(ownerID.value)
    const result = await aiChatApi.adminList({ owner_user_id: Number.isInteger(owner) && owner > 0 ? owner : undefined, from: from.value || undefined, to: to.value || undefined, page: 1, page_size: 60 }, controller.signal)
    items.value = result.items
  } catch (reason) {
    if (!controller.signal.aborted) error.value = reason instanceof Error ? reason.message : '读取对话失败'
  } finally {
    loading.value = false
  }
}

async function select(id: string) {
  detailLoading.value = true
  try {
    selected.value = await aiChatApi.adminGet(id)
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : '读取对话失败'
  } finally {
    detailLoading.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key !== 'Tab' || !panel.value) return
  const focusable = [...panel.value.querySelectorAll<HTMLElement>('button:not([disabled]),input:not([disabled]),[tabindex]:not([tabindex="-1"])')]
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? '' : date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
</script>

<style scoped>
.review-backdrop{position:fixed;inset:0;z-index:90;display:flex;justify-content:flex-end;background:rgb(var(--yb-overlay-night)/.38);backdrop-filter:blur(5px)}.review-drawer{display:flex;width:min(70rem,92vw);height:100%;flex-direction:column;background:rgb(var(--yb-surface));box-shadow:-18px 0 50px rgb(var(--yb-shadow)/.16)}.review-drawer>header{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem;border-bottom:1px solid rgb(var(--yb-border));padding:1.1rem 1.25rem}.review-drawer h2{margin:0;color:rgb(var(--yb-text-navy));font-size:1rem}.review-drawer header p{margin:.2rem 0 0;color:rgb(var(--yb-text-muted));font-size:.72rem}.review-drawer header button{display:grid;width:2rem;height:2rem;place-items:center;border:1px solid rgb(var(--yb-border));border-radius:.55rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-secondary));cursor:pointer}.review-drawer header svg{width:1rem}.review-filter{display:flex;align-items:flex-end;gap:.65rem;border-bottom:1px solid rgb(var(--yb-border-quiet));padding:.75rem 1.25rem}.review-filter label{display:grid;gap:.25rem;color:rgb(var(--yb-text-muted));font-size:.65rem}.review-filter input{min-height:2.15rem;border:1px solid rgb(var(--yb-border));border-radius:.5rem;background:rgb(var(--yb-surface));padding:0 .65rem;color:rgb(var(--yb-text));font:inherit;font-size:.72rem}.review-filter button{min-height:2.15rem;border:1px solid rgb(var(--yb-brand));border-radius:.5rem;background:rgb(var(--yb-brand));padding:0 .85rem;color:rgb(var(--yb-text-inverse));font:inherit;font-size:.72rem;font-weight:720;cursor:pointer}.review-body{display:grid;min-height:0;flex:1;grid-template-columns:19rem minmax(0,1fr)}.review-list{overflow:auto;border-right:1px solid rgb(var(--yb-border));padding:.65rem}.review-list>button{display:grid;width:100%;gap:.18rem;border:0;border-radius:.55rem;background:transparent;padding:.65rem;text-align:left;cursor:pointer}.review-list>button:hover,.review-list>button.active{background:rgb(var(--yb-surface-blue-soft))}.review-list strong{overflow:hidden;color:rgb(var(--yb-text));font-size:.75rem;text-overflow:ellipsis;white-space:nowrap}.review-list span,.review-list time{color:rgb(var(--yb-text-muted));font-size:.65rem}.review-thread{overflow:auto;padding:1.2rem 1.4rem}.thread-header{display:flex;justify-content:space-between;gap:1rem;border-bottom:1px solid rgb(var(--yb-border));padding-bottom:.8rem}.thread-header span{color:rgb(var(--yb-text-muted));font-size:.72rem}.thread-message{display:grid;gap:.3rem;margin-top:1rem}.thread-message>span{color:rgb(var(--yb-brand));font-size:.65rem;font-weight:800}.thread-message p{margin:0;border-left:2px solid rgb(var(--yb-border-blue-soft));padding-left:.8rem;white-space:pre-wrap;color:rgb(var(--yb-text-body));font-size:.78rem;line-height:1.65}.thread-message.is-user>span{color:rgb(var(--yb-text-muted))}.review-state{padding:.8rem;color:rgb(var(--yb-text-muted));font-size:.72rem}.review-state.is-error{color:rgb(var(--yb-danger-text))}@media(max-width:760px){.review-drawer{width:100vw}.review-filter{display:grid;grid-template-columns:1fr 1fr}.review-filter label:first-child{grid-column:1/-1}.review-body{grid-template-columns:1fr}.review-list{max-height:35vh;border-right:0;border-bottom:1px solid rgb(var(--yb-border))}.review-thread{padding:1rem}}
</style>
