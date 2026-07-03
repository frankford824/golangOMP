package service

import (
	"math"
	"testing"
)

func TestExtractCostDimensionsUsesLayoutRectangleForClosedBoxFaces(t *testing.T) {
	dims := extractCostDimensionsFromText("CPT-常规kt板/抽奖箱/30*30cm*6")
	if dims.WidthM == nil || math.Abs(*dims.WidthM-1.2) > 0.000001 {
		t.Fatalf("width = %+v, want 1.2", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-0.9) > 0.000001 {
		t.Fatalf("height = %+v, want 0.9", dims.HeightM)
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

func TestExtractCostDimensionsUsesSingleSizeAndTotalAreaForPieceSet(t *testing.T) {
	dims := extractCostDimensionsFromText("常规kt板 20*20cm 4件套")
	if dims.WidthM == nil || math.Abs(*dims.WidthM-0.2) > 0.000001 {
		t.Fatalf("width = %+v, want 0.2", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-0.2) > 0.000001 {
		t.Fatalf("height = %+v, want 0.2", dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.16) > 0.000001 {
		t.Fatalf("area = %+v, want 0.16", dims.AreaM2)
	}
}

func TestExtractCostDimensionsSumsMultipleIndependentSizes(t *testing.T) {
	dims := extractCostDimensionsFromText("40*160cm\n40*200cm")
	if dims.WidthM != nil || dims.HeightM != nil {
		t.Fatalf("width/height = %+v/%+v, want nil for mixed-size total", dims.WidthM, dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-1.44) > 0.000001 {
		t.Fatalf("area = %+v, want 1.44", dims.AreaM2)
	}
}

func TestExtractCostDimensionsDeduplicatesRepeatedSameSize(t *testing.T) {
	text := "100*150cm 汪程/常规海报/小王子骑着蓝鲸/9岁/100x150cm 汪程/常规海报/小王子骑着蓝鲸/9岁/100x150cm"
	dims := extractCostDimensionsFromText(text)
	if dims.WidthM == nil || math.Abs(*dims.WidthM-1) > 0.000001 {
		t.Fatalf("width = %+v, want 1", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-1.5) > 0.000001 {
		t.Fatalf("height = %+v, want 1.5", dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-1.5) > 0.000001 {
		t.Fatalf("area = %+v, want 1.5", dims.AreaM2)
	}
}

func TestExtractCostDimensionsDoesNotSumCutoutSize(t *testing.T) {
	dims := extractCostDimensionsFromText("30*42cm（镂空18*18cm）")
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.126) > 0.000001 {
		t.Fatalf("area = %+v, want 0.126", dims.AreaM2)
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
