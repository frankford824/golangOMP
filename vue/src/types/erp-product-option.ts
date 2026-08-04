// Lightweight ERP product option used only by task creation and binding flows.
// It is not the retired standalone product-center read model.
export interface ERPProductOption {
  id: string
  sku: string
  name: string
  category: string
  spec: string
  designHistorySummary?: string
  imageUrl?: string
  categoryCode?: string
  shortName?: string
}
