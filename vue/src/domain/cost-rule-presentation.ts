export interface ReadableRule {rule_type:string;formula_expression?:string;base_price?:number|null;tax_multiplier?:number|null;min_area?:number|null;area_threshold?:number|null;surcharge_amount?:number|null;special_process_keyword?:string;special_process_price?:number|null}
export function printPrices(expression=''){const match=/^print_side:single=([\d.]+),double=([\d.]+)$/.exec(expression);return match?{single:Number(match[1]),double:Number(match[2])}:null}
export function keywordPrice(expression=''){const match=/^keyword_area_unit_price:(.+)=([\d.]+)$/.exec(expression);return match?{keyword:match[1],price:Number(match[2])}:null}
export function ruleDescription(r:ReadableRule):string {
 const price=(n?:number|null)=>n==null?'价格待填写':`${n} 元`
 switch(r.rule_type){
 case 'fixed_unit_price':return `每平方米 ${price(r.base_price)}${r.tax_multiplier&&r.tax_multiplier!==1?`，再乘 ${r.tax_multiplier}`:''}`
 case 'minimum_billable_area':return `不足 ${r.min_area??'待填写'} 平方米，按 ${r.min_area??'待填写'} 平方米收费`
 case 'area_threshold_surcharge':return `面积小于 ${r.area_threshold??'待填写'} 平方米时，每平方米加 ${price(r.surcharge_amount)}`
 case 'special_process_surcharge':return `${r.special_process_keyword||'指定工艺'}：另加 ${price(r.special_process_price)}`
 case 'manual_quote':return '每个产品由工作人员确认成本，不自动报价'
 case 'size_based_formula':{const p=printPrices(r.formula_expression);if(p)return `单面 ${price(p.single)}/张，双面 ${price(p.double)}/张`;const k=keywordPrice(r.formula_expression);if(k)return `“${k.keyword}”产品每平方米 ${price(k.price)}`;return '按产品规格确认报价，请先核对适用尺寸和单价'}
 case 'cost_model':try{const m=JSON.parse(r.formula_expression||'{}');return m.basis==='manual'?'人工确认报价':`${m.material}，每${({area:'平方米',piece:'件',set:'套'} as Record<string,string>)[m.basis]||'单位'} ${price(m.unit_price)}${m.multiplier!==1?`，再乘 ${m.multiplier}`:''}`}catch{return '价格信息需要核对'}
 default:return '价格信息需要核对'
 }
}
