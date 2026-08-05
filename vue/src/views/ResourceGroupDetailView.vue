<template>
  <main class="detail-page">
    <nav class="breadcrumb" aria-label="当前位置"><button @click="router.push('/asset-center')">资产中心</button><span>/</span><span>{{ displaySKU }}</span></nav>
    <header class="detail-hero">
      <div>
        <div class="hero-badges"><span>{{ modeLabel }}</span><span v-if="group?.business_lane">{{ group.business_lane === 'customization' ? '定制' : '常规' }}</span></div>
        <p class="eyebrow">{{ displaySKU }}</p>
        <h1>{{ profile?.product_name || group?.product_name || 'SKU 资源详情' }}</h1>
        <p>当前生效资源已按运营参考、设计源文件和最终成品三个阶段整理。</p>
      </div>
      <div class="hero-actions">
        <button v-if="group" class="secondary" @click="router.push(`/tasks/${group.task_id}`)">查看来源任务</button>
        <button class="secondary" :disabled="loading" @click="load">{{ loading ? '刷新中…' : '刷新' }}</button>
        <button class="primary" :disabled="downloading || !group" @click="downloadAll">{{ downloading ? '准备中…' : '下载全部成品' }}</button>
      </div>
    </header>

    <section v-if="group" class="provenance-strip" aria-label="资源来源">
      <div><span>来源任务</span><strong>{{ group.task_no || group.task_id }}</strong></div>
      <div><span>创建人</span><strong>{{ group.creator_name || '—' }}</strong></div>
      <div><span>交付方式</span><strong>{{ modeLabel }}</strong></div>
      <div><span>最终成品</span><strong>{{ finalCount }} 张</strong></div>
    </section>

    <section v-if="group" class="sku-business-panel" aria-label="SKU 业务信息">
      <header>
        <div><p class="eyebrow">SKU 业务档案</p><h2>图片、规格与成本来自同一个 SKU</h2></div>
        <span class="sync-state" :class="syncTone">{{ syncLabel }}</span>
      </header>
      <div class="business-grid">
        <article><span>款式编码</span><strong>{{ profile?.product_i_id || profile?.erp_i_id || '待关联' }}</strong><small v-if="comboCodes">组合编码：{{ comboCodes }}</small></article>
        <article><span>规格与尺寸</span><strong>{{ profile?.size_text || profile?.spec_text || '待补充' }}</strong><small>{{ dimensionDetail }}</small></article>
        <article><span>计价面积</span><strong>{{ areaLabel }}</strong><small>{{ profile?.area_trace?.formula || profile?.area_trace?.warning || '暂无可解释的面积计算过程' }}</small></article>
        <article class="cost-card"><span>当前成本</span><strong>{{ costLabel }}</strong><small>{{ costRuleLabel }}</small></article>
      </div>
    </section>

    <CostExplanationPanel
      v-if="group"
      title="当前 SKU 成本规则试算与解释"
      :seed="costPreviewSeed"
      :task-id="group.task_id"
      :task-sku-item-id="group.task_sku_item_id || undefined"
      :asset-id="group.id"
      :resource-id="String(group.id)"
      :sku-code="displaySKU"
    />

    <div v-if="error" class="state-card error" role="alert">{{ error }}</div>
    <div v-if="loading && !group" class="state-card">正在加载当前有效资源…</div>
    <SkuResourceMatrix v-else-if="group" :bundle="{ task_id: group.task_id, workflow_revision: 0, groups: [group] }" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue'
import CostExplanationPanel from '@/components/cost/CostExplanationPanel.vue'
import { resourceGroupsApi, type ResourceGroup } from '@/services/api/resourceGroupsApi'
import { downloadBatchAsZip } from '@/utils/batchZipDownload'

const route = useRoute()
const router = useRouter()
const group = ref<ResourceGroup | null>(null)
const loading = ref(false)
const downloading = ref(false)
const error = ref('')
const groupId = computed(() => Number(route.params.id))
const activeRevision = computed(() => group.value?.finalized_revision || group.value?.working_revision)
const modeLabel = computed(() => activeRevision.value?.mode === 'set' ? '套装资源' : '单图资源')
const finalCount = computed(() => activeRevision.value?.items?.length || 0)
const profile = computed(() => group.value?.sku_profile || null)
const displaySKU = computed(() => group.value?.sku_code || profile.value?.sku_code || 'SKU 待关联')
const comboCodes = computed(() => (profile.value?.combo_sku_codes || []).join('、'))
const areaLabel = computed(() => typeof profile.value?.area_trace?.area_m2 === 'number' ? `${profile.value.area_trace.area_m2.toFixed(3)} ㎡` : '面积待核对')
const costLabel = computed(() => typeof profile.value?.cost_price === 'number' ? `¥ ${profile.value.cost_price.toFixed(2)}` : '成本待计算')
const costRuleLabel = computed(() => profile.value?.cost_trace?.rule_name ? `按「${profile.value.cost_trace.rule_name}」计算${profile.value.cost_trace.matched_rule_version ? ` · 第 ${profile.value.cost_trace.matched_rule_version} 版` : ''}` : '尚未关联成本规则')
const dimensionDetail = computed(() => {
  const trace = profile.value?.area_trace
  if (typeof trace?.width_m === 'number' && typeof trace?.height_m === 'number') return `${trace.width_m} m × ${trace.height_m} m${typeof trace.quantity === 'number' ? ` × ${trace.quantity} 件` : ''}`
  return trace?.source_label || '尺寸来源待补充'
})
const syncLabel = computed(() => ({ synced: 'ERP 已同步', queued: '等待同步', syncing: '正在同步', failed: '同步失败', cooling_down: '稍后重试', pending_sync: '待同步' }[profile.value?.erp_sync_status || ''] || 'ERP 待关联'))
const syncTone = computed(() => profile.value?.erp_sync_status === 'synced' ? 'is-synced' : profile.value?.erp_sync_status === 'failed' ? 'is-failed' : 'is-pending')
const costPreviewSeed = computed(() => ({
  categoryCode: profile.value?.category_name || '',
  productIID: profile.value?.product_i_id || '',
  erpIID: profile.value?.erp_i_id || '',
  width: profile.value?.area_trace?.width_m,
  height: profile.value?.area_trace?.height_m,
  area: profile.value?.area_trace?.area_m2,
  quantity: profile.value?.area_trace?.quantity,
  notes: [profile.value?.spec_text, profile.value?.size_text].filter(Boolean).join(' '),
  currentCost: profile.value?.cost_price,
  currentRuleName: profile.value?.cost_trace?.rule_name,
  currentRuleVersion: profile.value?.cost_trace?.matched_rule_version,
  requiresManualReview: profile.value?.cost_trace?.requires_manual_review,
}))

