export interface CostInput {
  area_mode: 'flat' | 'faces' | 'layout' | 'total'
  width_m: number; height_m: number; area_m2: number; thickness_mm: number
  pieces: number; hole_count: number; process_length_m: number
  faces: Array<{ width_m: number; height_m: number; count: number }>
  processes: Record<string, boolean>
}
export interface CostModel {
  version: number; material: string; basis: 'area' | 'piece' | 'set' | 'manual'
  unit_price: number; multiplier: number; minimum: number
  small_area_threshold: number; small_area_surcharge: number
  thickness_prices: Array<{ thickness_mm: number; unit_price: number }>
  processes: Array<{code: string; unit: string; unit_price: number; multiplier: number}>
}
export const processNames: Record<string,string> = {slot:'开槽',punch:'打孔',laminate:'覆膜',double_sided:'双面'}
export function emptyCostInput(): CostInput {
  return {area_mode:'total',width_m:0,height_m:0,area_m2:0,thickness_mm:0,pieces:1,hole_count:0,process_length_m:0,faces:[],processes:{}}
}
export function emptyCostModel(): CostModel {
  return {version:1,material:'',basis:'area',unit_price:0,multiplier:1,minimum:0,small_area_threshold:0,small_area_surcharge:0,thickness_prices:[],processes:[]}
}
