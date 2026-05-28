package service

import (
	"math"
	"testing"
)

func TestExtractCostDimensionsUsesLayoutRectangleForClosedBoxFaces(t *testing.T) {
	dims := extractCostDimensionsFromText("CPT-常规kt板/抽奖箱/30*30cm*6")
	if dims.WidthM == nil || math.Abs(*dims.WidthM-0.3) > 0.000001 {
		t.Fatalf("width = %+v, want 0.3", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-0.3) > 0.000001 {
		t.Fatalf("height = %+v, want 0.3", dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-1.08) > 0.000001 {
		t.Fatalf("area = %+v, want 1.08", dims.AreaM2)
	}
}

func TestExtractCostDimensionsUsesLayoutRectangleForLooseFaceCount(t *testing.T) {
	cases := []string{
		"CPT-常规kt板/抽奖箱/30*30cm*6面",
		"CPT-常规kt板/抽奖箱/30*30cm六面",
		"CPT-常规kt板/抽奖箱/30cm*30cm/六面",
		"CPT-常规kt板/抽奖箱/30cm*30cm 6面",
	}
	for _, text := range cases {
		dims := extractCostDimensionsFromText(text)
		if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-1.08) > 0.000001 {
			t.Fatalf("%s area = %+v, want 1.08", text, dims.AreaM2)
		}
	}
}

func TestExtractCostDimensionsKeepsPlainSizePairArea(t *testing.T) {
	dims := extractCostDimensionsFromText("常规kt板 30*30cm")
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.09) > 0.000001 {
		t.Fatalf("area = %+v, want 0.09", dims.AreaM2)
	}
}

func TestExtractCostDimensionsUsesLayoutRectangleForOpenTube(t *testing.T) {
	dims := extractCostDimensionsFromText("谷常规KT板/开槽/端午射五毒投壶筒/20*20*40cm")
	if dims.WidthM == nil || math.Abs(*dims.WidthM-0.8) > 0.000001 {
		t.Fatalf("layout width = %+v, want 0.8", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-0.6) > 0.000001 {
		t.Fatalf("layout height = %+v, want 0.6", dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.48) > 0.000001 {
		t.Fatalf("area = %+v, want 0.48", dims.AreaM2)
	}
}

func TestExtractCostDimensionsUnqualifiedTripleDoesNotFallbackToPair(t *testing.T) {
	dims := extractCostDimensionsFromText("常规KT板 20*20*40cm")
	if dims.AreaM2 != nil {
		t.Fatalf("area = %+v, want nil for unqualified three-dimensional size", *dims.AreaM2)
	}
}
