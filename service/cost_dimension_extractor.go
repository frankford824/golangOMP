package service

import (
	"regexp"
	"strconv"
	"strings"
)

type extractedCostDimensions struct {
	WidthM  *float64
	HeightM *float64
	AreaM2  *float64
}

var (
	costAreaPattern           = regexp.MustCompile(`(?i)(?:面积|area)?\s*([0-9]+(?:\.[0-9]+)?)\s*(平方米|平方|平米|㎡|m2|m²|平方厘米|cm2|cm²|平方毫米|mm2|mm²)`)
	costSizeTriplePattern     = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)`)
	costSizePairFacesPattern  = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:[/,，;；\s()（）-]*|(?:x|X|×|＊|\*)\s*)([0-9]+(?:\.[0-9]+)?|[一二三四五六七八九十两双单]+)\s*(?:个?面|片|p|P)`)
	costSizeMultiplierPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:面|片|p|P)?`)
	costSizePairPattern       = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?\s*(?:x|X|×|＊|\*)\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?`)
	costNamedSizePattern      = regexp.MustCompile(`(?i)(?:宽|w|width)\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?[\s,，;；/]*(?:高|长|h|height|l|length)\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?`)
	costLongestSidePattern    = regexp.MustCompile(`(?i)(?:最长边|长边|最大边|直径)\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|毫米|cm|厘米|公分|m|米)?`)
)

func extractCostDimensionsFromText(text string) extractedCostDimensions {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return extractedCostDimensions{}
	}
	if area := extractCostAreaM2(normalized); area != nil {
		return extractedCostDimensions{AreaM2: area}
	}
	if dims := extractCostSizePairM(normalized, costNamedSizePattern); dims.AreaM2 != nil {
		return dims
	}
	if dims, ok := extractCostBoxLayoutM(normalized); ok {
		return dims
	}
	if dims := extractCostSizePairWithFacesM(normalized); dims.AreaM2 != nil {
		return dims
	}
	if dims := extractCostSizeWithAreaMultiplierM(normalized); dims.AreaM2 != nil {
		return dims
	}
	if dims := extractCostSizePairM(normalized, costSizePairPattern); dims.AreaM2 != nil {
		return dims
	}
	return extractCostLongestSideM(normalized)
}

func extractCostBoxLayoutM(text string) (extractedCostDimensions, bool) {
	matches := costSizeTriplePattern.FindStringSubmatch(text)
	if len(matches) < 7 {
		return extractedCostDimensions{}, false
	}
	if !containsCostBoxLayoutKeyword(text) {
		return extractedCostDimensions{}, true
	}
	return buildCostBoxLayoutDimensions(matches[1], matches[2], matches[3], matches[4], matches[5], matches[6], text), true
}

