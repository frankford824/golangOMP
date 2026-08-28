package domain

import "time"

// ERPCostSKU is the read-only cost projection exposed by the 8081 Bridge.
// CostPrice is a decimal string so external consumers never lose four-decimal
// precision through JSON floating-point conversion.
type ERPCostSKU struct {
	SKUID      string    `json:"sku_id"`
	SKUType    *string   `json:"sku_type"`
	CostPrice  *string   `json:"cost_price"`
	SalePrice  *string   `json:"sale_price"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ERPCostFeedResult struct {
	Data            []ERPCostSKU `json:"data"`
	NextCursor      string       `json:"next_cursor,omitempty"`
	Watermark       time.Time    `json:"watermark"`
	SnapshotVersion string       `json:"snapshot_version"`
}

type ERPBatchCostResult struct {
	Data            []ERPCostSKU `json:"data"`
	MissingSKUIDs   []string     `json:"missing_sku_ids"`
	Watermark       time.Time    `json:"watermark"`
	SnapshotVersion string       `json:"snapshot_version"`
}

type JSTHistoryCostQuery struct {
	SKUIDs                []string `json:"sku_ids"`
	WMSCoIDs              []int64  `json:"wms_co_ids,omitempty"`
	GetWay                string   `json:"get_way,omitempty"`
	IsUseItemSKUCostPrice bool     `json:"is_use_item_sku_cost_price"`
}

type JSTHistoryCostPeriod struct {
	WMSCoID   string  `json:"wms_co_id"`
	SKUID     string  `json:"sku_id"`
	CostPrice *string `json:"cost_price"`
	BeginDate string  `json:"begin_date,omitempty"`
	EndDate   string  `json:"end_date,omitempty"`
	Remark    string  `json:"remark,omitempty"`
}

type JSTHistoryCostResponse struct {
	Periods []JSTHistoryCostPeriod `json:"periods"`
}

type ERPHistoryCostItem struct {
	SKUID     string  `json:"sku_id"`
	WMSCoID   string  `json:"wms_co_id"`
	CostPrice *string `json:"cost_price"`
	AsOf      string  `json:"as_of"`
	BeginDate string  `json:"begin_date,omitempty"`
	EndDate   string  `json:"end_date,omitempty"`
	Remark    string  `json:"remark,omitempty"`
}

type ERPHistoryCostResult struct {
	Data            []ERPHistoryCostItem `json:"data"`
	MissingSKUIDs   []string             `json:"missing_sku_ids"`
	SnapshotVersion string               `json:"snapshot_version"`
}

type ERPCostChange struct {
	ID               int64      `json:"id"`
	SKUID            string     `json:"sku_id"`
	SKUType          *string    `json:"sku_type"`
	OldCostPrice     *string    `json:"old_cost_price"`
	NewCostPrice     *string    `json:"new_cost_price"`
	SourceModifiedAt *time.Time `json:"modified_at"`
	ChangedAt        time.Time  `json:"changed_at"`
}

type ERPCostChangesResult struct {
	Data            []ERPCostChange `json:"data"`
	NextCursor      string          `json:"next_cursor,omitempty"`
	Watermark       int64           `json:"watermark"`
	SnapshotVersion string          `json:"snapshot_version"`
}
