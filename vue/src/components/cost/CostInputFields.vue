<template>
  <fieldset class="cost-inputs" :disabled="disabled">
    <legend>一个 SKU 的计价规格</legend>
    <label>面积方式<select v-model="draft.area_mode" @change="emitValue"><option value="total">已知整套总面积</option><option value="flat">平面 · 单片长宽</option><option value="faces">多面 / 多片汇总</option><option value="layout">展开用料总长宽</option></select></label>
    <label>每 SKU 片数<input v-model.number="draft.pieces" type="number" min="1" step="1" @input="emitValue" /></label>
    <template v-if="draft.area_mode === 'flat' || draft.area_mode === 'layout'">
      <label>{{ draft.area_mode === 'layout' ? '展开总长' : '单片长' }}（米）<input v-model.number="draft.width_m" type="number" min="0" step="any" @input="emitValue" /></label>
      <label>{{ draft.area_mode === 'layout' ? '展开总宽' : '单片宽' }}（米）<input v-model.number="draft.height_m" type="number" min="0" step="any" @input="emitValue" /></label>
    </template>
    <label v-if="draft.area_mode === 'total'">整套总面积（㎡）<input v-model.number="draft.area_m2" type="number" min="0" step="any" @input="emitValue" /></label>
    <label>厚度（mm，可选）<input v-model.number="draft.thickness_mm" type="number" min="0" step="any" @input="emitValue" /></label>
    <section v-if="draft.area_mode === 'faces'" class="full">
      <p>按实际用料填写每种面的尺寸。立体表面积不一定等于展开用料面积。</p>
      <div v-for="(face,i) in draft.faces" :key="i" class="face-row">
        <label>面{{ i+1 }}长（m）<input v-model.number="face.width_m" type="number" min="0" step="any" @input="emitValue" /></label>
        <label>宽（m）<input v-model.number="face.height_m" type="number" min="0" step="any" @input="emitValue" /></label>
        <label>片数<input v-model.number="face.count" type="number" min="1" @input="emitValue" /></label>
        <button type="button" :aria-label="`移除面${i+1}`" @click="draft.faces.splice(i,1);emitValue()">移除</button>
      </div>
      <button type="button" @click="draft.faces.push({width_m:0,height_m:0,count:1});emitValue()">添加一组面</button>
    </section>
    <label v-for="(name,code) in processNames" :key="code">{{name}}<select :value="draft.processes[code] == null ? '' : String(draft.processes[code])" @change="setProcess(code,$event)"><option value="">未确认</option><option value="false">否</option><option value="true">是</option></select></label>
    <label v-if="draft.processes.punch">整套孔数<input v-model.number="draft.hole_count" type="number" min="0" step="1" @input="emitValue" /></label>
    <label v-if="Object.values(draft.processes).some(Boolean)">整套加工长度（米，按米计费时填）<input v-model.number="draft.process_length_m" type="number" min="0" step="any" @input="emitValue" /></label>
    <small class="full">总面积、展开面积、分面面积已包含整套用料，不再乘片数。订单数量不参与单 SKU 成本。</small>
  </fieldset>
</template>
<script setup lang="ts">
import {ref,watch} from 'vue'
import {emptyCostInput,processNames,type CostInput} from '@/domain/cost-model'
const props=defineProps<{modelValue:CostInput;disabled?:boolean}>()
const emit=defineEmits<{ 'update:modelValue':[CostInput] }>()
const draft=ref<CostInput>(emptyCostInput())
watch(()=>props.modelValue,v=>{draft.value=JSON.parse(JSON.stringify({...emptyCostInput(),...v,faces:v?.faces||[],processes:v?.processes||{}}))},{immediate:true,deep:true})
function emitValue(){emit('update:modelValue',JSON.parse(JSON.stringify(draft.value)))}
function setProcess(code:string,e:Event){const value=(e.target as HTMLSelectElement).value;if(value==='')delete draft.value.processes[code];else draft.value.processes[code]=value==='true';emitValue()}
</script>
<style scoped>
.cost-inputs{margin:0;padding:12px;border:1px solid rgb(var(--yb-border));border-radius:10px;display:grid;grid-template-columns:1fr 1fr;gap:10px;min-width:0}.cost-inputs legend{font-size:13px;font-weight:700}.cost-inputs label{display:grid;gap:5px;font-size:12px;color:rgb(var(--yb-text-secondary))}.cost-inputs input,.cost-inputs select{box-sizing:border-box;width:100%;min-width:0;padding:8px;border:1px solid rgb(var(--yb-border));border-radius:7px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.full{grid-column:1/-1;font-size:12px;color:rgb(var(--yb-text-muted))}.face-row{display:grid;grid-template-columns:1fr 1fr 1fr auto;gap:6px;margin-bottom:8px}.cost-inputs button{border:1px solid rgb(var(--yb-border));border-radius:7px;padding:6px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));cursor:pointer}@media(max-width:600px){.face-row{grid-template-columns:1fr 1fr}.cost-inputs{grid-template-columns:1fr}.full{grid-column:auto}}
</style>
