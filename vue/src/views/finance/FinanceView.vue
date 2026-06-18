<template>
  <div class="finance-view">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">财务核算</h2>
        <span class="page-badge">即将上线</span>
      </div>
    </div>

    <div class="placeholder-card">
      <div class="placeholder-icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
          <rect x="4" y="8" width="40" height="32" rx="4" stroke="#94a3b8" stroke-width="2"/>
          <path d="M4 16h40M14 24h6M14 30h10M32 24h6M32 30h4" stroke="#94a3b8" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </div>
      <h3 class="placeholder-title">财务核算模块（开发中）</h3>
      <p class="placeholder-desc">
        本模块即将上线，将提供以下能力：
      </p>
      <ul class="feature-list">
        <li>各运营组任务成本汇总（按时间段 / 任务类型筛选）</li>
        <li>成本价录入与审计追溯</li>
        <li>结单成本 Excel 导出</li>
        <li>核算价 vs 实际成本差异报告</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
// P2 placeholder — 无业务逻辑，domain/types/finance.ts 已预留数据契约
</script>

<style scoped>
.finance-view {
  min-height: 100dvh;
  padding: 1.5rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: #0f172a;
}

.page-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.625rem;
  font-size: 0.6875rem;
  font-weight: 600;
  background: #f1f5f9;
  color: #64748b;
  border-radius: 999px;
  border: 1px solid #e2e8f0;
}

.placeholder-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 3rem 2rem;
  background: #fff;
  border: 1px dashed #cbd5e1;
  border-radius: 12px;
  text-align: center;
  width: 100%;
  max-width: none;
}

.placeholder-icon {
  color: #94a3b8;
}

.placeholder-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: #334155;
}

.placeholder-desc {
  margin: 0;
  font-size: 0.875rem;
  color: #64748b;
}

.feature-list {
  margin: 0;
  padding: 0;
  list-style: none;
  text-align: left;
  width: 100%;
}

.feature-list li {
  padding: 0.375rem 0;
  padding-left: 1.25rem;
  font-size: 0.875rem;
  color: #475569;
  border-bottom: 1px solid #f1f5f9;
  position: relative;
}

.feature-list li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 0.375rem;
  height: 0.375rem;
  background: #94a3b8;
  border-radius: 50%;
}

.feature-list li:last-child {
  border-bottom: none;
}

.api-note {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 0.75rem;
  width: 100%;
}

.api-note-label {
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
}

.api-note-code {
  font-family: var(--yb-font-data);
  color: #0f172a;
  word-break: break-all;
}
</style>
