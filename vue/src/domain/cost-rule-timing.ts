// Business schedules are entered in China time, not the operator's OS timezone.
export function chinaDateInput(value?:string|null):string {
 if(!value)return ''
 const d=new Date(value);if(!Number.isFinite(d.getTime()))return ''
 return new Intl.DateTimeFormat('sv-SE',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false}).format(d).replace(' ','T')
}
export function pricingWindow(start:string,end:string){
 const from=start?`${start}:00+08:00`:null,to=end?`${end}:00+08:00`:null
 if((from&&!Number.isFinite(Date.parse(from)))||(to&&!Number.isFinite(Date.parse(to))))throw new Error('请填写完整的生效日期和时间。')
 if(from&&to&&Date.parse(to)<=Date.parse(from))throw new Error('结束时间必须晚于开始时间。')
 return {effective_from:from,effective_to:to}
}
export function pricingWindowLabel(rule:{effective_from?:string|null;effective_to?:string|null}){
 return `${rule.effective_from?chinaDateInput(rule.effective_from).replace('T',' ')+' 起':'立即生效'} · ${rule.effective_to?chinaDateInput(rule.effective_to).replace('T',' ')+' 止':'长期有效'}（北京时间）`
}
