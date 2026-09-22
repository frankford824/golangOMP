package domain

// CostBoundSKU is a current binding read model, not a historical cost match.
type CostBoundSKU struct {
	ID          int64    `json:"id"`
	SKUCode     string   `json:"sku_code"`
	ProductName string   `json:"product_name"`
	StyleCode   string   `json:"style_code"`
	CostPrice   *float64 `json:"cost_price"`
	Status      string   `json:"status"`
}
