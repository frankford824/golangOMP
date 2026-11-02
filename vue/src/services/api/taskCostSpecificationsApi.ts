import http from '@/services/http'
import type {CostInput} from '@/domain/cost-model'

export interface TaskCostSpecification {
  task_id:number;task_no:string;task_status:string;sku_item_id:number;sku_code:string;product_name:string
  requires_area:boolean;missing_area:boolean;cost_input:CostInput;cost_price:number|null
  cost_review_required:boolean;can_edit:boolean;token:string
}
export interface TaskCostSpecificationList {data:TaskCostSpecification[];total:number;page:number;page_size:number}
export const taskCostSpecificationsApi={
  async list(taskID?:number,page=1){const {data}=await http.get<TaskCostSpecificationList>(taskID?`/v1/tasks/${taskID}/cost-specifications`:'/v1/tasks/cost-specification-todos',{params:{page,page_size:20}});return data},
  async save(row:TaskCostSpecification,input:CostInput,reason:string){const {data}=await http.patch<TaskCostSpecificationList>(`/v1/tasks/${row.task_id}/cost-specifications`,{sku_item_id:row.sku_item_id,token:row.token,reason,cost_input:input});return data},
}
