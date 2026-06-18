<template>
  <div class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th class="col-order">定制单号</th>
          <th class="col-task">关联任务号</th>
          <th class="col-sku">SKU</th>
          <th class="col-product">产品名称</th>
          <th class="col-type">定制类型</th>
          <th class="col-supplier">供应方</th>
          <th class="col-status">状态</th>
          <th class="col-created">创建时间</th>
          <th class="col-returned">回传时间</th>
          <th class="col-review">复核结果</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in orders"
          :key="row.id"
          class="row-click"
          @click="$emit('select', row)"
        >
          <td class="cell-mono">{{ row.orderNo }}</td>
          <td class="cell-mono">{{ row.taskNo }}</td>
          <td class="cell-mono">{{ row.sku }}</td>
          <td class="cell-product" :title="row.productName">{{ row.productName }}</td>
          <td>{{ row.outsourceType }}</td>
          <td>{{ row.supplierName }}</td>
          <td>{{ statusLabel(row.status) }}</td>
          <td class="col-created-cell">{{ formatDate(row.createdAt) }}</td>
          <td class="col-returned-cell">
            {{ row.returnedAt ? formatDate(row.returnedAt) : '-' }}
          </td>
          <td class="col-review-cell">
            {{ row.reviewResult === 'passed' ? '通过' : row.reviewResult === 'rejected' ? '打回' : '-' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { OutsourceOrder, OutsourceOrderStatus } from '@/types'
import { formatDateBeijing } from '@/utils/date'

defineProps<{ orders: OutsourceOrder[] }>()
defineEmits<{ select: [OutsourceOrder] }>()

function statusLabel(s: OutsourceOrderStatus) {
  const m: Record<OutsourceOrderStatus, string> = {
    draft: '草稿',
    sent: '已发送',
    in_progress: '处理中',
    returned: '已回传',
    reviewing: '复核中',
    review_passed: '复核通过',
    review_rejected: '复核打回',
    closed: '已关闭',
  }
  return m[s] ?? s
}

function formatDate(iso: string) {
  return formatDateBeijing(iso)
}
</script>

<style scoped>
.table-wrap {
  width: 100%;
  max-width: 100%;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
  table-layout: auto;
}
.data-table th,
.data-table td {
  padding: 0.45rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid #e2e8f0;
  line-height: 1.4;
}
.data-table th {
  background: #f8fafc;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #64748b;
}
.data-table tbody tr:nth-child(even) {
  background: #fafafa;
}
.row-click {
  cursor: pointer;
}
.row-click:hover {
  background: #f1f5f9 !important;
}
.cell-product {
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-mono {
  white-space: nowrap;
  font-family: var(--yb-font-data);
  font-weight: 600;
  color: #0f172a;
}
</style>
