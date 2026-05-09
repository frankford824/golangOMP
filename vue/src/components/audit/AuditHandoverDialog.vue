/**
 * 组件职责：审核交班/转派对话框，选择接手人 + 填写原因 + 确认提交
 * 
 * 核心业务规则（来自 Prompt.md）：
 *   - 审核支持交班/转派/接手
 *   - 转派后原审核员脱离，状态变更需通知
 * 
 * 主要 Store：useAuditStore
 * 预留接口：POST /api/audits/handover (mock)
 * 
 * 当前状态：已迁移 Base 组件，对话框响应式完整
 * 维护注意 / 风险点：
 *   - 转派后状态变更需同步通知相关方
 *   - 弹窗关闭逻辑需完整（取消/确认）
 */
<template>
  <BaseModal
    :model-value="visible"
    title="审核交班"
    confirm-text="确认交班"
    @update:model-value="visible = $event"
    @confirm="onConfirm"
  >
    <div class="form-fields">
      <BaseInput v-model="form.fromUserName" label="原处理人" disabled />
      <BaseSelect
        v-model="form.toUserId"
        label="接手人"
        placeholder="选择接手人"
        :options="toUserOptions"
      />
      <BaseTextarea v-model="form.reason" label="交班原因" :rows="2" placeholder="交班原因" />
      <BaseTextarea v-model="form.judgment" label="当前判断" :rows="2" placeholder="当前判断" />
      <BaseTextarea v-model="form.riskNote" label="风险备注" :rows="2" placeholder="风险备注" />
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import { usersApi } from '@/services/api/usersApi'
import { usePermission } from '@/composables/usePermission'

const props = defineProps<{ modelValue: boolean; fromUserName?: string; taskId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; confirm: [{ toUserId: string; reason: string; judgment: string; riskNote: string }] }>()

const toUserOptions = ref<{ value: string; label: string }[]>([])
const usersLoading = ref(false)
const { canAccessMenu } = usePermission()

const visible = ref(props.modelValue)
const form = reactive({
  fromUserName: props.fromUserName ?? '当前用户',
  toUserId: '',
  reason: '',
  judgment: '',
  riskNote: '',
})

async function loadAuditorOptions() {
  usersLoading.value = true
  try {
    const mayListOrgUsers =
      canAccessMenu('org_admin') || canAccessMenu('user_admin')
    if (!mayListOrgUsers) {
      toUserOptions.value = []
      return
    }
    const res = await usersApi.list({ page: 1, page_size: 500 })
    const data = res?.data as { data?: unknown } | unknown
    const body = data && typeof data === 'object' && 'data' in data ? (data as { data: unknown }).data : data
    const list = Array.isArray(body) ? body : (body as { items?: unknown[] })?.items ?? []
    toUserOptions.value = (list as Record<string, unknown>[]).map((raw) => {
      const id = String(raw.id ?? '')
      const label =
        String(raw.display_name ?? raw.displayName ?? raw.username ?? id) + (id ? ` (#${id})` : '')
      return { value: id, label }
    })
  } catch {
    toUserOptions.value = []
  } finally {
    usersLoading.value = false
  }
}

watch(
  () => props.modelValue,
  (v) => {
    visible.value = v
    if (v) {
      form.fromUserName = props.fromUserName ?? '当前用户'
      form.toUserId = ''
      form.reason = ''
      form.judgment = ''
      form.riskNote = ''
      void loadAuditorOptions()
    }
  },
)
watch(visible, (v) => emit('update:modelValue', v))

function onConfirm() {
  emit('confirm', {
    toUserId: form.toUserId,
    reason: form.reason,
    judgment: form.judgment,
    riskNote: form.riskNote,
  })
  visible.value = false
}
</script>

<style scoped>
.form-fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
