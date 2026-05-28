package service

import (
	"math"
	"testing"
)

func TestExtractCostDimensionsMultipliesBoxFaces(t *testing.T) {
	dims := extractCostDimensionsFromText("CPT-常规kt板/抽奖箱/30*30cm*6")
	if dims.WidthM == nil || math.Abs(*dims.WidthM-0.3) > 0.000001 {
		t.Fatalf("width = %+v, want 0.3", dims.WidthM)
	}
	if dims.HeightM == nil || math.Abs(*dims.HeightM-0.3) > 0.000001 {
		t.Fatalf("height = %+v, want 0.3", dims.HeightM)
	}
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.54) > 0.000001 {
		t.Fatalf("area = %+v, want 0.54", dims.AreaM2)
	}
}

func TestExtractCostDimensionsKeepsPlainSizePairArea(t *testing.T) {
	dims := extractCostDimensionsFromText("常规kt板 30*30cm")
	if dims.AreaM2 == nil || math.Abs(*dims.AreaM2-0.09) > 0.000001 {
		t.Fatalf("area = %+v, want 0.09", dims.AreaM2)
	}
}
