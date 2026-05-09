<template>
  <BaseModal
    :model-value="visible"
    title="创建定制单"
    confirm-text="提交"
    @update:model-value="visible = $event"
    @confirm="onConfirm"
  >
    <div class="form-fields">
      <BaseInput v-model="form.vendor_name" label="供应商名称" placeholder="必填" />
      <BaseInput v-model="form.outsource_type" label="定制类型" placeholder="如：二创 / 打样" />
      <BaseTextarea v-model="form.delivery_requirement" label="交付要求" :rows="2" placeholder="可选" />
      <BaseTextarea v-model="form.settlement_note" label="结算说明" :rows="2" placeholder="可选" />
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [boolean]
  confirm: [
    {
      vendor_name: string
      outsource_type: string
      delivery_requirement?: string
      settlement_note?: string
    },
  ]
}>()

const visible = ref(props.modelValue)
const form = reactive({
  vendor_name: '',
  outsource_type: '',
  delivery_requirement: '',
  settlement_note: '',
})

const canSubmit = computed(
  () => form.vendor_name.trim().length > 0 && form.outsource_type.trim().length > 0,
)

watch(
  () => props.modelValue,
  (v) => {
    visible.value = v
    if (v) {
      form.vendor_name = ''
      form.outsource_type = ''
      form.delivery_requirement = ''
      form.settlement_note = ''
    }
  },
)
watch(visible, (v) => emit('update:modelValue', v))

function onConfirm() {
  if (!canSubmit.value) return
  emit('confirm', {
    vendor_name: form.vendor_name.trim(),
    outsource_type: form.outsource_type.trim(),
    delivery_requirement: form.delivery_requirement.trim() || undefined,
    settlement_note: form.settlement_note.trim() || undefined,
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
