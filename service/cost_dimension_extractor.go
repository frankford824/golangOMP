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
	if dims := extractCostSizeWithAreaMultiplierM(normalized); dims.AreaM2 != nil {
		return dims
	}
	if dims := extractCostSizePairM(normalized, costSizePairPattern); dims.AreaM2 != nil {
		return dims
	}
	return extractCostLongestSideM(normalized)
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

func extractCostSizeWithAreaMultiplierM(text string) extractedCostDimensions {
	matches := costSizeMultiplierPattern.FindStringSubmatch(text)
	if len(matches) < 6 {
		return extractedCostDimensions{}
	}
	multiplier, err := strconv.ParseFloat(matches[5], 64)
	if err != nil || multiplier <= 0 {
		return extractedCostDimensions{}
	}
	return buildCostSizeDimensions(matches[1], matches[2], matches[3], matches[4], multiplier)
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
	maxValue := width
	if height > maxValue {
		maxValue = height
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
