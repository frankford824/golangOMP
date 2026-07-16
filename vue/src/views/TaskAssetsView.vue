<template>
  <main class="task-resources-page">
    <header class="page-head">
      <div>
        <p class="eyebrow">任务 {{ taskId }}</p>
        <h1>任务资源</h1>
        <p>以资源组为唯一口径，不展示历史文件版本和存储生命周期。</p>
      </div>
      <div class="actions">
        <BaseButton variant="secondary" @click="router.push(`/tasks/${taskId}`)">返回任务</BaseButton>
        <BaseButton :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新' }}</BaseButton>
      </div>
    </header>

    <div v-if="error" class="error-banner" role="alert">{{ error }}</div>
    <div v-if="loading && !bundle" class="loading">正在加载资源组…</div>
    <SkuResourceMatrix v-else-if="bundle" :bundle="bundle" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseButton from '@/components/base/BaseButton.vue'
import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue'
import { resourceGroupsApi, type ResourceBundle } from '@/services/api/resourceGroupsApi'

const route = useRoute()
const router = useRouter()
const taskId = computed(() => Number(route.params.id))
const bundle = ref<ResourceBundle | null>(null)
const loading = ref(false)
const error = ref('')

async function load() {
  if (!Number.isInteger(taskId.value) || taskId.value <= 0) {
    error.value = '任务 ID 无效。'
    return
  }
  loading.value = true
  error.value = ''
  try {
    bundle.value = await resourceGroupsApi.taskBundle(taskId.value)
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : '资源组加载失败。'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.task-resources-page { max-width: 1180px; margin: 0 auto; padding: 28px; display: grid; gap: 24px; }
.page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 24px; }
.page-head h1 { margin: 3px 0 8px; font-size: 30px; }
.page-head p { margin: 0; color: rgb(var(--yb-text-muted)); }
.eyebrow { font-size: 12px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
.actions { display: flex; gap: 10px; }
.error-banner { padding: 12px 14px; border-radius: 12px; color: rgb(var(--yb-danger-text)); background: rgb(var(--yb-danger-soft)); }
.loading { padding: 48px; text-align: center; color: rgb(var(--yb-text-muted)); }
@media (max-width: 720px) { .task-resources-page { padding: 18px; } .page-head { flex-direction: column; } }
</style>
