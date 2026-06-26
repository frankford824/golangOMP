<template>
  <section class="aw-page-stack">
    <div class="aw-page-heading">
      <div>
        <p class="aw-eyebrow">Monthly piecework baseline</p>
        <h2>标准人月聚合 + 每员工两条工资行</h2>
      </div>
      <span class="aw-status-pill aw-status-pill--submitted">工资条口径已接入</span>
    </div>

    <div class="aw-metric-grid">
      <article class="aw-metric-card">
        <span>待提交文件</span>
        <strong>0</strong>
        <small>上传会话、item/files 与预览 worker 已接入</small>
      </article>
      <article class="aw-metric-card">
        <span>待补价明细</span>
        <strong>0</strong>
        <small>未定级或未命中价目矩阵</small>
      </article>
      <article class="aw-metric-card">
        <span>本月结算净额</span>
        <strong class="aw-money">0.00</strong>
        <small>正常计件工资 + 补录计件工资</small>
      </article>
    </div>

    <div class="aw-two-column">
      <section class="aw-panel">
        <h3>实施顺序</h3>
        <ol class="aw-ordered-list">
          <li>账户档案与岗级历史</li>
          <li>价目矩阵、扣减、福利、大促券</li>
          <li>上传会话、提交批次、item/files</li>
          <li>出错导入、两条工资行与补录权限</li>
        </ol>
      </section>
      <section class="aw-panel aw-panel--stage">
        <h3>接触印相评审区</h3>
        <div class="aw-contact-sheet">
          <span v-for="index in 12" :key="index" />
        </div>
      </section>
    </div>
  </section>
</template>
