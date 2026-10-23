import http from "@/services/http";
import type { CostInput } from "@/domain/cost-model";

export interface MaintenanceRow {
  id: number;
  task_id: number;
  sku_code: string;
  product_name: string;
  style_code: string;
  width_cm: number | null;
  height_cm: number | null;
  area_m2: number | null;
  size_text: string;
  cost_input: CostInput | null;
  cost_price: number | null;
  erp_cost: number | null;
  revision: number;
  status: string;
  token: string;
  updated_at: string;
}
export const maintenanceReasons: Record<string, string> = {
  area_incorrect: "面积不对",
  style_binding_incorrect: "款式编码绑定不对",
  cost_incorrect: "成本价有误",
  other: "其他",
};
export const maintenanceStatuses: Record<string, string> = {
  baseline: "待核对",
  observe: "正在核对",
  pending: "系统已生效 · 待同步",
  retry: "同步待重试",
  synced: "ERP 已回读一致",
  conflict: "冲突待确认",
  no_price: "未报价",
};
export interface MaintenanceRequest {
  action: "confirm" | "modify";
  token: string;
  revision: number;
  erp_cost: number | null;
  reason_code?: string;
  reason_detail?: string;
  product_name?: string;
  style_code?: string;
  width_cm?: number;
  height_cm?: number;
  area_m2?: number;
  cost_price?: number;
  cost_input?: CostInput;
}
export interface MaintenanceLog {
  id: string;
  operator_id: number;
  operator_name: string;
  created_at: string;
  payload: {
    sku_code: string;
    action: string;
    reason_code: string;
    reason_detail: string;
    before: MaintenanceRow;
    after: MaintenanceRow;
  };
}
export const costMaintenanceApi = {
  async list(
    params: {
      keyword?: string;
      status?: string;
      before_id?: number;
      page_size?: number;
    },
    signal?: AbortSignal,
  ) {
    const { data } = await http.get<{
      data: MaintenanceRow[];
      next_before_id: number;
      refreshed_at: string;
    }>("/v1/cost-management/sku-costs", { params, signal });
    return data;
  },
  async maintain(id: number, payload: MaintenanceRequest) {
    const { data } = await http.post<{ data: MaintenanceRow }>(
      `/v1/cost-management/sku-costs/${id}/maintain`,
      payload,
    );
    return data.data;
  },
  async logs(params: {
    keyword?: string;
    reason_code?: string;
    page: number;
    page_size: number;
  }) {
    const { data } = await http.get<{
      data: MaintenanceLog[];
      pagination: { total: number };
    }>("/v1/cost-management/maintenance-logs", { params });
    return data;
  },
};
