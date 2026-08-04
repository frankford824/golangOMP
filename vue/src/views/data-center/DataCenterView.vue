<template>
  <div class="data-assistant-view">
    <AIChatConversationRail
      class="desktop-rail"
      :items="conversations"
      :active-id="activeConversationID"
      :loading="listLoading"
      :can-review-all="config?.can_review_all"
      @create="startNewConversation"
      @select="selectConversation"
      @review="adminOpen = true"
    />

    <section class="assistant-stage" aria-labelledby="assistant-title">
      <header class="assistant-header">
        <button type="button" class="history-button" aria-label="打开历史对话" @click="openHistory">
          <History aria-hidden="true" />
        </button>
        <div>
          <h1 id="assistant-title">数据智能助手</h1>
          <p>回答均附检索依据</p>
        </div>
        <div class="header-actions">
          <button v-if="activeConversationID" type="button" class="delete-button" aria-label="删除当前对话" @click="confirmDelete">
            <Trash2 aria-hidden="true" />
          </button>
          <button type="button" class="mobile-new-chat" @click="startNewConversation">
            <Plus aria-hidden="true" />
            <span>新对话</span>
          </button>
        </div>
      </header>

      <div ref="messageViewport" class="message-viewport" tabindex="-1">
        <div v-if="configLoading" class="stage-state">
          <LoaderCircle class="is-spinning" aria-hidden="true" />
          <p>正在连接数据助手…</p>
        </div>
        <div v-else-if="pageError" class="stage-state is-error" role="alert">
          <CircleAlert aria-hidden="true" />
          <h2>数据助手暂时不可用</h2>
          <p>{{ pageError }}</p>
          <button type="button" @click="initialize">重新连接</button>
        </div>
        <div v-else-if="config && !config.enabled" class="stage-state">
          <BotOff aria-hidden="true" />
          <h2>数据助手尚未启用</h2>
          <p>任务、资产和报表仍可正常使用；管理员启用分析服务后，这里会自动开放。</p>
        </div>
        <div v-else-if="conversationLoading" class="stage-state">
          <LoaderCircle class="is-spinning" aria-hidden="true" />
          <p>正在读取对话…</p>
        </div>
        <div v-else-if="!messages.length" class="assistant-empty">
          <div class="empty-mark" aria-hidden="true"><Sparkles /></div>
          <h2>从业务问题开始，而不是从报表开始</h2>
          <p>我会在您当前有权访问的任务、SKU 资源与经营数据中查找证据，并把依据一并列出。</p>
          <div class="starter-list" aria-label="常用问题">
            <button v-for="prompt in starterPrompts" :key="prompt" type="button" @click="sendMessage(prompt)">
              <span>{{ prompt }}</span><ArrowUpRight aria-hidden="true" />
            </button>
          </div>
        </div>
        <div v-else class="message-thread" aria-live="polite" aria-relevant="additions text">
          <AIChatMessage v-for="message in messages" :key="message.id" :message="message" @open-source="openSource" />
          <div v-if="streamStatus" class="stream-status" role="status">
            <span class="status-dot" aria-hidden="true" />
            <span>{{ streamStatus }}</span>
          </div>
          <div v-if="streamError" class="inline-error" role="alert">
            <CircleAlert aria-hidden="true" />
            <span>{{ streamError }}</span>
          </div>
        </div>
      </div>

      <footer v-if="config?.enabled" class="composer-dock">
        <AIChatComposer
          ref="composer"
          :max-length="config.max_input_chars"
          :disabled="conversationLoading"
          :streaming="streaming"
          @submit="sendMessage"
          @stop="stopGeneration"
        />
        <p>回答仅基于您当前可访问的数据，重要结论请打开依据复核。</p>
      </footer>
    </section>

    <Teleport to="body">
      <div v-if="historyOpen" class="history-backdrop" @click.self="closeHistory">
        <div ref="historyDrawer" class="history-drawer" role="dialog" aria-modal="true" aria-label="历史对话" @keydown="onHistoryKeydown">
          <header><strong>历史对话</strong><button ref="historyCloseButton" type="button" aria-label="关闭历史对话" @click="closeHistory"><X /></button></header>
          <AIChatConversationRail
            mobile
            :items="conversations"
            :active-id="activeConversationID"
            :loading="listLoading"
            :can-review-all="config?.can_review_all"
            @create="startNewConversation"
            @select="selectConversation"
            @review="openAdminFromMobile"
          />
        </div>
      </div>
    </Teleport>

    <AIChatAdminDrawer :open="adminOpen" @close="adminOpen = false" />
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowUpRight, BotOff, CircleAlert, History, LoaderCircle, Plus, Sparkles, Trash2, X } from 'lucide-vue-next'
import AIChatAdminDrawer from '@/components/data-center/AIChatAdminDrawer.vue'
import AIChatComposer from '@/components/data-center/AIChatComposer.vue'
import AIChatConversationRail from '@/components/data-center/AIChatConversationRail.vue'
import AIChatMessage from '@/components/data-center/AIChatMessage.vue'
import { aiChatApi, type AIChatConfig, type AIConversation, type AIMessage, type AIMessageSource, type AIStreamEvent } from '@/services/api/aiChatApi'
import type { HttpAppError } from '@/services/http'

