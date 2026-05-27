<template>
  <section class="retouch-requirements-block">
    <p class="block-kicker">P 图需求明细</p>
    <article v-for="(item, index) in requirements" :key="item.id || index" class="requirement-card">
      <header class="requirement-card-head">
        <span class="requirement-no">需求 {{ index + 1 }}</span>
      </header>
      <p class="requirement-desc">{{ item.description }}</p>
      <dl v-if="hasOptionalFields(item)" class="requirement-meta">
        <div v-if="item.skuCode">
          <dt>SKU / 款号</dt>
          <dd>{{ item.skuCode }}</dd>
        </div>
        <div v-if="item.spec">
          <dt>规格</dt>
          <dd>{{ item.spec }}</dd>
        </div>
        <div v-if="item.remark">
          <dt>备注</dt>
          <dd>{{ item.remark }}</dd>
        </div>
      </dl>
    </article>
  </section>
</template>

<script setup lang="ts">
import type { RetouchRequirement } from '@/domain/types/retouch-requirement'

defineProps<{
  requirements: RetouchRequirement[]
}>()

function hasOptionalFields(item: RetouchRequirement): boolean {
  return Boolean(item.skuCode?.trim() || item.spec?.trim() || item.remark?.trim())
}
</script>

<style scoped>
.retouch-requirements-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.block-kicker {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-secondary, #64748b);
}

.requirement-card {
  padding: 12px 14px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  background: #f8fafc;
}

.requirement-card-head {
  margin-bottom: 6px;
}

.requirement-no {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.requirement-desc {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-primary, #0f172a);
  white-space: pre-wrap;
}

.requirement-meta {
  margin: 10px 0 0;
  display: grid;
  gap: 6px;
}

.requirement-meta div {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 8px;
  font-size: 13px;
}

.requirement-meta dt {
  margin: 0;
  color: var(--text-secondary, #64748b);
}

.requirement-meta dd {
  margin: 0;
  color: var(--text-primary, #0f172a);
}
</style>
