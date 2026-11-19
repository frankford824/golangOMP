<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import type { FeeConfirmation, ConfirmationResult } from '../useFeeConfirm'

const props = defineProps<{ config?: FeeConfirmation }>()
const emit = defineEmits<{ finish: [result: ConfirmationResult] }>()
const reason = ref('')
watch(() => props.config, () => { reason.value = '' })
function confirm() {
  if (!props.config || (props.config.reasonLabel && !reason.value.trim())) return
  emit('finish', { reason: reason.value.trim() })
}
</script>
<template>
  <BaseModal :model-value="!!config" :title="config?.title" panel-max-width="460px" @cancel="emit('finish', null)">
    <div v-if="config" class="fee-confirm">
      <p v-if="config.message">{{ config.message }}</p>
      <dl v-if="config.facts?.length">
        <div v-for="[key, value] in config.facts" :key="key"><dt>{{ key }}</dt><dd>{{ value || '—' }}</dd></div>
      </dl>
      <label v-if="config.reasonLabel">{{ config.reasonLabel }}<textarea v-model="reason" required rows="3" maxlength="2000" /></label>
    </div>
    <template #footer><footer class="fee-confirm-actions">
      <button type="button" @click="emit('finish', null)">取消</button>
      <button type="button" :class="{ danger: config?.danger }" :disabled="!!config?.reasonLabel && !reason.trim()" @click="confirm">{{ config?.confirmText }}</button>
    </footer></template>
  </BaseModal>
</template>
<style scoped>
.fee-confirm { display: grid; gap: 12px; }
.fee-confirm dl { margin:0; padding:12px; border:1px solid var(--yb-line-structure); border-radius:8px; }
.fee-confirm dl div { display:grid; grid-template-columns: 6em 1fr; gap:8px; padding:4px; }
.fee-confirm dt { color:var(--oa-secondary); }
.fee-confirm dd { margin:0; font-weight:600; overflow-wrap:anywhere; }
.fee-confirm label { display:grid; gap:6px; }
.fee-confirm textarea,.fee-confirm-actions button { border:1px solid var(--yb-line-control); border-radius:6px; padding:6px 12px; }
.fee-confirm-actions { display:flex; gap:8px; justify-content:flex-end; padding:12px 20px; }
.fee-confirm-actions button:last-child { background:var(--oa-accent); color:rgb(var(--yb-text-inverse)); }
.fee-confirm-actions button.danger { background:rgb(var(--oa-danger-rgb)); }
.fee-confirm-actions button:disabled { opacity:.5; cursor:not-allowed; }
</style>
