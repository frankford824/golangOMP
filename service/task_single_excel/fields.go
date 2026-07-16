package task_single_excel

import (
	"workflow/domain"
)

type FieldFormat string

const (
	FieldFormatString FieldFormat = "string"
	FieldFormatInt64  FieldFormat = "int64"
)

type ViolationCodeSet struct {
	Missing string
	Invalid string
}

type FieldSpec struct {
	Column         string
	Key            string
	Required       bool
	Format         FieldFormat
	HelpText       string
	ViolationCodes ViolationCodeSet
}

func FieldsForTaskType(taskType domain.TaskType, mode string) ([]FieldSpec, bool) {
	if mode != AssistModeSingle {
		return nil, false
	}
	switch taskType {
	case domain.TaskTypeNewProductDevelopment:
		return append([]FieldSpec(nil), npdSingleFields...), true
	case domain.TaskTypeOriginalProductDevelopment:
		return append([]FieldSpec(nil), originalSingleFields...), true
	default:
		return nil, false
	}
}

func fieldByKey(fields []FieldSpec) map[string]FieldSpec {
	out := make(map[string]FieldSpec, len(fields))
	for _, field := range fields {
		out[field.Key] = field
	}
	return out
}

var npdSingleFields = []FieldSpec{
	{
		Column:   "产品款式编码",
		Key:      "product_i_id",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "必填；须为 ERP 产品款式编码（i_id）",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
			Invalid: "invalid_i_id",
		},
	},
	{
		Column:   "产品名称",
		Key:      "product_name",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "必填",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "设计要求",
		Key:      "design_requirement",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "必填",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "规格尺寸",
		Key:      "spec_text",
		Format:   FieldFormatString,
		HelpText: "可选",
	},
	{
		Column:   "材质",
		Key:      "material",
		Format:   FieldFormatString,
		HelpText: "可选",
	},
	{
		Column:   "材质备注",
		Key:      "material_other",
		Format:   FieldFormatString,
		HelpText: "可选",
	},
	{
		Column:   "备注",
		Key:      "remark",
		Format:   FieldFormatString,
		HelpText: "可选",
	},
}

var originalSingleFields = []FieldSpec{
	{
		Column:   "SKU编码",
		Key:      "sku_code",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "必填；须为 ERP 已有商品 SKU 编码",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
			Invalid: "invalid_sku_code",
		},
	},
	{
		Column:   "修改要求",
		Key:      "change_request",
		Required: true,
		Format:   FieldFormatString,
		HelpText: "必填",
		ViolationCodes: ViolationCodeSet{
			Missing: "missing_required_field",
		},
	},
	{
		Column:   "规格尺寸",
		Key:      "spec_text",
		Format:   FieldFormatString,
		HelpText: "可选；未填写时创建后可在详情查看 ERP 规格",
	},
	{
		Column:   "备注",
		Key:      "remark",
		Format:   FieldFormatString,
		HelpText: "可选",
	},
}
