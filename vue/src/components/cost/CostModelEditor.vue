<template>
  <section class="model-editor" aria-label="统一计价方案">
    <div class="fields">
      <label>材质<input v-model.trim="draft.material" @input="update" placeholder="例如：覆膜KT板" /></label>
      <label>计费方式<select v-model="draft.basis" @change="update"><option value="area">按面积</option><option value="piece">按件</option><option value="set">按套</option><option value="manual">人工报价</option></select></label>
      <template v-if="draft.basis !== 'manual'">
        <label>基础单价<input v-model.number="draft.unit_price" type="number" min="0" step="any" @input="update" /></label>
        <label>基础价格系数<input v-model.number="draft.multiplier" type="number" min="0.0001" step="any" @input="update" /></label>
        <label>最低计费量<input v-model.number="draft.minimum" type="number" min="0" step="any" @input="update" /></label>
        <template v-if="draft.basis === 'area'"><label>小面积阈值（㎡，0表示无）<input v-model.number="draft.small_area_threshold" type="number" min="0" step="any" @input="update" /></label><label>低于阈值每㎡加价<input v-model.number="draft.small_area_surcharge" type="number" min="0" step="any" @input="update" /></label></template>
      </template>
    </div>
    <p class="formula">{{draft.basis==='manual'?'人工报价':`计费量 × 单价 × ${draft.multiplier} + 工艺费用（元 / SKU）`}}</p>
    <template v-if="draft.basis !== 'manual'">
      <h3>厚度价格档位 <small>可选；配置后必须精确命中厚度</small></h3>
      <div v-for="(p,i) in draft.thickness_prices" :key="i" class="row"><label>厚度（mm）<input v-model.number="p.thickness_mm" type="number" min="0.001" step="any" @input="update" /></label><label>对应单价<input v-model.number="p.unit_price" type="number" min="0" step="any" @input="update" /></label><button type="button" @click="draft.thickness_prices.splice(i,1);update()">移除档位</button></div>
      <button type="button" @click="draft.thickness_prices.push({thickness_mm:0,unit_price:0});update()">添加厚度档位</button>
      <h3>工艺收费 <small>每项独立确认，不读取备注关键词</small></h3>
      <div v-for="(p,i) in draft.processes" :key="i" class="fields process">
        <label>工艺<select v-model="p.code" @change="update"><option v-for="(name,code) in processNames" :key="code" :value="code">{{name}}</option></select></label>
        <label>计费单位<select v-model="p.unit" @change="update"><option value="sku">每SKU</option><option value="piece">每片</option><option value="hole">每孔</option><option value="metre">每米</option><option value="area">每㎡</option></select></label>
        <label>工艺单价<input v-model.number="p.unit_price" type="number" min="0" step="any" @input="update" /></label>
        <label>工艺系数<input v-model.number="p.multiplier" type="number" min="0.0001" step="any" @input="update" /></label>
        <button type="button" @click="draft.processes.splice(i,1);update()">移除此工艺</button>
      </div>
      <button type="button" @click="draft.processes.push({code:'slot',unit:'sku',unit_price:0,multiplier:1});update()">添加工艺</button>
    </template>
  </section>
</template>
<script setup lang="ts">
import {ref,watch} from 'vue'
import {emptyCostModel,processNames,type CostModel} from '@/domain/cost-model'
const props=defineProps<{modelValue:string}>();const emit=defineEmits<{'update:modelValue':[string]}>()
const draft=ref<CostModel>(emptyCostModel())
watch(()=>props.modelValue,v=>{try {draft.value={...emptyCostModel(),...JSON.parse(v)}}catch{draft.value=emptyCostModel()}},{immediate:true})
function update(){emit('update:modelValue',JSON.stringify(draft.value))}
</script>
<style scoped>
.model-editor{min-width:0}.fields{display:grid;grid-template-columns:1fr 1fr;gap:10px}.row{display:grid;grid-template-columns:1fr 1fr auto;gap:8px;margin:8px 0}.model-editor label{display:grid;gap:4px;font-size:12px;color:rgb(var(--yb-text-secondary))}.model-editor input,.model-editor select{box-sizing:border-box;width:100%;min-width:0;padding:8px;border:1px solid rgb(var(--yb-border));border-radius:7px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.model-editor h3{font-size:13px;margin:16px 0 8px}.model-editor small{font-size:11px;font-weight:400;color:rgb(var(--yb-text-muted))}.model-editor button{padding:7px;border:1px solid rgb(var(--yb-border));border-radius:7px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));cursor:pointer}.formula{padding:12px;background:rgb(var(--yb-brand-soft));border-radius:8px;font-size:13px}.process{border-bottom:1px solid rgb(var(--yb-border));padding:10px 0;margin-bottom:8px}
</style>