const route = useRoute()
const router = useRouter()

const config = ref<AIChatConfig | null>(null)
const conversations = ref<AIConversation[]>([])
const activeConversationID = ref('')
const messages = ref<AIMessage[]>([])
const configLoading = ref(true)
const listLoading = ref(false)
const conversationLoading = ref(false)
const streaming = ref(false)
const streamStatus = ref('')
const streamError = ref('')
const pageError = ref('')
const historyOpen = ref(false)
const adminOpen = ref(false)
const messageViewport = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof AIChatComposer> | null>(null)
const historyDrawer = ref<HTMLElement | null>(null)
const historyCloseButton = ref<HTMLButtonElement | null>(null)
let loadController: AbortController | null = null
let streamController: AbortController | null = null
let historyOpener: HTMLElement | null = null

const starterPrompts = [
  '过去 30 天哪些环节最影响任务交付？',
  '最近审核打回主要集中在哪些原因？',
  '哪些 SKU 资源组最适合复用？',
]

onMounted(initialize)
onBeforeUnmount(() => {
  loadController?.abort()
  streamController?.abort()
})

async function initialize() {
  loadController?.abort()
  loadController = new AbortController()
  configLoading.value = true
  pageError.value = ''
  try {
    config.value = await aiChatApi.config(loadController.signal)
    if (!config.value.enabled) return
    await loadConversationList(loadController.signal)
    const requested = String(route.query.conversation ?? '').trim()
    if (requested && conversations.value.some((item) => item.id === requested)) await selectConversation(requested)
  } catch (reason) {
    if (!loadController.signal.aborted) pageError.value = friendlyError(reason)
  } finally {
    configLoading.value = false
  }
}

async function loadConversationList(signal?: AbortSignal) {
  listLoading.value = true
  try {
    conversations.value = (await aiChatApi.list(1, 60, signal)).items
  } finally {
    listLoading.value = false
  }
}

function startNewConversation() {
  if (streaming.value) stopGeneration()
  activeConversationID.value = ''
  messages.value = []
  historyOpen.value = false
  streamError.value = ''
  streamStatus.value = ''
  void router.replace({ path: route.path, query: {} })
  void nextTick(() => composer.value?.focus())
}

function openHistory(event: MouseEvent) {
  historyOpener = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
  historyOpen.value = true
  void nextTick(() => historyCloseButton.value?.focus())
}

function closeHistory() {
  historyOpen.value = false
  const target = historyOpener
  historyOpener = null
  void nextTick(() => target?.focus())
}

function onHistoryKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    closeHistory()
    return
  }
  if (event.key !== 'Tab') return
  const focusable = [...(historyDrawer.value?.querySelectorAll<HTMLElement>('button:not([disabled]),a[href],input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])') ?? [])]
    .filter((element) => element.getClientRects().length > 0)
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

async function selectConversation(id: string) {
  if (!id || conversationLoading.value) return
  if (streaming.value) stopGeneration()
  historyOpen.value = false
  conversationLoading.value = true
  streamError.value = ''
  try {
    const item = await aiChatApi.get(id)
    activeConversationID.value = item.id
    messages.value = item.messages ?? []
    await router.replace({ path: route.path, query: { conversation: item.id } })
    await scrollToLatest(false)
  } catch (reason) {
    streamError.value = friendlyError(reason)
  } finally {
    conversationLoading.value = false
  }
}

