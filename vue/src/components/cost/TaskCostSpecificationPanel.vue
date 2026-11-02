<template>
  <section class="task-cost-specification" aria-label="计价资料待办">
    <button type="button" class="spec-heading" :aria-expanded="expanded" @click="expanded=!expanded">
      <span><Ruler :size="16" :stroke-width="1.75" aria-hidden="true" /><strong>{{taskId?'计价资料':'我的待补计价资料'}}</strong><b v-if="taskId?missingCount>0:total>0">{{taskId?`本页 ${missingCount}`:total}}</b></span>
      <span>{{loading?'读取中…':taskId?missingCount?'缺面积 · 结单后仍可补录':'查看 / 补录':total?'图已完成，仍有资料待补':'暂无待补面积'}}<ChevronDown :size="16" :stroke-width="1.75" aria-hidden="true" /></span>
    </button>
    <p v-if="error" class="spec-error" role="alert">{{error}}<button type="button" @click="load">重试读取</button></p>
    <p v-if="notice" class="spec-notice" role="status">{{notice}}</p>
    <div v-if="expanded" class="spec-content">
      <p>按面积计价的编码缺面积时，会持续留在待办；通知已读或任务结单都不会清除。</p>
      <div v-if="!loading&&!rows.length" class="spec-empty">{{taskId?'暂无计价资料。':'当前没有需要补录面积的编码。'}}</div>
      <article v-for="row in rows" :key="`${row.task_id}:${row.sku_item_id}`" class="spec-row">
        <div><strong>{{row.sku_code||row.task_no}}</strong><small>{{row.product_name}}</small><small>{{row.task_no}} · {{row.task_status==='Completed'?'已结单':row.task_status==='PendingAudit'?'图已完成 · 待审核':'处理中'}} · {{row.missing_area?'待补面积':row.requires_area?'面积已填写':'当前计价无需面积'}}<template v-if="row.cost_review_required"> · 成本待核对</template></small></div>
        <button v-if="row.can_edit" type="button" @click="edit(row)">{{row.missing_area?'补录面积':'调整计价资料'}}</button>
        <span v-else class="spec-readonly">只读</span>
      </article>
      <footer v-if="total>20"><button type="button" :disabled="loading||page<=1" @click="page--;load()">上一页</button><span>{{page}} / {{Math.ceil(total/20)}}</span><button type="button" :disabled="loading||page*20>=total" @click="page++;load()">下一页</button></footer>
    </div>
    <Teleport to="body">
      <div v-if="selected" class="spec-modal" @keydown.esc="close">
        <button class="spec-backdrop" type="button" aria-label="关闭计价资料补录" :disabled="saving" @click="close" />
        <form ref="form" class="spec-dialog" role="dialog" aria-modal="true" aria-labelledby="cost-spec-title" tabindex="-1" @keydown.tab="trapFocus" @submit.prevent="save">
          <header><div><h2 id="cost-spec-title">补录计价资料</h2><p>{{selected.sku_code||selected.task_no}} · {{selected.product_name}}</p></div><button type="button" aria-label="关闭补录窗口" :disabled="saving" @click="close"><X :size="18" :stroke-width="1.75" aria-hidden="true" /></button></header>
          <div class="spec-editor">
            <p>保存尺寸与面积会保留当前价格，并提示核对成本。任务保持原状态。</p>
            <p v-if="editError" class="spec-error" role="alert">{{editError}}</p>
            <CostInputFields v-if="selected.sku_item_id>0" v-model="input" dimensions-only :disabled="saving" />
            <label v-else>整套计费总面积（㎡）<input v-model.number="input.area_m2" type="number" min="0.0001" max="999999.9999" step="any" required :disabled="saving" /><small>填写整套用料面积；编码生成后可继续补充厚度和形状。</small></label>
            <label>补录 / 调整原因<input v-model.trim="reason" required maxlength="200" :disabled="saving" /></label>
          </div>
          <footer><button type="button" :disabled="saving" @click="close">取消</button><button type="submit" class="spec-primary" :disabled="saving">{{saving?'保存中…':'保存资料，成本待核对'}}</button></footer>
        </form>
      </div>
    </Teleport>
  </section>
