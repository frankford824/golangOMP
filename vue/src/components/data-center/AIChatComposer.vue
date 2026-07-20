<template>
  <form class="chat-composer" @submit.prevent="submit">
    <label class="sr-only" for="ai-chat-question">向数据助手提问</label>
    <textarea
      id="ai-chat-question"
      ref="textarea"
      v-model="draft"
      rows="1"
      :maxlength="maxLength"
      :disabled="disabled"
      placeholder="询问当前可访问的任务、资源与经营数据…"
      @input="resize"
      @keydown.enter.exact.prevent="submit"
    />
    <div class="composer-actions">
      <span v-if="draft.length > maxLength * 0.8" class="input-count">{{ draft.length }}/{{ maxLength }}</span>
      <button
        v-if="streaming"
        type="button"
        class="stop-button"
        aria-label="停止生成"
        @click="$emit('stop')"
      >
        <Square aria-hidden="true" />
        <span>停止生成</span>
      </button>
      <button
        v-else
        type="submit"
        class="send-button"
        aria-label="发送问题"
        :disabled="disabled || !draft.trim()"
      >
        <ArrowUp aria-hidden="true" />
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { ArrowUp, Square } from 'lucide-vue-next'

withDefaults(defineProps<{ maxLength?: number; disabled?: boolean; streaming?: boolean }>(), {
  maxLength: 4000,
  disabled: false,
  streaming: false,
})
const emit = defineEmits<{ (event: 'submit', value: string): void; (event: 'stop'): void }>()

const draft = ref('')
const textarea = ref<HTMLTextAreaElement | null>(null)

function resize() {
  if (!textarea.value) return
  textarea.value.style.height = 'auto'
  textarea.value.style.height = `${Math.min(textarea.value.scrollHeight, 156)}px`
}

function submit() {
  const value = draft.value.trim()
  if (!value) return
  emit('submit', value)
  draft.value = ''
  void nextTick(resize)
}

function focus() {
  textarea.value?.focus()
}

defineExpose({ focus })
</script>

<style scoped>
.chat-composer{display:flex;min-height:4.35rem;align-items:flex-end;gap:.75rem;border:1px solid rgb(var(--yb-border-form-strong));border-radius:1rem;background:rgb(var(--yb-surface));padding:.7rem .75rem .7rem 1rem;box-shadow:0 12px 32px rgb(var(--yb-shadow)/.08);transition:border-color .16s ease,box-shadow .16s ease}.chat-composer:focus-within{border-color:rgb(var(--yb-brand-border-strong));box-shadow:0 14px 34px rgb(var(--yb-brand)/.1),0 0 0 3px rgb(var(--yb-brand)/.06)}textarea{min-height:2.2rem;max-height:9.75rem;flex:1;resize:none;border:0;background:transparent;padding:.45rem 0;color:rgb(var(--yb-text));font:inherit;font-size:.82rem;line-height:1.55;outline:0}textarea::placeholder{color:rgb(var(--yb-text-placeholder))}.composer-actions{display:flex;flex:0 0 auto;align-items:center;gap:.55rem}.input-count{color:rgb(var(--yb-text-faint));font-size:.64rem}.send-button,.stop-button{display:inline-flex;min-height:2.45rem;align-items:center;justify-content:center;border-radius:.7rem;font:inherit;cursor:pointer}.send-button{width:2.45rem;border:1px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));box-shadow:0 6px 15px rgb(var(--yb-brand)/.2)}.send-button:disabled{border-color:rgb(var(--yb-border));background:rgb(var(--yb-surface-control));color:rgb(var(--yb-text-disabled-strong));box-shadow:none;cursor:not-allowed}.stop-button{gap:.42rem;border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft));padding:0 .78rem;color:rgb(var(--yb-text-secondary));font-size:.72rem;font-weight:720}.send-button svg{width:1.05rem;height:1.05rem}.stop-button svg{width:.78rem;height:.78rem;fill:currentColor}.send-button:focus-visible,.stop-button:focus-visible{outline:2px solid rgb(var(--yb-brand));outline-offset:2px}@media(max-width:640px){.chat-composer{min-height:4.8rem;padding:.62rem .62rem .62rem .8rem}.stop-button span{display:none}.stop-button{width:2.45rem;padding:0}.input-count{display:none}}
</style>
