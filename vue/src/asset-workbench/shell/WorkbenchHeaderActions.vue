<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Bell, ChevronDown, Copy, LifeBuoy, LogOut, UserRound } from 'lucide-vue-next'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'

import { useWorkbenchSession } from '@aw/app/useWorkbenchSession'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'

const props = withDefaults(defineProps<{ displayName: string; profileHint: string; unreadCount?: number }>(), {
  unreadCount: 0,
})
const emit = defineEmits<{ openMessages: [] }>()
const route = useRoute()
const { logout } = useWorkbenchSession()
const openPanel = ref<'support' | 'account' | null>(null)
const feedbackContext = ref('')
const feedbackInput = ref<HTMLTextAreaElement | null>(null)
const feedbackNotice = ref('')
const loggingOut = ref(false)
const unreadLabel = computed(() => props.unreadCount ? '，未读 ' + props.unreadCount + ' 条消息' : '')
const unreadBadge = computed(() => props.unreadCount > 99 ? '99+' : String(props.unreadCount))

function setPanel(panel: 'support' | 'account', open: boolean) {
  if (open) openPanel.value = panel
  else if (openPanel.value === panel) openPanel.value = null
}

watch(() => route.path, () => {
  openPanel.value = null
  feedbackNotice.value = ''
  feedbackContext.value = ''
})

async function prepareFeedback() {
  const title = typeof route.meta.label === 'string' ? route.meta.label : '资产工作台'
  feedbackContext.value = ['资产工作台问题反馈', '页面：' + title + '（' + route.path + '）',
    '时间（北京时间）：' + formatShanghaiDateTime(new Date()), '问题描述：'].join('\n')
  feedbackNotice.value = ''
  try {
    await navigator.clipboard.writeText(feedbackContext.value)
    feedbackNotice.value = '反馈信息已复制，可补充问题描述后发给工作台管理员。'
  } catch {
    feedbackNotice.value = '可选中下方内容手动复制，再发给工作台管理员。'
    await nextTick()
    feedbackInput.value?.select()
  }
}

async function signOut() {
  if (loggingOut.value) return
  loggingOut.value = true
  try {
    await logout()
    openPanel.value = null
  } finally {
    loggingOut.value = false
  }
}
</script>

<template>
  <div class="aw-header-actions" role="group" aria-label="工作台操作">
    <PopoverRoot :open="openPanel === 'support'" @update:open="setPanel('support', $event)">
      <PopoverTrigger class="aw-secondary-button aw-header-actions__trigger" type="button"
        :aria-label="'反馈与协作' + unreadLabel" :data-current="route.path === '/notifications' || undefined">
        <LifeBuoy :size="16" aria-hidden="true" />
        <span>反馈与协作</span>
        <span v-if="unreadCount" class="aw-header-actions__badge" aria-hidden="true">{{ unreadBadge }}</span>
        <ChevronDown :size="14" aria-hidden="true" />
      </PopoverTrigger>
      <PopoverPortal>
        <PopoverContent class="aw-token-scope aw-header-popover" align="end" :side-offset="8" :collision-padding="12" aria-label="反馈与协作">
          <RouterLink class="aw-header-popover__action" to="/notifications" :aria-current="route.path === '/notifications' ? 'page' : undefined"
            @click="setPanel('support', false); emit('openMessages')">
            <Bell :size="17" aria-hidden="true" />
            <span>协作消息</span>
            <strong v-if="unreadCount" class="aw-header-popover__meta">{{ unreadBadge }} 条未读</strong>
          </RouterLink>
          <button class="aw-header-popover__action" type="button" @click="prepareFeedback">
            <Copy :size="17" aria-hidden="true" /> <span>复制反馈信息</span>
          </button>
          <div v-if="feedbackContext" class="aw-header-popover__context">
            <p class="aw-copy" role="status">{{ feedbackNotice }}</p>
            <textarea ref="feedbackInput" :value="feedbackContext" rows="4" readonly aria-label="反馈信息" />
          </div>
          <div v-if="$slots.connection" class="aw-header-popover__connection">
            <slot name="connection" />
          </div>
        </PopoverContent>
      </PopoverPortal>
    </PopoverRoot>

    <PopoverRoot :open="openPanel === 'account'" @update:open="setPanel('account', $event)">
      <PopoverTrigger class="aw-secondary-button aw-header-actions__trigger" type="button"
        aria-label="账号操作" :data-current="route.path === '/account' || undefined" :title="displayName">
        <UserRound :size="16" aria-hidden="true" />
        <span class="aw-header-actions__name">{{ displayName }}</span>
        <ChevronDown :size="14" aria-hidden="true" />
      </PopoverTrigger>
      <PopoverPortal>
        <PopoverContent class="aw-token-scope aw-header-popover" align="end" :side-offset="8" :collision-padding="12" aria-label="账号操作">
          <div class="aw-header-popover__identity">
            <strong>{{ displayName }}</strong>
            <small>{{ profileHint }}</small>
          </div>
          <RouterLink class="aw-header-popover__action" to="/account" :aria-current="route.path === '/account' ? 'page' : undefined" @click="setPanel('account', false)">
            <UserRound :size="17" aria-hidden="true" /> <span>我的资料</span>
          </RouterLink>
          <button class="aw-header-popover__action aw-header-popover__action--signout" type="button" :disabled="loggingOut" @click="signOut">
            <LogOut :size="17" aria-hidden="true" /> <span>{{ loggingOut ? '正在退出…' : '退出登录' }}</span>
          </button>
        </PopoverContent>
      </PopoverPortal>
    </PopoverRoot>
  </div>
</template>
