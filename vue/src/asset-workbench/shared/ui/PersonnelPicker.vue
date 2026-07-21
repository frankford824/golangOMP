<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchProfile } from '@aw/shared/api/assetWorkbenchApi'
import { chipClass, profileStatusMeta, workerTypeMeta } from '@aw/shared/format/status'

const props = withDefaults(defineProps<{
  modelValue: number
  label: string
  hint?: string
  clearable?: boolean
  compact?: boolean
}>(), {
  hint: '按姓名或人员编号查找',
  clearable: false,
  compact: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: number]
  selected: [profile: AssetWorkbenchProfile]
  cleared: []
}>()

const dialogOpen = ref(false)
const query = ref('')
const loading = ref(false)
const error = ref('')
const profiles = ref<AssetWorkbenchProfile[]>([])
const selectedProfile = ref<AssetWorkbenchProfile | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)

const selectedLabel = computed(() => {
  if (selectedProfile.value?.user_id === props.modelValue) {
    return `${selectedProfile.value.real_name || '未填写姓名'} · 编号 ${selectedProfile.value.user_id}`
  }
  return props.modelValue > 0 ? `人员编号 ${props.modelValue}` : '尚未选择人员'
})

watch(() => props.modelValue, (value) => {
  if (selectedProfile.value && selectedProfile.value.user_id !== value) selectedProfile.value = null
})

async function openPicker() {
  dialogOpen.value = true
  query.value = ''
  await loadProfiles()
  await nextTick()
  searchInput.value?.focus()
}

function closePicker() {
  dialogOpen.value = false
  error.value = ''
}

async function loadProfiles() {
  loading.value = true
  error.value = ''
  try {
    const value = query.value.trim()
    const isPersonnelCode = /^\d+$/.test(value)
    const result = await assetWorkbenchApi.listProfiles({
      page: 1,
      page_size: 50,
      ...(value ? (isPersonnelCode ? { user_id: Number(value) } : { q: value }) : {}),
    })
    profiles.value = result.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '人员列表加载失败'
    profiles.value = []
  } finally {
    loading.value = false
  }
}

function selectProfile(profile: AssetWorkbenchProfile) {
  selectedProfile.value = profile
  emit('update:modelValue', profile.user_id)
  emit('selected', profile)
  closePicker()
}

function clearSelection() {
  selectedProfile.value = null
  emit('update:modelValue', 0)
  emit('cleared')
}
</script>

<template>
  <div class="aw-personnel-picker" :class="{ 'aw-personnel-picker--compact': compact }">
    <span class="aw-personnel-picker__label">{{ label }}</span>
    <div class="aw-personnel-picker__field" :class="{ 'aw-personnel-picker__field--selected': modelValue > 0 }">
      <span>
        <strong>{{ selectedLabel }}</strong>
        <small>{{ hint }}</small>
      </span>
      <span class="aw-inline-actions aw-inline-actions--compact">
        <button v-if="clearable && modelValue > 0" class="aw-secondary-button" type="button" @click="clearSelection">清除</button>
        <button class="aw-secondary-button" type="button" @click="openPicker">{{ modelValue > 0 ? '重新选择' : '选择人员' }}</button>
      </span>
    </div>

    <div v-if="dialogOpen" class="aw-dialog-backdrop" role="presentation" @click.self="closePicker">
      <section class="aw-confirm-dialog aw-personnel-picker__dialog" role="dialog" aria-modal="true" :aria-labelledby="`personnel-picker-${label}`" @keydown.esc.prevent="closePicker">
        <div>
          <p class="aw-eyebrow">人员查找</p>
          <h3 :id="`personnel-picker-${label}`">{{ label }}</h3>
          <p class="aw-copy">选择姓名后，系统会自动使用对应人员编码，不需要手工记忆编号。</p>
        </div>

        <form class="aw-personnel-picker__search" @submit.prevent="loadProfiles">
          <input ref="searchInput" v-model="query" type="search" placeholder="输入姓名或人员编号" aria-label="搜索人员" />
          <button class="aw-secondary-button" type="submit" :disabled="loading">{{ loading ? '查找中…' : '查找' }}</button>
        </form>

        <p v-if="error" class="aw-inline-alert aw-inline-alert--warning">{{ error }}</p>
        <div v-else-if="profiles.length" class="aw-personnel-picker__results" role="listbox" aria-label="人员搜索结果">
          <button
            v-for="profile in profiles"
            :key="profile.user_id"
            class="aw-personnel-picker__result"
            type="button"
            role="option"
            :aria-selected="profile.user_id === modelValue"
            @click="selectProfile(profile)"
          >
            <span>
              <strong>{{ profile.real_name || '未填写姓名' }}</strong>
              <small>人员编号 {{ profile.user_id }} · {{ workerTypeMeta(profile.worker_type).label }} · {{ profile.job_grade || '未定级' }}</small>
            </span>
            <span :class="chipClass(profileStatusMeta(profile.status).tone)">{{ profileStatusMeta(profile.status).label }}</span>
          </button>
        </div>
        <p v-else-if="!loading" class="aw-copy">没有找到匹配人员，请检查姓名或编号。</p>

        <div class="aw-inline-actions">
          <button class="aw-secondary-button" type="button" @click="closePicker">取消</button>
        </div>
      </section>
    </div>
  </div>
</template>