func containsCostBoxLayoutKeyword(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	for _, keyword := range []string{
		"立体", "三维", "3d", "箱", "盒", "筒", "柜", "六面", "6面", "6 面", "投壶", "抽奖箱", "开槽",
	} {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func buildCostBoxLayoutDimensions(widthText, widthUnit, depthText, depthUnit, heightText, heightUnit, text string) extractedCostDimensions {
	width, errW := strconv.ParseFloat(widthText, 64)
	depth, errD := strconv.ParseFloat(depthText, 64)
	height, errH := strconv.ParseFloat(heightText, 64)
	if errW != nil || errD != nil || errH != nil || width <= 0 || depth <= 0 || height <= 0 {
		return extractedCostDimensions{}
	}
	unitW, unitD, unitH := inheritTripleDimensionUnits(widthUnit, depthUnit, heightUnit, width, depth, height)
	widthM := dimensionToMeters(width, unitW)
	depthM := dimensionToMeters(depth, unitD)
	heightM := dimensionToMeters(height, unitH)
	if widthM <= 0 || depthM <= 0 || heightM <= 0 {
		return extractedCostDimensions{}
	}
	baseExtra := minPositiveFloat(widthM, depthM)
	if containsClosedBoxKeyword(text) {
		baseExtra = baseExtra * 2
	}
	layoutWidth := 2 * (widthM + depthM)
	layoutHeight := heightM + baseExtra
	area := layoutWidth * layoutHeight
	return extractedCostDimensions{WidthM: &layoutWidth, HeightM: &layoutHeight, AreaM2: &area}
}

func containsClosedBoxKeyword(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	for _, keyword := range []string{"六面", "6面", "6 面", "全封闭", "封闭", "闭合"} {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func inheritTripleDimensionUnits(widthUnit, depthUnit, heightUnit string, width, depth, height float64) (string, string, string) {
	unitW := normalizeDimensionUnitForValues(widthUnit, width, depth, height)
	unitD := normalizeDimensionUnitForValues(depthUnit, width, depth, height)
	unitH := normalizeDimensionUnitForValues(heightUnit, width, depth, height)
	for _, unit := range []string{unitW, unitD, unitH} {
		if strings.TrimSpace(unit) != "" {
			if strings.TrimSpace(widthUnit) == "" {
				unitW = unit
			}
			if strings.TrimSpace(depthUnit) == "" {
				unitD = unit
			}
			if strings.TrimSpace(heightUnit) == "" {
				unitH = unit
			}
			break
		}
	}
	return unitW, unitD, unitH
}

func minPositiveFloat(a, b float64) float64 {
	if a <= 0 {
		return b
	}
	if b <= 0 || a < b {
		return a
	}
	return b
}

func extractCostAreaM2(text string) *float64 {
	matches := costAreaPattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return nil
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || value <= 0 {
		return nil
	}
	unit := strings.ToLower(strings.TrimSpace(matches[2]))
	switch unit {
	case "平方厘米", "cm2", "cm²":
		value = value / 10000
	case "平方毫米", "mm2", "mm²":
		value = value / 1000000
	default:
	}
	return &value
}

func extractCostSizePairM(text string, pattern *regexp.Regexp) extractedCostDimensions {
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 5 {
		return extractedCostDimensions{}
	}
	return buildCostSizeDimensions(matches[1], matches[2], matches[3], matches[4], 1)
}

func extractCostSizePairWithFacesM(text string) extractedCostDimensions {
	matches := costSizePairFacesPattern.FindStringSubmatch(text)
	if len(matches) < 6 {
		return extractedCostDimensions{}
	}
	multiplier, ok := parseCostFaceMultiplier(matches[5])
	if !ok || multiplier <= 0 {
		return extractedCostDimensions{}
	}
	if containsCostBoxLayoutKeyword(text) && isWholeNumber(multiplier) && multiplier >= 5 {
		return buildCostFaceLayoutDimensions(matches[1], matches[2], matches[3], matches[4], int(multiplier))
	}
	return buildCostSizeDimensions(matches[1], matches[2], matches[3], matches[4], multiplier)
}

func extractCostSizeWithAreaMultiplierM(text string) extractedCostDimensions {
	matches := costSizeMultiplierPattern.FindStringSubmatch(text)
	if len(matches) < 6 {
		return extractedCostDimensions{}
	}
	multiplier, err := strconv.ParseFloat(matches[5], 64)
	if err != nil || multiplier <= 0 {
		return extractedCostDimensions{}
	}
	if containsCostBoxLayoutKeyword(text) && isWholeNumber(multiplier) && multiplier >= 5 {
		return buildCostFaceLayoutDimensions(matches[1], matches[2], matches[3], matches[4], int(multiplier))
	}
	return buildCostSizeDimensions(matches[1], matches[2], matches[3], matches[4], multiplier)
}

func buildCostFaceLayoutDimensions(widthText, widthUnit, heightText, heightUnit string, faces int) extractedCostDimensions {
	base := buildCostSizeDimensions(widthText, widthUnit, heightText, heightUnit, 1)
	if base.WidthM == nil || base.HeightM == nil || faces <= 0 {
		return extractedCostDimensions{}
	}
	cols, rows := costFaceLayoutGrid(faces)
	area := (*base.WidthM) * float64(cols) * (*base.HeightM) * float64(rows)
	return extractedCostDimensions{WidthM: base.WidthM, HeightM: base.HeightM, AreaM2: &area}
}

func costFaceLayoutGrid(faces int) (int, int) {
	switch {
	case faces <= 1:
		return 1, 1
	case faces == 2:
		return 2, 1
	case faces == 3:
		return 3, 1
	case faces == 4:
		return 4, 1
	case faces == 5:
		return 4, 2
	case faces == 6:
		return 4, 3
	default:
		cols := 4
		rows := (faces + cols - 1) / cols
		return cols, rows
	}
}

func parseCostFaceMultiplier(raw string) (float64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		return n, n > 0
	}
	switch value {
	case "单", "一":
		return 1, true
	case "双", "两", "二":
		return 2, true
	case "三":
		return 3, true
	case "四":
		return 4, true
	case "五":
		return 5, true
	case "六":
		return 6, true
	case "七":
		return 7, true
	case "八":
		return 8, true
	case "九":
		return 9, true
	case "十":
		return 10, true
	}
	return 0, false
}

func isWholeNumber(value float64) bool {
	return value == float64(int(value))
}

func buildCostSizeDimensions(widthText, widthUnit, heightText, heightUnit string, areaMultiplier float64) extractedCostDimensions {
	width, errW := strconv.ParseFloat(widthText, 64)
	height, errH := strconv.ParseFloat(heightText, 64)
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return extractedCostDimensions{}
	}
	if areaMultiplier <= 0 {
		areaMultiplier = 1
	}
	unitW := normalizeDimensionUnit(widthUnit, width, height)
	unitH := normalizeDimensionUnit(heightUnit, width, height)
	if strings.TrimSpace(widthUnit) == "" && strings.TrimSpace(heightUnit) != "" {
		unitW = unitH
	}
	if strings.TrimSpace(heightUnit) == "" && strings.TrimSpace(widthUnit) != "" {
		unitH = unitW
	}
	widthM := dimensionToMeters(width, unitW)
	heightM := dimensionToMeters(height, unitH)
	if widthM <= 0 || heightM <= 0 {
		return extractedCostDimensions{}
	}
	area := widthM * heightM * areaMultiplier
	return extractedCostDimensions{WidthM: &widthM, HeightM: &heightM, AreaM2: &area}
}

func extractCostLongestSideM(text string) extractedCostDimensions {
	matches := costLongestSidePattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return extractedCostDimensions{}
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || value <= 0 {
		return extractedCostDimensions{}
	}
	unit := normalizeDimensionUnit(matches[2], value, value)
	sideM := dimensionToMeters(value, unit)
	if sideM <= 0 {
		return extractedCostDimensions{}
	}
	area := sideM * sideM
	return extractedCostDimensions{WidthM: &sideM, HeightM: &sideM, AreaM2: &area}
}

func normalizeDimensionUnit(raw string, width, height float64) string {
	return normalizeDimensionUnitForValues(raw, width, height)
}

func normalizeDimensionUnitForValues(raw string, values ...float64) string {
	unit := strings.ToLower(strings.TrimSpace(raw))
	switch unit {
	case "毫米":
		return "mm"
	case "厘米", "公分":
		return "cm"
	case "米":
		return "m"
	case "mm", "cm", "m":
		return unit
	}
	maxValue := 0.0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue > 10 {
		return "cm"
	}
	return "m"
}

func dimensionToMeters(value float64, unit string) float64 {
	switch unit {
	case "mm":
		return value / 1000
	case "cm":
		return value / 100
	default:
		return value
	}
}
