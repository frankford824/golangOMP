package task_batch_excel

import (
	"workflow/domain"
)

type FieldFormat string

const (
	FieldFormatString  FieldFormat = "string"
	FieldFormatInt64   FieldFormat = "int64"
	FieldFormatFloat64 FieldFormat = "float64"
	FieldFormatJSON    FieldFormat = "json"
)

type ViolationCodeSet struct {
	Missing string
	Invalid string
}

type FieldSpec struct {
	Column         string
	Key            string
	Required       bool
	AllowedValues  []string
	Format         FieldFormat
	NotAllowed     bool
	HelpText       string
	ViolationCodes ViolationCodeSet
}

func FieldsForTaskType(taskType domain.TaskType) ([]FieldSpec, bool) {
	switch taskType {
	case domain.TaskTypeNewProductDevelopment:
		return append([]FieldSpec(nil), npdFields...), true
	case domain.TaskTypePurchaseTask:
		return append([]FieldSpec(nil), ptFields...), true
	default:
		return nil, false
	}
}

func EnumDictionary() map[string][]string {
	return map[string][]string{
		"material_mode":   []string{string(domain.MaterialModePreset), string(domain.MaterialModeOther)},
		"cost_price_mode": []string{string(domain.CostPriceModeManual), string(domain.CostPriceModeTemplate)},
	}
}

func fieldByKey(fields []FieldSpec) map[string]FieldSpec {
	out := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		out[field.Key] = field
	}
	return out
}

var npdFields = []FieldSpec{
	{
		Column:   "产品名称",
		Key:      "product_name",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "新品开发 SKU 产品名称",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "设计要求",
		Key:      "design_requirement",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "本 SKU 的设计要求",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "产品i_id",
		Key:      "product_i_id",
		Format:   FieldFormatString,
		HelpText: "可选；如需创建后立即同步 ERP，每行必须选择一个来自 /v1/erp/iids 的 i_id",
		ViolationCodes: ViolationCodeSet{
			Invalid: "invalid_i_id",
		},
	},
	{
		Column:   "参考图",
		Key:      "reference_image",
		Format:   FieldFormatString,
		HelpText: "可选；将图片贴到本行任意单元格，解析时后端会提取并上传为本 SKU 的 reference_file_refs",
	},
}

var ptFields = []FieldSpec{
	{
		Column:   "产品名称",
		Key:      "product_name",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "采购 SKU 产品名称",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "类目编码",
		Key:      "category_code",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "ERP/业务类目编码",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "产品i_id",
		Key:      "product_i_id",
		Format:   FieldFormatString,
		HelpText: "可选；如需创建后同步 ERP，每行必须选择一个来自 /v1/erp/iids 的 i_id",
		ViolationCodes: ViolationCodeSet{
			Invalid: "invalid_i_id",
		},
	},
	{
		Column:        "成本模式",
		Key:           "cost_price_mode",
		Required:      true,
		AllowedValues: []string{string(domain.CostPriceModeManual), string(domain.CostPriceModeTemplate)},
		Format:        FieldFormatString,
		HelpText:      "manual 或 template",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
			Invalid: "invalid_cost_price_mode",
		},
	},
	{
		Column:   "数量",
		Key:      "quantity",
		Required: true,
		Format:   FieldFormatInt64,
		HelpText: "大于 0 的整数",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
			Invalid: "missing_required_field",
		},
	},
	{
		Column:   "基础售价",
		Key:      "base_sale_price",
		Required: true,
		Format:   FieldFormatFloat64,
		HelpText: "数字",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "变体JSON",
		Key:      "variant_json",
		Format:   FieldFormatJSON,
		HelpText: "可选 JSON 对象",
		ViolationCodes: ViolationCodeSet{
			Invalid: "invalid_variant_json",
		},
	},
	{
		Column:   "采购SKU",
		Key:      "purchase_sku",
		Format:   FieldFormatString,
		HelpText: "可选；用于指定采购 SKU",
		ViolationCodes: ViolationCodeSet{
			Invalid: "duplicate_batch_sku",
		},
	},
	{
		Column:   "参考图",
		Key:      "reference_image",
		Format:   FieldFormatString,
		HelpText: "可选；将图片贴到本行任意单元格，解析时后端会提取并上传为本 SKU 的 reference_file_refs",
	},
}
