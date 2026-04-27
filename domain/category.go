package domain

import "time"

type CategoryType string

const (
	CategoryTypeCodedStyle CategoryType = "coded_style"
	CategoryTypeBoard      CategoryType = "board"
	CategoryTypePaper      CategoryType = "paper"
	CategoryTypePrint      CategoryType = "print"
	CategoryTypeCloth      CategoryType = "cloth"
	CategoryTypeMaterial   CategoryType = "material"
	CategoryTypeCustom     CategoryType = "custom"
	CategoryTypeManual     CategoryType = "manual_quote"
	CategoryTypeOther      CategoryType = "other"
)

func (t CategoryType) Valid() bool {
	switch t {
	case CategoryTypeCodedStyle, CategoryTypeBoard, CategoryTypePaper, CategoryTypePrint,
		CategoryTypeCloth, CategoryTypeMaterial, CategoryTypeCustom, CategoryTypeManual, CategoryTypeOther:
		return true
	default:
		return false
	}
}

type Category struct {
	CategoryID      int64        `db:"id"                json:"category_id"`
	CategoryCode    string       `db:"category_code"     json:"category_code"`
	CategoryName    string       `db:"category_name"     json:"category_name"`
	DisplayName     string       `db:"display_name"      json:"display_name"`
	ParentID        *int64       `db:"parent_id"         json:"parent_id,omitempty"`
	Level           int          `db:"level_no"          json:"level"`
	SearchEntryCode string       `db:"search_entry_code" json:"search_entry_code"`
	IsSearchEntry   bool         `db:"is_search_entry"   json:"is_search_entry"`
	CategoryType    CategoryType `db:"category_type"     json:"category_type"`
	IsActive        bool         `db:"is_active"         json:"is_active"`
	SortOrder       int          `db:"sort_order"        json:"sort_order"`
	Source          string       `db:"source"            json:"source"`
	Remark          string       `db:"remark"            json:"remark"`
	CreatedAt       time.Time    `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time    `db:"updated_at"        json:"updated_at"`
}
