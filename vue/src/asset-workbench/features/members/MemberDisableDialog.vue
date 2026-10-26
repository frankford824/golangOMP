<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import type { WorkbenchMemberRow } from '@aw/shared/api/assetWorkbenchApi'

const props = defineProps<{ member: WorkbenchMemberRow | null; busy: boolean; error: string }>()
const emit = defineEmits<{ close: []; closed: []; confirm: [reason: string] }>()
const reason = ref('')
const name = computed(() => props.member?.real_name || props.member?.display_name || props.member?.username || '')

watch(() => props.member?.user_id, () => { reason.value = '' })

function submit() {
  if (!props.member || props.busy || !reason.value.trim()) return
  emit('confirm', reason.value.trim())
}
</script>

<template>
  <DialogRoot :open="Boolean(member)" @update:open="(open) => { if (!open && !busy) emit('close') }">
    <DialogPortal>
      <div v-if="member" class="aw-token-scope aw-account-dialog-layer">
        <DialogOverlay class="aw-account-dialog-backdrop" />
        <DialogContent
          class="aw-account-dialog aw-account-dialog--password"
          @escape-key-down="(event) => { if (busy) event.preventDefault() }"
          @interact-outside="(event) => { if (busy) event.preventDefault() }"
          @close-auto-focus.prevent="emit('closed')"
        >
          <DialogTitle as="h3">停用资产工作台访问</DialogTitle>
          <DialogDescription class="aw-copy">将停用 {{ name }}（账号：{{ member.username }}）的资产工作台访问。保留账号、作品及历史结算记录，运营系统账号状态不变。</DialogDescription>
          <form class="aw-form-grid" :aria-busy="busy" @submit.prevent="submit">
            <label class="aw-form-grid__full">
              停用原因（必填）
              <input v-model="reason" required :maxlength="512" :disabled="busy" placeholder="例如：暂停合作、账号误开" />
            </label>
            <p v-if="error" class="aw-inline-alert aw-form-grid__full" role="alert">{{ error }}</p>
            <p v-if="busy" class="aw-copy aw-form-grid__full" role="status">正在停用，请勿重复操作。</p>
            <div class="aw-account-dialog__footer aw-form-grid__full">
              <button class="aw-secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
              <button class="aw-secondary-button aw-secondary-button--danger" type="submit" :disabled="busy || !reason.trim()">{{ busy ? '停用中…' : '确认停用' }}</button>
            </div>
          </form>
        </DialogContent>
      </div>
    </DialogPortal>
  </DialogRoot>
</template>