async function ensureConversation(question: string): Promise<string> {
  if (activeConversationID.value) return activeConversationID.value
  const item = await aiChatApi.create(question.slice(0, 80))
  activeConversationID.value = item.id
  conversations.value = [item, ...conversations.value.filter((existing) => existing.id !== item.id)]
  await router.replace({ path: route.path, query: { conversation: item.id } })
  return item.id
}

async function sendMessage(content: string) {
  if (streaming.value || !config.value?.enabled) return
  streamError.value = ''
  streamStatus.value = '正在准备分析…'
  try {
    const conversationID = await ensureConversation(content)
    const now = new Date().toISOString()
    const localUserID = `local-user-${crypto.randomUUID()}`
    const localAssistantID = `local-assistant-${crypto.randomUUID()}`
    messages.value.push({ id: localUserID, conversation_id: conversationID, client_message_id: localUserID, role: 'user', content, status: 'completed', created_at: now, updated_at: now })
    messages.value.push({ id: localAssistantID, conversation_id: conversationID, reply_to_message_id: localUserID, role: 'assistant', content: '', status: 'streaming', created_at: now, updated_at: now, sources: [] })
    streaming.value = true
    streamController = new AbortController()
    await scrollToLatest(true)
    await aiChatApi.streamMessage(conversationID, { client_message_id: localUserID, content }, (event) => handleStreamEvent(localAssistantID, event), streamController.signal)
    await loadConversationList()
  } catch (reason) {
    const assistant = [...messages.value].reverse().find((item) => item.role === 'assistant' && item.status === 'streaming')
    if (assistant) assistant.status = isAbort(reason) ? 'cancelled' : 'failed'
    if (!isAbort(reason)) streamError.value = friendlyError(reason)
  } finally {
    streaming.value = false
    streamStatus.value = ''
    streamController = null
    await scrollToLatest(true)
  }
}

function handleStreamEvent(localID: string, event: AIStreamEvent) {
  const index = messages.value.findIndex((item) => item.id === localID)
  const message = index >= 0 ? messages.value[index] : undefined
  if (event.type === 'meta' && message) {
    message.id = event.data.assistant_message_id
  } else if (event.type === 'status') {
    streamStatus.value = event.data.label
  } else if (event.type === 'retrieval' && message) {
    message.sources = event.data.sources
  } else if (event.type === 'delta' && message) {
    message.content += event.data.text
  } else if (event.type === 'done') {
    const current = messages.value.find((item) => item.id === event.data.message.id) ?? message
    if (current) Object.assign(current, event.data.message)
    streamStatus.value = ''
  } else if (event.type === 'error') {
    streamError.value = event.data.message
    if (message) {
      message.status = event.data.code.includes('cancel') ? 'cancelled' : 'failed'
      message.error_code = event.data.code
    }
  }
  void scrollToLatest(true)
}

function stopGeneration() {
  streamController?.abort()
  streamStatus.value = '正在停止生成…'
}

async function confirmDelete() {
  if (!activeConversationID.value || streaming.value) return
  if (!window.confirm('删除后对话会立即从列表中消失，是否继续？')) return
  try {
    await aiChatApi.remove(activeConversationID.value)
    conversations.value = conversations.value.filter((item) => item.id !== activeConversationID.value)
    startNewConversation()
  } catch (reason) {
    streamError.value = friendlyError(reason)
  }
}

function openSource(source: AIMessageSource) {
  const target = String(source.internal_route ?? '')
  if (!target.startsWith('/') || target.startsWith('//')) return
  void router.push(target)
}

function openAdminFromMobile() {
  historyOpen.value = false
  adminOpen.value = true
}

