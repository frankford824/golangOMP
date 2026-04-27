package domain

import "encoding/json"

type ERPProductSnapshot struct {
	Code        string          `json:"code"`
	ProductName string          `json:"product_name"`
	Snapshot    json.RawMessage `json:"snapshot"`
}
