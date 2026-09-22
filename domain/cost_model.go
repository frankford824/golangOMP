package domain

// CostModel is a bounded configuration, never executable expression text.
// Unit costs always describe ONE SKU sales unit (one piece or one complete set).
type CostModel struct {
	Version            int                  `json:"version"`
	Material           string               `json:"material"`
	Basis              string               `json:"basis"` // area, piece, set, manual
	UnitPrice          float64              `json:"unit_price"`
	Multiplier         float64              `json:"multiplier"`
	Minimum            float64              `json:"minimum"`
	SmallAreaThreshold float64              `json:"small_area_threshold"`
	SmallAreaSurcharge float64              `json:"small_area_surcharge"`
	ThicknessPrices    []CostThicknessPrice `json:"thickness_prices,omitempty"`
	Processes          []CostProcessPrice   `json:"processes,omitempty"`
}
type CostThicknessPrice struct {
	ThicknessMM float64 `json:"thickness_mm"`
	UnitPrice   float64 `json:"unit_price"`
}
type CostProcessPrice struct {
	Code       string  `json:"code"`
	Unit       string  `json:"unit"` // sku, piece, hole, metre, area
	UnitPrice  float64 `json:"unit_price"`
	Multiplier float64 `json:"multiplier"`
}
type CostFace struct {
	WidthM  float64 `json:"width_m"`
	HeightM float64 `json:"height_m"`
	Count   int     `json:"count"`
}
type CostInput struct {
	AreaMode       string          `json:"area_mode"` // flat, faces, layout, total
	WidthM         float64         `json:"width_m"`
	HeightM        float64         `json:"height_m"`
	DepthM         float64         `json:"depth_m,omitempty"` // reference dimension; never guessed as layout
	AreaM2         float64         `json:"area_m2"`
	ThicknessMM    float64         `json:"thickness_mm"`
	Pieces         int             `json:"pieces"` // pieces within ONE SKU, not order quantity
	Faces          []CostFace      `json:"faces,omitempty"`
	Processes      map[string]bool `json:"processes,omitempty"`
	HoleCount      int             `json:"hole_count"`
	ProcessLengthM float64         `json:"process_length_m"`
}
type CostLine struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	Unit       string  `json:"unit"`
	UnitPrice  float64 `json:"unit_price"`
	Multiplier float64 `json:"multiplier"`
	Amount     float64 `json:"amount"`
}
type CostCalculation struct {
	Status           string     `json:"status"` // calculated, missing_input, manual_quote, rule_conflict
	Input            CostInput  `json:"input"`
	AreaM2           float64    `json:"area_m2"`
	BillableQuantity float64    `json:"billable_quantity"`
	Lines            []CostLine `json:"lines"`
	Missing          []string   `json:"missing"`
}
