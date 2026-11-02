<template>
  <section class="model-editor" aria-label="统一计价方案">
    <div class="fields">
      <label v-if="!hideMaterial">材质<input v-model.trim="draft.material" @input="update" placeholder="例如：覆膜KT板" /></label>
      <label>计费方式<select v-model="draft.basis" @change="update"><option value="area">按面积</option><option value="piece">按件</option><option value="set">按套</option><option value="manual">人工报价</option></select></label>
      <template v-if="draft.basis !== 'manual'">
        <label v-if="!draft.variant_prices?.length">每{{draft.basis==='area'?'平方米':draft.basis==='piece'?'件':'套'}}价格（元）<input v-model.number="draft.unit_price" required type="number" min="0" step="any" @input="update" /></label>
        <details class="extras" :open="draft.multiplier!==1 || draft.minimum>0 || draft.small_area_threshold>0"><summary><ChevronDown :size="16" :stroke-width="1.75" aria-hidden="true" />起步价与价格调整（选填）</summary><div class="fields">
        <label>价格调整倍数<input v-model.number="draft.multiplier" type="number" min="0.0001" step="any" @input="update" /></label>
        <label>最低计费量<input v-model.number="draft.minimum" type="number" min="0" step="any" @input="update" /></label>
        <template v-if="draft.basis === 'area'"><label>小面积阈值（㎡，0表示无）<input v-model.number="draft.small_area_threshold" type="number" min="0" step="any" @input="update" /></label><label>低于阈值每㎡加价<input v-model.number="draft.small_area_surcharge" type="number" min="0" step="any" @input="update" /></label></template>
        </div></details>
      </template>
    </div>
    <p class="formula">{{draft.basis==='manual'?'每个产品由工作人员确认价格。':draft.variant_prices?.length?'按厚度和形状取单价，再乘计费量与倍率。留空表示未核价，0 表示免费。':'按实际用料收费，选中的加工项目另计费用。'}}</p>
    <template v-if="draft.basis !== 'manual'">
      <section v-if="draft.variant_prices?.length" class="variant-prices" aria-label="厚度与形状价格表">
        <div class="price-table-scroll"><table><thead><tr><th>厚度（mm）</th><th>普通（元/{{unitLabel}}）</th><th>异形（元/{{unitLabel}}）</th><th><span class="sr-only">操作</span></th></tr></thead><tbody>
          <tr v-for="(row,i) in draft.variant_prices" :key="i">
            <td><input v-model.number="row.thickness_mm" :aria-label="`第${i+1}行厚度`" required type="number" min="0" step="any" @input="update" /></td>
            <td><input :value="row.normal_price ?? ''" :aria-label="`第${i+1}行普通单价`" type="number" min="0" step="any" placeholder="待核价" @input="setPrice(i,'normal_price',$event)" /></td>
            <td><input :value="row.irregular_price ?? ''" :aria-label="`第${i+1}行异形单价`" type="number" min="0" step="any" placeholder="待核价" @input="setPrice(i,'irregular_price',$event)" /></td>
            <td><button type="button" :aria-label="`移除第${i+1}行价格`" :disabled="draft.variant_prices.length===1" @click="draft.variant_prices.splice(i,1);update()"><X :size="16" :stroke-width="1.75" aria-hidden="true" /></button></td>
          </tr>
        </tbody></table></div>
        <div class="price-table-footer"><small>厚度 0 表示不分厚度，只能单独一行。异形面积请在 SKU 中明确填写。</small><button type="button" :disabled="draft.variant_prices.length>=32" @click="draft.variant_prices.push({thickness_mm:0,normal_price:null,irregular_price:null});update()"><Plus :size="16" :stroke-width="1.75" aria-hidden="true" />添加厚度</button></div>
      </section>
      <template v-else>
      <button type="button" class="enable-variants" @click="enableVariants"><Plus :size="16" :stroke-width="1.75" aria-hidden="true" />启用普通 / 异形价格表</button>
      <details :open="draft.thickness_prices.length>0"><summary><ChevronDown :size="16" :stroke-width="1.75" aria-hidden="true" />不同厚度，不同价格（选填）</summary>
      <div v-for="(p,i) in draft.thickness_prices" :key="i" class="row"><label>厚度（mm）<input v-model.number="p.thickness_mm" type="number" min="0.001" step="any" @input="update" /></label><label>对应单价<input v-model.number="p.unit_price" type="number" min="0" step="any" @input="update" /></label><button type="button" @click="draft.thickness_prices.splice(i,1);update()">移除档位</button></div>
      <button type="button" @click="draft.thickness_prices.push({thickness_mm:0,unit_price:0});update()">添加厚度档位</button>
      </details></template><details :open="draft.processes.length>0"><summary><ChevronDown :size="16" :stroke-width="1.75" aria-hidden="true" />开槽、打孔等另收费（选填）</summary>
      <div v-for="(p,i) in draft.processes" :key="i" class="fields process">
        <label>工艺<select v-model="p.code" @change="update"><option v-for="(name,code) in processNames" :key="code" :value="code" :disabled="draft.processes.some((other,index)=>index!==i&&other.code===code)">{{name}}</option></select></label>
        <label>计费单位<select v-model="p.unit" @change="update"><option value="sku">每SKU</option><option value="piece">每片</option><option value="hole">每孔</option><option value="metre">每米</option><option value="area">每㎡</option></select></label>
        <label>工艺单价<input v-model.number="p.unit_price" type="number" min="0" step="any" @input="update" /></label>
        <label>工艺系数<input v-model.number="p.multiplier" type="number" min="0.0001" step="any" @input="update" /></label>
        <button type="button" @click="draft.processes.splice(i,1);update()">移除此工艺</button>
      </div>
      <button type="button" :disabled="draft.processes.length>=Object.keys(processNames).length" @click="addProcess">添加工艺</button>
      </details>
    </template>
  </section>
