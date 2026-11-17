<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'
import { formatInt } from '@aw/shared/format/number'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const props = defineProps<{ canReview: boolean; canOpenLedger: boolean; canUpload: boolean }>()
const limit = 6
const request = usePageRequest(async (signal) => {
  const [corrections, pendingReview] = await Promise.all([
    assetWorkbenchApi.listSubmissions({ status: 'needs_fix', page: 1, page_size: limit }, signal),
    props.canReview ? assetWorkbenchApi.listSubmissions({ status: 'submitted', page: 1, page_size: limit }, signal) : Promise.resolve(null),
  ])
  const reviewItems = pendingReview?.items ?? []
  const correctionLimit = Math.max(limit / 2, limit - reviewItems.length)
  const correctionItems = corrections.items.slice(0, correctionLimit)
  return {
    total: corrections.total + (pendingReview?.total ?? 0),
    items: [...correctionItems, ...reviewItems.slice(0, limit - correctionItems.length)],
  }
})
const tasks = request.data
const loading = request.loading
const loadError = computed(() => request.error.value ? '待处理事项读取失败，请重试。' : '')
onMounted(request.run)
</script>

<template>
  <section class="aw-panel aw-submission-tasks" aria-labelledby="aw-submission-tasks-title">
    <header class="aw-panel__head">
      <div>
        <h2 id="aw-submission-tasks-title">待处理事项</h2>
        <p v-if="!loading && !loadError && tasks" class="aw-copy">
          {{ tasks.total > 0 ? '共 ' + formatInt(tasks.total) + ' 项' : '当前没有待处理提交' }}
          <template v-if="tasks.total > tasks.items.length"> · 显示 {{ tasks.items.length }} 项</template>
        </p>
      </div>
      <div class="aw-inline-actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="request.run">刷新</button>
        <RouterLink v-if="canReview && canOpenLedger" class="aw-primary-button" to="/upload-overview">查看上传总览</RouterLink>
        <RouterLink v-else-if="canUpload" class="aw-primary-button" to="/upload">交作品</RouterLink>
      </div>
    </header>
    <AsyncBoundary :loading="loading" :error="loadError" loading-label="正在读取待处理事项" @retry="request.run">
      <ul v-if="tasks?.items.length" class="aw-submission-tasks__list">
        <li v-for="item in tasks.items" :key="item.id">
          <component
            :is="canOpenLedger ? RouterLink : 'div'"
            :to="canOpenLedger ? { path: '/upload-overview', query: { q: item.submission_no } } : undefined"
            class="aw-submission-tasks__row"
          >
            <span class="aw-chip" :class="item.status === 'needs_fix' ? 'aw-chip--danger' : 'aw-chip--warn'">
              {{ item.status === 'needs_fix' ? '需修正' : '待质检' }}
            </span>
            <strong>{{ item.submission_no }}</strong>
            <span v-if="canReview" class="aw-submission-tasks__owner">{{ item.submitter_name || item.submitter_username || '用户 ' + item.submitter_user_id }}</span>
            <span class="aw-copy">{{ formatInt(item.item_count) }} 件 · {{ formatShanghaiDateTime(item.submitted_at) }}</span>
            <span v-if="canOpenLedger" class="aw-submission-tasks__action">查看提交</span>
          </component>
        </li>
      </ul>
      <p v-else class="aw-submission-tasks__empty aw-copy">
        {{ canReview ? '有待质检或需修正的提交时，会列在这里。' : '需要修正的作品会列在这里。' }}
      </p>
    </AsyncBoundary>
  </section>
</template>