async function scrollToLatest(smooth: boolean) {
  await nextTick()
  const viewport = messageViewport.value
  if (!viewport) return
  if (typeof viewport.scrollTo === 'function') {
    viewport.scrollTo({ top: viewport.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
  } else {
    viewport.scrollTop = viewport.scrollHeight
  }
}

function isAbort(reason: unknown) {
  return reason instanceof DOMException && reason.name === 'AbortError'
}

function friendlyError(reason: unknown) {
  const error = reason as HttpAppError
  if (error?.status === 401) return '登录状态已失效，请重新登录后再试。'
  if (error?.status === 403) return '当前账号没有数据分析权限。'
  if (error?.status === 429) return '当前生成任务较多，请稍后再试。'
  if (error?.status === 503) return '分析服务正在维护，任务与资产功能不受影响。'
  return error?.message || '数据助手暂时无法连接，请稍后重试。'
}
</script>

<style scoped>
.data-assistant-view{display:flex;height:calc(100dvh - 7rem);min-height:36rem;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:.95rem;background:rgb(var(--yb-surface));box-shadow:0 12px 34px rgb(var(--yb-shadow)/.06);font-family:var(--yb-font-text);color:rgb(var(--yb-text))}.assistant-stage{display:flex;min-width:0;flex:1;flex-direction:column;background:linear-gradient(180deg,rgb(var(--yb-surface)) 0%,rgb(var(--yb-surface-near-white)) 100%)}.assistant-header{display:grid;min-height:5rem;grid-template-columns:2.5rem 1fr auto;align-items:center;gap:.75rem;border-bottom:1px solid rgb(var(--yb-border-quiet));padding:.85rem 1.4rem}.assistant-header h1{margin:0;color:rgb(var(--yb-text-navy));font-size:1.15rem;font-weight:820;letter-spacing:-.025em}.assistant-header p{margin:.2rem 0 0;color:rgb(var(--yb-text-muted));font-size:.72rem}.history-button,.delete-button,.mobile-new-chat{display:inline-flex;align-items:center;justify-content:center;border-radius:.65rem;font:inherit;cursor:pointer}.history-button{width:2.25rem;height:2.25rem;border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-secondary))}.history-button svg,.delete-button svg{width:1rem;height:1rem}.header-actions{display:flex;align-items:center;gap:.45rem}.delete-button{width:2.25rem;height:2.25rem;border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-muted))}.delete-button:hover{border-color:rgb(var(--yb-danger-border));background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.mobile-new-chat{min-height:2.25rem;gap:.35rem;border:1px solid rgb(var(--yb-brand-border-strong));background:rgb(var(--yb-surface));padding:0 .75rem;color:rgb(var(--yb-brand));font-size:.72rem;font-weight:760}.mobile-new-chat svg{width:.9rem;height:.9rem}.history-button{visibility:hidden}.history-button:focus-visible,.delete-button:focus-visible,.mobile-new-chat:focus-visible,.stage-state button:focus-visible,.starter-list button:focus-visible{outline:2px solid rgb(var(--yb-brand));outline-offset:2px}.message-viewport{display:flex;min-height:0;flex:1;flex-direction:column;overflow:auto;padding:2rem clamp(1.25rem,4vw,4rem);scroll-behavior:smooth}.message-thread{display:flex;width:min(100%,60rem);flex-direction:column;gap:1.55rem;margin:0 auto}.assistant-empty{display:grid;width:min(100%,42rem);margin:auto;justify-items:center;text-align:center}.empty-mark{display:grid;width:3rem;height:3rem;place-items:center;border:1px solid rgb(var(--yb-brand-border));border-radius:1rem;background:rgb(var(--yb-surface-blue-soft));color:rgb(var(--yb-brand));box-shadow:0 12px 30px rgb(var(--yb-brand)/.09)}.empty-mark svg{width:1.3rem}.assistant-empty h2{margin:1rem 0 0;color:rgb(var(--yb-text-navy));font-size:1.25rem;letter-spacing:-.025em}.assistant-empty>p{max-width:34rem;margin:.5rem 0 0;color:rgb(var(--yb-text-muted));font-size:.78rem;line-height:1.7}.starter-list{display:grid;width:min(100%,34rem);gap:.45rem;margin-top:1.4rem}.starter-list button{display:flex;min-height:2.85rem;align-items:center;justify-content:space-between;gap:1rem;border:1px solid rgb(var(--yb-border-blue-soft));border-radius:.72rem;background:rgb(var(--yb-surface));padding:.55rem .75rem .55rem .9rem;color:rgb(var(--yb-text-body));font:inherit;font-size:.76rem;text-align:left;cursor:pointer;transition:transform .16s ease,border-color .16s ease,box-shadow .16s ease}.starter-list button:hover{border-color:rgb(var(--yb-brand-border-strong));box-shadow:0 8px 18px rgb(var(--yb-brand)/.07);transform:translateY(-1px)}.starter-list svg{width:.9rem;height:.9rem;color:rgb(var(--yb-brand))}.stream-status{display:flex;align-items:center;gap:.5rem;color:rgb(var(--yb-brand-link));font-size:.72rem;font-weight:690}.status-dot{width:.42rem;height:.42rem;border-radius:999px;background:rgb(var(--yb-brand));box-shadow:0 0 0 .25rem rgb(var(--yb-brand)/.1);animation:status-breathe 1.2s ease-in-out infinite}.inline-error{display:flex;align-items:center;gap:.45rem;border:1px solid rgb(var(--yb-danger-border));border-radius:.65rem;background:rgb(var(--yb-danger-soft));padding:.65rem .75rem;color:rgb(var(--yb-danger-text));font-size:.72rem}.inline-error svg{width:.9rem;height:.9rem}.stage-state{display:grid;margin:auto;justify-items:center;text-align:center;color:rgb(var(--yb-text-muted))}.stage-state>svg{width:1.65rem;height:1.65rem;color:rgb(var(--yb-brand))}.stage-state h2{margin:.75rem 0 0;color:rgb(var(--yb-text-navy));font-size:1rem}.stage-state p{max-width:30rem;margin:.4rem 0 0;font-size:.75rem;line-height:1.6}.stage-state button{margin-top:1rem;border:1px solid rgb(var(--yb-brand));border-radius:.6rem;background:rgb(var(--yb-brand));padding:.55rem .8rem;color:rgb(var(--yb-text-inverse));font:inherit;font-size:.72rem;font-weight:720;cursor:pointer}.stage-state.is-error>svg{color:rgb(var(--yb-danger-text))}.composer-dock{display:grid;gap:.45rem;padding:.8rem clamp(1rem,3vw,2.5rem) 1rem}.composer-dock>*{width:min(100%,60rem);margin:0 auto}.composer-dock>p{color:rgb(var(--yb-text-faint));font-size:.62rem;text-align:center}.history-backdrop{position:fixed;inset:0;z-index:80;display:flex;background:rgb(var(--yb-overlay-night)/.36);backdrop-filter:blur(4px)}.history-drawer{display:flex;width:min(21rem,88vw);height:100%;flex-direction:column;background:rgb(var(--yb-surface));box-shadow:14px 0 36px rgb(var(--yb-shadow)/.16)}.history-drawer>header{display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid rgb(var(--yb-border));padding:1rem}.history-drawer>header strong{font-size:.85rem}.history-drawer>header button{display:grid;width:2rem;height:2rem;place-items:center;border:1px solid rgb(var(--yb-border));border-radius:.55rem;background:rgb(var(--yb-surface));cursor:pointer}.history-drawer>header svg{width:1rem}.is-spinning{animation:spin .9s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}@keyframes status-breathe{50%{opacity:.35;transform:scale(.8)}}@media(max-width:767px){.data-assistant-view{height:calc(100dvh - 5.5rem);min-height:32rem;border:0;border-radius:0;box-shadow:none}.desktop-rail{display:none}.assistant-header{min-height:4.8rem;grid-template-columns:2.35rem minmax(0,1fr) auto;padding:.7rem 1rem}.assistant-header h1{font-size:1rem;text-align:center}.assistant-header p{text-align:center}.history-button{visibility:visible}.delete-button{display:none}.message-viewport{padding:1.35rem 1rem}.assistant-empty{margin-top:2rem}.assistant-empty h2{font-size:1.05rem}.assistant-empty>p{font-size:.75rem}.message-thread{gap:1.35rem}.composer-dock{padding:.65rem .75rem max(.75rem,env(safe-area-inset-bottom))}.mobile-new-chat span{display:none}.mobile-new-chat{width:2.35rem;padding:0}.assistant-stage{min-height:0}}@media(prefers-reduced-motion:reduce){.status-dot,.is-spinning{animation:none}.starter-list button{transition:none}.message-viewport{scroll-behavior:auto}}
</style>