</template>
<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {Ruler,ChevronDown,X} from 'lucide-vue-next'
import CostInputFields from './CostInputFields.vue'
import {emptyCostInput} from '@/domain/cost-model'
import {taskCostSpecificationsApi,type TaskCostSpecification} from '@/services/api/taskCostSpecificationsApi'
import {retryCostRead} from '@/utils/cost-read-retry'
const props=defineProps<{taskId?:number}>()
const emit=defineEmits<{'saved':[]}>()
const expanded=ref(false),loading=ref(false),saving=ref(false),error=ref(''),notice=ref(''),editError=ref(''),rows=ref<TaskCostSpecification[]>([]),total=ref(0),page=ref(1)
const selected=ref<TaskCostSpecification|null>(null),input=ref(emptyCostInput()),reason=ref(''),form=ref<HTMLFormElement>()
const missingCount=computed(()=>rows.value.filter(r=>r.missing_area).length)
let request=0,timer:ReturnType<typeof setInterval>|undefined,focusBefore:HTMLElement|null=null,bodyOverflow=''
async function load(){const ticket=++request;loading.value=true;error.value='';try{const result=await retryCostRead(()=>taskCostSpecificationsApi.list(props.taskId,page.value));if(ticket!==request)return;const lastPage=Math.max(1,Math.ceil((result.total||0)/20));if(page.value>lastPage){page.value=lastPage;await load();return}rows.value=result.data||[];total.value=result.total||0}catch(e){if(ticket===request)error.value=e instanceof Error?e.message:'读取失败'}finally{if(ticket===request)loading.value=false}}
function edit(row:TaskCostSpecification){focusBefore=document.activeElement as HTMLElement;selected.value=row;input.value=row.sku_item_id===0?{...emptyCostInput(),area_m2:row.cost_input.area_m2}:JSON.parse(JSON.stringify(row.cost_input));reason.value=row.missing_area?'补录缺失计价资料':'调整计价尺寸与面积';editError.value='';bodyOverflow=document.body.style.overflow;document.body.style.overflow='hidden';nextTick(()=>form.value?.focus())}
function close(){if(saving.value)return;selected.value=null;document.body.style.overflow=bodyOverflow;nextTick(()=>focusBefore?.focus())}
function trapFocus(event:KeyboardEvent){const elements=[...form.value?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')||[]];const first=elements[0],last=elements.at(-1);if(!first||!last)return;if(event.shiftKey&&(document.activeElement===first||document.activeElement===form.value)){event.preventDefault();last.focus()}else if(!event.shiftKey&&(document.activeElement===last||document.activeElement===form.value)){event.preventDefault();first.focus()}}
async function save(){if(!selected.value)return;saving.value=true;editError.value='';try{await taskCostSpecificationsApi.save(selected.value,input.value,reason.value);saving.value=false;close();notice.value='计价资料已保存，原价格保留；成本待核对。';emit('saved');await load()}catch(e){editError.value=e instanceof Error?e.message:'保存失败；请稍后重试'}finally{saving.value=false}}
watch(()=>props.taskId,()=>{page.value=1;notice.value='';void load()})
onMounted(()=>{void load();timer=setInterval(()=>{if(document.visibilityState==='visible'&&!selected.value&&!loading.value)void load()},30000)})
onBeforeUnmount(()=>{request++;if(timer)clearInterval(timer);if(selected.value)document.body.style.overflow=bodyOverflow})
</script>
<style scoped>
.task-cost-specification{border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface));overflow:hidden;min-width:0}.spec-heading{display:flex;width:100%;align-items:center;justify-content:space-between;gap:12px;padding:12px 16px;text-align:left;background:transparent;border:0;color:rgb(var(--yb-text));cursor:pointer}.spec-heading>span{display:flex;align-items:center;gap:8px;min-width:0;font-size:13px;line-height:20px}.spec-heading strong{font-size:14px;font-weight:600}.spec-heading b{font-size:12px;color:rgb(var(--yb-warning-text));background:rgb(var(--yb-surface-soft));border-radius:6px;padding:1px 6px}.spec-content{padding:0 16px 12px}.spec-content>p,.spec-empty{font-size:12px;line-height:20px;color:rgb(var(--yb-text-secondary))}.spec-row{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 0;border-top:1px solid rgb(var(--yb-border))}.spec-row>div{min-width:0;display:grid;gap:4px}.spec-row strong{font-size:14px;font-weight:600}.spec-row small,.spec-readonly{font-size:12px;line-height:18px;color:rgb(var(--yb-text-secondary))}.spec-error,.spec-notice{padding:10px 16px;margin:0;font-size:13px;line-height:20px}.spec-error{color:rgb(var(--yb-danger))}.spec-notice{color:rgb(var(--yb-success))}.spec-row button,.spec-error button,.spec-content footer button,.spec-dialog button{display:inline-flex;align-items:center;justify-content:center;min-height:36px;gap:6px;border:1px solid rgb(var(--yb-border));border-radius:8px;padding:7px 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand));font:500 13px/20px var(--yb-font-text);cursor:pointer;flex-shrink:0}.spec-content footer,.spec-dialog footer{display:flex;align-items:center;justify-content:flex-end;gap:10px}.spec-modal{position:fixed;inset:0;z-index:2200;display:grid;place-items:center;padding:16px}.spec-backdrop{position:absolute;inset:0;border:0;background:rgb(var(--yb-shadow)/.45)}.spec-dialog{position:relative;width:min(660px,100%);max-height:calc(100dvh - 32px);display:flex;flex-direction:column;min-height:0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:14px;background:rgb(var(--yb-surface));box-shadow:0 18px 60px rgb(var(--yb-shadow)/.2);outline:none}.spec-dialog header{display:flex;align-items:start;justify-content:space-between;gap:12px;padding:16px 20px;border-bottom:1px solid rgb(var(--yb-border))}.spec-dialog h2{margin:0;font:600 18px/26px var(--yb-font-text)}.spec-dialog header p{margin:4px 0 0;font:400 13px/20px var(--yb-font-text);color:rgb(var(--yb-text-secondary))}.spec-editor{overflow:auto;padding:16px 20px;display:grid;gap:12px}.spec-editor>p{font:400 13px/20px var(--yb-font-text);margin:0;color:rgb(var(--yb-text-secondary))}.spec-editor label{display:grid;gap:6px;font:500 13px/20px var(--yb-font-text)}.spec-editor input{width:100%;min-height:38px;border:1px solid rgb(var(--yb-border));border-radius:8px;padding:8px 10px;font:400 14px/20px var(--yb-font-text);box-sizing:border-box;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.spec-dialog footer{padding:12px 20px;border-top:1px solid rgb(var(--yb-border))}.spec-dialog .spec-primary{background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));border-color:rgb(var(--yb-brand))}.spec-dialog button:disabled{opacity:.5;cursor:default}@media(max-width:600px){.spec-heading,.spec-row{align-items:start;flex-wrap:wrap}.spec-heading>span:last-child{font-size:12px}.spec-editor{padding:12px}.spec-dialog header,.spec-dialog footer{padding:12px}.spec-dialog footer{flex-wrap:wrap}}
</style>
<style scoped>
.spec-editor :deep(.cost-inputs label){font:500 13px/20px var(--yb-font-text)}.spec-editor :deep(.cost-inputs input),.spec-editor :deep(.cost-inputs select){font:400 14px/20px var(--yb-font-text);min-height:38px}
</style>