</template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {Plus,X,ChevronDown} from 'lucide-vue-next'
import {emptyCostModel,processNames,type CostModel} from '@/domain/cost-model'
const props=defineProps<{modelValue:string;hideMaterial?:boolean}>();const emit=defineEmits<{'update:modelValue':[string]}>()
const draft=ref<CostModel>(emptyCostModel())
const unitLabel=computed(()=>({area:'㎡',piece:'件',set:'套',manual:'次'})[draft.value.basis])
watch(()=>props.modelValue,v=>{try {draft.value={...emptyCostModel(),...JSON.parse(v)}}catch{draft.value=emptyCostModel()}},{immediate:true})
function update(){emit('update:modelValue',JSON.stringify(draft.value))}
function enableVariants(){draft.value.variant_prices=draft.value.thickness_prices.length?draft.value.thickness_prices.map(p=>({thickness_mm:p.thickness_mm,normal_price:p.unit_price,irregular_price:null})):[{thickness_mm:0,normal_price:draft.value.unit_price,irregular_price:null}];draft.value.thickness_prices=[];update()}
function setPrice(index:number,key:'normal_price'|'irregular_price',event:Event){const input=event.target as HTMLInputElement;draft.value.variant_prices![index]![key]=input.value===''?null:input.valueAsNumber;update()}
function addProcess(){const code=Object.keys(processNames).find(code=>!draft.value.processes.some(p=>p.code===code));if(!code)return;draft.value.processes.push({code,unit:'sku',unit_price:0,multiplier:1});update()}
</script>
<style scoped>
.extras{grid-column:1/-1}.model-editor details{padding:10px 0;border-top:1px solid rgb(var(--yb-border))}.model-editor summary{font-size:13px;cursor:pointer;padding:4px 0 8px}
.model-editor{min-width:0}.fields{display:grid;grid-template-columns:1fr 1fr;gap:10px}.row{display:grid;grid-template-columns:1fr 1fr auto;gap:8px;margin:8px 0}.model-editor label{display:grid;gap:4px;font-size:12px;color:rgb(var(--yb-text-secondary))}.model-editor input,.model-editor select{box-sizing:border-box;width:100%;min-width:0;padding:8px;border:1px solid rgb(var(--yb-border));border-radius:7px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.model-editor h3{font-size:13px;margin:16px 0 8px}.model-editor small{font-size:11px;font-weight:400;color:rgb(var(--yb-text-muted))}.model-editor button{padding:7px;border:1px solid rgb(var(--yb-border));border-radius:7px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));cursor:pointer}.formula{padding:12px;background:rgb(var(--yb-brand-soft));border-radius:8px;font-size:13px}.process{border-bottom:1px solid rgb(var(--yb-border));padding:10px 0;margin-bottom:8px}
</style>
<style scoped>
.variant-prices{border-top:1px solid rgb(var(--yb-border));padding:12px 0}.price-table-scroll{overflow-x:auto}.variant-prices table{width:100%;border-collapse:collapse;table-layout:fixed}.variant-prices th{font-size:12px;font-weight:500;text-align:left;padding:0 6px 8px;color:rgb(var(--yb-text-secondary))}.variant-prices td{padding:4px}.variant-prices th:last-child{width:36px}.variant-prices td button{width:32px;height:36px;display:inline-flex;align-items:center;justify-content:center;padding:0}.price-table-footer{display:flex;align-items:center;gap:12px;justify-content:space-between;margin-top:8px}.price-table-footer small{line-height:18px}.price-table-footer button,.enable-variants{display:inline-flex;align-items:center;gap:6px;flex-shrink:0}.model-editor button:disabled{opacity:.45;cursor:default}.model-editor label{font-size:13px;font-weight:500}.model-editor input,.model-editor select{font-size:14px;line-height:20px;min-height:38px}.model-editor button{font-size:13px;font-weight:500;line-height:20px}.model-editor small{font-size:12px}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}@media(max-width:600px){.price-table-footer{align-items:start;flex-direction:column}.fields{grid-template-columns:1fr}}
.model-editor summary{display:flex;align-items:center;gap:6px;list-style:none}.model-editor summary::-webkit-details-marker{display:none}.model-editor summary svg{display:block;flex-shrink:0;transform:rotate(-90deg)}.model-editor details[open]>summary svg{transform:rotate(0)}
</style>
