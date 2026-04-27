//go:build integration

package asset_center

import (
	"context"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

func TestSAAI_GetGlobalAsset_NotFound_Returns404(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	mysqlDB := mysqlrepo.New(db)
	svc := NewService(mysqlrepo.NewTaskAssetSearchRepo(mysqlDB), nil, nil)

	detail, appErr := svc.GetDetail(context.Background(), 999999999)
	if detail != nil {
		t.Fatalf("GetDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}
