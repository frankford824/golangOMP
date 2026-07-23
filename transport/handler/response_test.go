package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestAssetHistoricallyUnavailableMapsToGone(t *testing.T) {
	if got := httpStatusFromCode(domain.ErrCodeAssetHistoricallyUnavailable); got != http.StatusGone {
		t.Fatalf("httpStatusFromCode(%q) = %d, want %d", domain.ErrCodeAssetHistoricallyUnavailable, got, http.StatusGone)
	}
}

func TestAssetMissingRemainsBadRequest(t *testing.T) {
	if got := httpStatusFromCode(domain.ErrCodeAssetMissing); got != http.StatusBadRequest {
		t.Fatalf("httpStatusFromCode(%q) = %d, want %d", domain.ErrCodeAssetMissing, got, http.StatusBadRequest)
	}
}
