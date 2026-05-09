import type { OutsourceOrder } from '../types/outsource'
import { getOutsourceOrderStatusLabel } from '../enums/outsource'

export interface OutsourceOrderViewModel extends OutsourceOrder {
  statusLabel: string
}

export function outsourceOrderToViewModel(dto: OutsourceOrder): OutsourceOrderViewModel {
  return {
    ...dto,
    statusLabel: getOutsourceOrderStatusLabel(dto.status),
  }
}
