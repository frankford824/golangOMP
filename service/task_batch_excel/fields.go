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
	default:
		return nil, false
	}
}

func EnumDictionary() map[string][]string {
	return map[string][]string{
		"material_mode": []string{string(domain.MaterialModePreset), string(domain.MaterialModeOther)},
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
		Column:   "产品款式编码",
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
