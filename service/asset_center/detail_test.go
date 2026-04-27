package asset_center

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestGetDetail_NotFound_ReturnsErrNotFound(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, nil, nil)

	detail, appErr := svc.GetDetail(context.Background(), 999999999)
	if detail != nil {
		t.Fatalf("GetDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}
