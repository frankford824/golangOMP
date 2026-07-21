/**
 * 组件职责：产品选择弹窗，用于原有产品绑定 ERP 已有 SKU
 *
 * 核心业务规则（来自 Prompt.md）：
 *   - 原有产品必须选择 ERP 已有产品并绑定 SKU（不可手填）
 *
 * 主要 Store：useTaskStore 或 useProductsStore
 * 数据来源：GET /v1/erp/products
 *
 * 当前状态：已迁移 Base 组件，搜索/选择交互完整
 * 维护注意 / 风险点：
 *   - 绑定后 SKU 不可修改
 *   - 弹窗关闭需返回选中 SKU
 */
<template>
  <BaseModal
    :model-value="visible"
    title="选择已有产品（ERP）"
    confirm-text="确定"
    :show-confirm="true"
    @update:model-value="visible = $event"
    @confirm="confirmSelect"
    @cancel="handleClose"
  >
    <div class="picker-body">
      <BaseInput
        v-model="keyword"
        placeholder="按产品名称 / SKU 关键字搜索（当前仅支持关键字模糊匹配）"
        class="search-input"
      />
      <p v-if="productsStore.searchError" class="search-error">{{ productsStore.searchError }}</p>
      <div v-if="productsStore.loading" class="loading-hint">加载中…</div>
      <div v-else class="table-scroll">
        <table class="data-table" v-if="productsStore.total > 0">
          <thead>
            <tr>
              <th>图片</th>
              <th>产品名称</th>
              <th>SKU</th>
              <th>分类</th>
              <th>规格</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in filteredProducts"
              :key="row.id"
              class="row-click"
              :class="{ selected: currentRow?.id === row.id }"
              @click="selectRow(row)"
            >
              <td>
                <div class="thumb">
                  <img
                    v-if="row.imageUrl"
                    :src="row.imageUrl"
                    alt="产品图片"
                    class="thumb-img"
                    loading="lazy"
                    decoding="async"
                    @error="onImageError($event)"
                  />
                  <div v-else class="thumb-placeholder">无图</div>
                </div>
              </td>
              <td class="ellipsis" :title="row.name">{{ row.name || '未命名产品' }}</td>
              <td class="ellipsis">{{ row.sku || '无 SKU' }}</td>
              <td class="ellipsis" :title="row.category || row.categoryCode">
                {{ row.category || row.categoryCode || '未分类' }}
              </td>
              <td class="ellipsis" :title="row.spec">{{ row.spec || '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty-state">
          <p class="empty-text">未找到匹配的 ERP 产品，请调整关键字重试。</p>
        </div>
      </div>
      <div class="pagination-row" v-if="productsStore.totalPages > 1">
        <span class="pagination-text">
          第 {{ productsStore.page }} / {{ productsStore.totalPages }} 页，
          共 {{ productsStore.total }} 条
        </span>
        <div class="pagination-buttons">
          <BaseButton
            size="sm"
            variant="secondary"
            :disabled="productsStore.page <= 1 || productsStore.loading"
            @click="goPrevPage"
          >
            上一页
          </BaseButton>
          <BaseButton
            size="sm"
            variant="secondary"
            :disabled="productsStore.page >= productsStore.totalPages || productsStore.loading"
            @click="goNextPage"
          >
            下一页
          </BaseButton>
        </div>
      </div>
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount, watch } from 'vue'
import type { ERPProductOption } from '@/types'
import { useProductsStore } from '@/stores/products'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; select: [ERPProductOption] }>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const productsStore = useProductsStore()
const keyword = ref('')
const currentRow = ref<ERPProductOption | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | null = null
const DEBOUNCE_MS = 300

const filteredProducts = computed(() => productsStore.list)

watch(visible, (v) => {
  if (v) {
    currentRow.value = null
    keyword.value = ''
    productsStore.loadProducts({ page: 1 })
  }
})

watch(keyword, (kw) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  const trimmed = kw.trim()
  debounceTimer = setTimeout(() => {
    productsStore.loadProducts({ keyword: trimmed || undefined, page: 1 })
    debounceTimer = null
  }, DEBOUNCE_MS)
})

onBeforeUnmount(() => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
})

function selectRow(row: ERPProductOption) {
  currentRow.value = row
}

function confirmSelect() {
  if (currentRow.value) {
    emit('select', currentRow.value)
    visible.value = false
  }
}

function handleClose() {
  visible.value = false
}

function onImageError(event: Event) {
  const target = event.target as HTMLImageElement | null
  if (!target) return
  const wrapper = target.parentElement
  if (wrapper) {
    target.remove()
    if (!wrapper.querySelector('.thumb-placeholder')) {
      const placeholder = document.createElement('div')
      placeholder.className = 'thumb-placeholder'
      placeholder.textContent = '无图'
      wrapper.appendChild(placeholder)
    }
  }
}

function goPrevPage() {
  if (productsStore.page <= 1 || productsStore.loading) return
  productsStore.loadProducts({ keyword: keyword.value.trim() || undefined, page: productsStore.page - 1 })
}

function goNextPage() {
  if (productsStore.page >= productsStore.totalPages || productsStore.loading) return
  productsStore.loadProducts({ keyword: keyword.value.trim() || undefined, page: productsStore.page + 1 })
}
</script>

<style scoped>
.picker-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.search-input {
  width: 100%;
}
.search-error {
  margin: 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-danger-text));
}
.loading-hint {
  font-size: 0.875rem;
  color: rgb(var(--yb-text-muted-strong));
}
.table-scroll {
  max-height: 360px;
  overflow: auto;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
}
.thumb {
  width: 56px;
  height: 56px;
  border-radius: 6px;
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-slate));
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.thumb-placeholder {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-placeholder));
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.data-table th,
.data-table td {
  padding: 0.5rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid rgb(var(--yb-border-slate));
}
.data-table td {
  color: rgb(var(--yb-text-body));
}
.data-table th {
  background: rgb(var(--yb-surface-subtle));
  font-weight: 600;
  color: rgb(var(--yb-text-slate));
}
.row-click {
  cursor: pointer;
}
.row-click:hover,
.row-click.selected {
  background: rgb(var(--yb-success-ui-soft));
}
.row-click.selected {
  box-shadow: inset 3px 0 0 rgb(var(--yb-success-emerald-bright));
}
.ellipsis {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pagination-row {
  margin-top: 0.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-muted-strong));
}
.pagination-buttons {
  display: flex;
  gap: 0.5rem;
}
.empty-state {
  padding: 1.5rem 0.5rem;
  text-align: center;
}
.empty-text {
  margin: 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-placeholder));
}
</style>
