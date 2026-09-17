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

func TestOSSIMGThumbnailProcessUsesLowBandwidthWebP(t *testing.T) {
	process, previewable := OSSIMGThumbnailProcessForSize("normal.png", "image/png", 1024)
	if !previewable || !strings.Contains(process, "resize,w_480,m_lfit") || !strings.Contains(process, "format,webp") || !strings.Contains(process, "quality,Q_75") {
		t.Fatalf("thumbnail strategy = (%q, %v)", process, previewable)
	}

	process, previewable = OSSIMGThumbnailProcessForSize("large.png", "image/png", ossIMGDefaultMaxSourceBytes+1)
	if !previewable || process != "" {
		t.Fatalf("oversized thumbnail strategy = (%q, %v), want direct fallback", process, previewable)
	}
}
