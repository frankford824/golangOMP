<script setup lang="ts">
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import type { PendingWorkDeletion } from './workSelection'

defineProps<{ selection: PendingWorkDeletion | null; busy: boolean }>()
const emit = defineEmits<{ close: []; confirm: [] }>()
</script>

<template>
  <DialogRoot :open="Boolean(selection)" @update:open="(open) => { if (!open && !busy) emit('close') }">
    <DialogPortal>
      <div v-if="selection" class="aw-token-scope aw-account-dialog-layer">
        <DialogOverlay class="aw-account-dialog-backdrop" />
        <DialogContent class="aw-account-dialog aw-account-dialog--password" @escape-key-down="(event) => { if (busy) event.preventDefault() }" @interact-outside="(event) => { if (busy) event.preventDefault() }">
          <DialogTitle as="h3">确认按整件作品删除</DialogTitle>
          <DialogDescription class="aw-copy">将删除 {{ selection.fileIds.length }} 个文件，共 {{ selection.workCount }} 件作品，包括作品内全部子文件夹的文件。</DialogDescription>
          <p v-if="selection.scopeLabel" class="aw-copy">{{ selection.scopeLabel }}</p>
          <p v-if="selection.extraCount" class="aw-inline-alert">为保持计件数量和金额一致，已补齐 {{ selection.extraCount }} 个同作品文件，它们可能在其他页、子文件夹或筛选结果之外。</p>
          <p class="aw-copy">删除原因：{{ selection.reason }}</p>
          <p class="aw-copy">已结算、无权限或补录关联的作品仍会被系统保护。取消不会删除任何文件。</p>
          <div class="aw-account-dialog__footer">
            <button class="aw-secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
            <button class="aw-secondary-button aw-secondary-button--danger" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? '删除中…' : `确认删除 ${selection.fileIds.length} 个文件` }}</button>
          </div>
        </DialogContent>
      </div>
    </DialogPortal>
  </DialogRoot>
</template>
