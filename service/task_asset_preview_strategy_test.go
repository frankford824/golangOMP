package service

import (
	"strings"
	"testing"
)

func TestOSSIMGPreviewProcessForSizeFallsBackAboveDefaultLimit(t *testing.T) {
	process, previewable := OSSIMGPreviewProcessForSize("large.jpg", "image/jpeg", ossIMGDefaultMaxSourceBytes+1)
	if !previewable || process != "" {
		t.Fatalf("oversized strategy = (%q, %v), want direct untransformed preview", process, previewable)
	}

	process, previewable = OSSIMGPreviewProcessForSize("normal.jpg", "image/jpeg", ossIMGDefaultMaxSourceBytes)
	if !previewable || !strings.Contains(process, "resize,w_1600,m_lfit") {
		t.Fatalf("bounded strategy = (%q, %v), want transformed preview", process, previewable)
	}
}