async function load() {
  loading.value = true
  error.value = ''
  try { group.value = await resourceGroupsApi.get(groupId.value) }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '资源详情加载失败。' }
  finally { loading.value = false }
}
async function downloadAll() {
  if (!group.value) return
  downloading.value = true
  error.value = ''
  try {
    const result = await resourceGroupsApi.batchDownload([group.value.id])
    const items = [...result.items].sort((a, b) => a.sort_order - b.sort_order)
    const zipResult = await downloadBatchAsZip({
      items: items.map((item) => ({
        key: String(item.revision_item_id),
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `成品-${item.sort_order + 1}`,
      })),
      zipFilename: `${displaySKU.value}-${activeRevision.value?.mode === 'set' ? '套装成品' : '最终成品'}.zip`,
      normalizeNestedZipFilenames: true,
    })
    if (zipResult.failureCount) {
      error.value = `ZIP 已生成，但有 ${zipResult.failureCount} 个文件下载失败；失败明细已写入压缩包。`
    }
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成品打包下载失败。' }
  finally { downloading.value = false }
}
onMounted(load)
</script>

<style scoped>
.detail-page{max-width:1320px;margin:0 auto;padding:28px;display:grid;gap:20px}.breadcrumb{display:flex;align-items:center;gap:8px;color:rgb(var(--yb-text-muted));font-size:12px}.breadcrumb button{padding:0;border:0;background:transparent;color:rgb(var(--yb-brand));cursor:pointer}.detail-hero{display:flex;align-items:flex-start;justify-content:space-between;gap:22px;padding:22px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface))}.detail-hero h1{margin:4px 0;font-size:30px}.detail-hero p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{color:rgb(var(--yb-brand));font-size:12px;font-weight:900;letter-spacing:.08em}.hero-badges,.hero-actions{display:flex;gap:8px;flex-wrap:wrap}.hero-badges span{padding:5px 8px;border-radius:999px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-size:11px;font-weight:800}.hero-actions button{min-height:40px;padding:0 14px;border-radius:10px;cursor:pointer}.primary{border:1px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.provenance-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface));overflow:hidden}.provenance-strip div{display:grid;gap:4px;padding:14px 17px}.provenance-strip div+div{border-left:1px solid rgb(var(--yb-border))}.provenance-strip span{color:rgb(var(--yb-text-muted));font-size:11px}.provenance-strip strong{font-size:13px}.state-card{padding:34px;text-align:center;border:1px dashed rgb(var(--yb-border));border-radius:14px;color:rgb(var(--yb-text-muted))}.state-card.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:760px){.detail-page{padding:16px}.detail-hero{flex-direction:column}.provenance-strip{grid-template-columns:1fr 1fr}.provenance-strip div+div{border-left:0}.provenance-strip div:nth-child(even){border-left:1px solid rgb(var(--yb-border))}.provenance-strip div:nth-child(n+3){border-top:1px solid rgb(var(--yb-border))}.hero-actions{width:100%}.hero-actions button{flex:1}}
</style>

<style scoped>
.sku-business-panel{display:grid;gap:1rem;padding:1.15rem;border:1px solid rgb(var(--yb-border));border-radius:1.1rem;background:rgb(var(--yb-surface))}.sku-business-panel>header{display:flex;align-items:flex-start;justify-content:space-between;gap:1rem}.sku-business-panel h2{margin:.2rem 0 0;font-size:1.05rem}.sync-state{border-radius:999px;padding:.38rem .65rem;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-muted));font-size:.72rem;font-weight:800}.sync-state.is-synced{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.sync-state.is-failed{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.business-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:.9rem;background:rgb(var(--yb-surface-soft))}.business-grid article{min-width:0;display:grid;align-content:start;gap:.35rem;padding:.9rem}.business-grid article+article{border-left:1px solid rgb(var(--yb-border))}.business-grid span,.business-grid small{color:rgb(var(--yb-text-muted));font-size:.68rem}.business-grid strong{overflow:hidden;font-size:.9rem;text-overflow:ellipsis}.business-grid small{line-height:1.45}.cost-card strong{color:rgb(var(--yb-brand));font-size:1.05rem}@media(max-width:900px){.business-grid{grid-template-columns:1fr 1fr}.business-grid article:nth-child(3){border-left:0}.business-grid article:nth-child(n+3){border-top:1px solid rgb(var(--yb-border))}}@media(max-width:560px){.business-grid{grid-template-columns:1fr}.business-grid article+article{border-left:0;border-top:1px solid rgb(var(--yb-border))}}
</style>
