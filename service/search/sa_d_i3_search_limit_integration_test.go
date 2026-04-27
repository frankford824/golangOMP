//go:build integration

package search

import (
	"fmt"
	"testing"

	"workflow/domain"
)

func TestSADI3_SearchLimit(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	var taskIDs []int64
	for i := int64(50010); i < 50015; i++ {
		taskIDs = append(taskIDs, i)
	}
	sadCleanup(t, db, taskIDs, nil)
	t.Cleanup(func() { sadCleanup(t, db, taskIDs, nil) })
	for _, id := range taskIDs {
		sadInsertTaskAsset(t, db, id, fmt.Sprintf("SADLIMIT-%d", id), fmt.Sprintf("SADLIMIT-SKU-%d", id), fmt.Sprintf("sad_limit_%d.psd", id))
	}
	ctx, cancel := sadCtx(t)
	defer cancel()
	got, appErr := svc.Search(ctx, sadActor(50010, domain.RoleSuperAdmin), "SADLIMIT", "all", 3)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if len(got.Tasks) > 3 || len(got.Assets) > 3 || len(got.Products) > 3 || len(got.Users) > 3 {
		t.Fatalf("limit exceeded: %+v", got)
	}
}
