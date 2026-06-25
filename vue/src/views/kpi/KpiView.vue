<template>
  <div class="kpi-view">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">设计师 KPI</h2>
        <span class="page-badge">即将上线</span>
      </div>
    </div>

    <div class="placeholder-card">
      <div class="placeholder-icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
          <path d="M8 36V28M16 36V20M24 36V24M32 36V16M40 36V12" stroke="rgb(var(--yb-text-placeholder))" stroke-width="2.5" stroke-linecap="round"/>
          <path d="M8 28l8-8 8 4 8-8 8-4" stroke="rgb(var(--yb-text-placeholder))" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </div>
      <h3 class="placeholder-title">设计师 KPI 报表（开发中）</h3>
      <p class="placeholder-desc">
        本模块即将上线，将提供以下能力：
      </p>
      <ul class="feature-list">
        <li>设计师任务完成量与按时完成率</li>
        <li>打回率与版本修改次数统计</li>
        <li>运营组横向对比排行</li>
        <li>按时间段（周 / 月 / 季度）筛选导出</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
// P2 placeholder — 无业务逻辑，domain/types/kpi.ts 已预留数据契约
</script>

<style scoped>
.kpi-view {
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
  color: rgb(var(--yb-text-navy));
}

.page-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.625rem;
  font-size: 0.6875rem;
  font-weight: 600;
  background: rgb(var(--yb-surface-slate));
  color: rgb(var(--yb-text-muted-strong));
  border-radius: 999px;
  border: 1px solid rgb(var(--yb-border-slate));
}

.placeholder-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 3rem 2rem;
  background: rgb(var(--yb-surface));
  border: 1px dashed rgb(var(--yb-text-disabled));
  border-radius: 12px;
  text-align: center;
  width: 100%;
  max-width: none;
}

.placeholder-icon {
  color: rgb(var(--yb-text-placeholder));
}

.placeholder-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: rgb(var(--yb-text-slate));
}

.placeholder-desc {
  margin: 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-muted-strong));
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
  color: rgb(var(--yb-text-soft));
  border-bottom: 1px solid rgb(var(--yb-surface-slate));
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
  background: rgb(var(--yb-text-placeholder));
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
  background: rgb(var(--yb-surface-subtle));
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 6px;
  font-size: 0.75rem;
  width: 100%;
}

.api-note-label {
  font-weight: 600;
  color: rgb(var(--yb-text-muted-strong));
  white-space: nowrap;
}

.api-note-code {
  font-family: var(--yb-font-data);
  color: rgb(var(--yb-text-navy));
  word-break: break-all;
}
</style>
